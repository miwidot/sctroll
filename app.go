package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"sctroll/internal/actions"
	"sctroll/internal/config"
	"sctroll/internal/debuglog"
	"sctroll/internal/input"
	"sctroll/internal/keylock"
	"sctroll/internal/starcitizen"
	"sctroll/internal/twitch"
)

type App struct {
	ctx       context.Context
	cfg       *config.Config
	twClient  *twitch.Client
	executor  *actions.Executor
	keyLocker *keylock.KeyLocker
	rewardMu  sync.Mutex // serializes SyncRewards / DeleteAllRewards
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	cfg, err := config.Load()
	if err != nil {
		cfg = config.DefaultConfig()
	}
	a.cfg = cfg

	a.keyLocker = keylock.New(cfg.TargetWindow)
	a.keyLocker.Start()
	a.keyLocker.SetOnKeyBlocked(func(key string) {
		runtime.EventsEmit(a.ctx, "key-blocked", key)
	})

	input.SetMode(cfg.InputMode)

	a.executor = actions.NewExecutor(cfg, a.keyLocker)
	a.executor.SetOnAction(func(actionID, userName string) {
		runtime.EventsEmit(a.ctx, "action-executed", map[string]string{
			"action": actionID,
			"user":   userName,
		})
	})

	go a.autoDetectStarCitizen()

	// Der Refresh Token entscheidet, nicht der Access Token: der ist nach ein
	// paar Stunden ohnehin abgelaufen und wird beim Verbinden erneuert.
	if cfg.Twitch.HasLogin() {
		go a.autoConnect()
	}
}

// autoDetectStarCitizen sucht die Installation, sofern noch keine (oder eine
// nicht mehr existierende) hinterlegt ist.
func (a *App) autoDetectStarCitizen() {
	known := false
	if a.cfg.SCDir != "" {
		if _, err := os.Stat(a.cfg.SCDir); err == nil {
			known = true
		} else {
			debuglog.Log("autoDetect: hinterlegter Pfad existiert nicht mehr: %s", a.cfg.SCDir)
		}
	}

	if !known {
		installs := starcitizen.FindInstalls()
		if len(installs) == 0 {
			runtime.EventsEmit(a.ctx, "twitch-log",
				"Star Citizen nicht gefunden — Ordner bitte in den Einstellungen wählen")
			return
		}
		a.cfg.SCDir = installs[0].Dir
		a.cfg.SCChannel = installs[0].Channel
		_ = a.cfg.Save()
		runtime.EventsEmit(a.ctx, "sc-detected", installs[0])
		runtime.EventsEmit(a.ctx, "twitch-log",
			fmt.Sprintf("Star Citizen %s gefunden: %s", installs[0].Channel, installs[0].Dir))
	}

	a.fillMissingKeys()
}

// fillMissingKeys holt beim Start die Tasten fuer Aktionen, die noch keine
// haben, aus dem Spielprofil.
//
// Betrifft vor allem Aktionen, die Star Citizen ab Werk nicht belegt -- Emotes
// und Tueren. Wer die im Spiel gebunden hat, soll sie nicht jedes Mal von Hand
// nachtragen muessen. Bereits gesetzte Tasten bleiben unangetastet; ein
// vollstaendiger Abgleich passiert nur ueber den Knopf in den Einstellungen.
func (a *App) fillMissingKeys() {
	path, err := a.actionMapsPath()
	if err != nil {
		return
	}
	am, err := starcitizen.Load(path)
	if err != nil {
		return
	}
	overrides := am.AllKeyboardBinds()

	filled := 0
	for _, act := range a.cfg.GetActions() {
		if act.SCAction == "" || act.Key != "" {
			continue
		}
		key, source, hold := effectiveBind(act, overrides)
		if key == "" {
			continue
		}
		act.Key = key
		if hold > act.HoldMs {
			act.HoldMs = hold
		}
		a.cfg.SetAction(act)
		filled++
		debuglog.Log("fillMissingKeys: %s (%s) -> %q [%s]", act.ID, act.SCAction, key, source)
	}

	if filled > 0 {
		_ = a.cfg.Save()
		runtime.EventsEmit(a.ctx, "actions-updated", nil)
		runtime.EventsEmit(a.ctx, "twitch-log",
			fmt.Sprintf("%d fehlende Tasten aus dem Spielprofil ergänzt", filled))
	}
}

func (a *App) shutdown(ctx context.Context) {
	debuglog.Log("=== SCTroll beendet ===")
	if a.twClient != nil {
		a.twClient.Disconnect()
	}
	a.keyLocker.Stop()
	a.cfg.Save()
	debuglog.Close()
}

// --- Config Methods ---

func (a *App) GetConfig() *config.Config {
	return a.cfg
}

func (a *App) GetActions() []config.Action {
	return a.cfg.GetActions()
}

func (a *App) UpdateAction(action config.Action) error {
	a.cfg.SetAction(action)
	if err := a.cfg.Save(); err != nil {
		return err
	}

	// Sync changes to Twitch if connected and reward exists
	if a.twClient != nil && a.twClient.IsConnected() && action.RewardID != "" {
		fields := map[string]interface{}{
			"title": action.RewardTitle,
			"cost":  action.RewardCost,
		}
		twCooldown := action.Cooldown / 1000
		if action.TwitchCooldown > 0 {
			twCooldown = action.TwitchCooldown
		}
		if twCooldown >= 60 {
			fields["is_global_cooldown_enabled"] = true
			fields["global_cooldown_seconds"] = twCooldown
		} else if twCooldown > 0 {
			fields["is_global_cooldown_enabled"] = true
			fields["global_cooldown_seconds"] = 60
		}
		if err := a.twClient.UpdateReward(action.RewardID, fields, action.RewardColor); err != nil {
			debuglog.Log("UpdateAction: Twitch sync error: %s", err)
		} else {
			debuglog.Log("UpdateAction: Twitch reward updated for %s", action.ID)
		}
	}
	return nil
}

func (a *App) AddCustomAction(action config.Action) (config.Action, error) {
	action.ID = fmt.Sprintf("custom_%d", time.Now().UnixNano())
	action.Custom = true
	action.Enabled = false

	if action.Name == "" || action.Key == "" || action.RewardTitle == "" {
		return action, fmt.Errorf("Name, Key und Reward-Titel sind erforderlich")
	}

	a.cfg.SetAction(action)
	if err := a.cfg.Save(); err != nil {
		return action, err
	}

	debuglog.Log("AddCustomAction: created %s (%s)", action.ID, action.Name)
	runtime.EventsEmit(a.ctx, "actions-updated", nil)
	return action, nil
}

func (a *App) DeleteAction(id string) error {
	actions := a.cfg.GetActions()
	var target *config.Action
	for _, act := range actions {
		if act.ID == id {
			act := act
			target = &act
			break
		}
	}
	if target == nil {
		return fmt.Errorf("action not found: %s", id)
	}
	if !target.Custom {
		return fmt.Errorf("cannot delete default action: %s", id)
	}

	// Delete Twitch reward if it exists
	if target.RewardID != "" && a.twClient != nil && a.twClient.IsConnected() {
		if err := a.twClient.DeleteReward(target.RewardID); err != nil {
			debuglog.Log("DeleteAction: Twitch reward delete error: %s", err)
		}
	}

	if !a.cfg.DeleteAction(id) {
		return fmt.Errorf("failed to delete action: %s", id)
	}

	debuglog.Log("DeleteAction: deleted %s (%s)", id, target.Name)
	runtime.EventsEmit(a.ctx, "actions-updated", nil)
	return a.cfg.Save()
}

func (a *App) ToggleAction(id string, enabled bool) error {
	a.cfg.ToggleAction(id, enabled)

	actns := a.cfg.GetActions()
	for _, act := range actns {
		if act.ID == id && a.twClient != nil && a.twClient.IsConnected() {
			if act.RewardID != "" {
				// Reward auf Twitch aktivieren/deaktivieren (pausieren)
				debuglog.Log("ToggleAction: %s enabled=%v rewardID=%s", id, enabled, act.RewardID)
				if err := a.twClient.UpdateRewardEnabled(act.RewardID, enabled, act.RewardColor); err != nil {
					debuglog.Log("ToggleAction: UpdateRewardEnabled error: %s", err)
				}
			} else if enabled {
				// Kein Reward vorhanden, neu erstellen
				twCooldown := act.Cooldown
				if act.TwitchCooldown > 0 {
					twCooldown = act.TwitchCooldown * 1000
				}
				rewardID, err := a.twClient.CreateReward(act.RewardTitle, act.RewardCost, twCooldown, act.RewardColor)
				if err == nil {
					act.RewardID = rewardID
					a.cfg.SetAction(act)
				}
			}
		}
	}

	return a.cfg.Save()
}

func (a *App) SetGlobalEnable(enabled bool) error {
	a.cfg.GlobalEnable = enabled
	runtime.EventsEmit(a.ctx, "global-toggle", enabled)

	// AUS = Rewards löschen, AN = Rewards neu erstellen
	if a.twClient != nil && a.twClient.IsConnected() {
		go func() {
			if enabled {
				// Rewards neu erstellen
				runtime.EventsEmit(a.ctx, "twitch-log", "Erstelle Rewards auf Twitch...")
				a.SyncRewards()
				runtime.EventsEmit(a.ctx, "twitch-log", "Rewards erstellt")
			} else {
				// Rewards löschen
				runtime.EventsEmit(a.ctx, "twitch-log", "Lösche Rewards von Twitch...")
				a.DeleteAllRewards()
				runtime.EventsEmit(a.ctx, "twitch-log", "Rewards gelöscht")
			}
		}()
	}

	return a.cfg.Save()
}

func (a *App) GetGlobalEnable() bool {
	return a.cfg.GlobalEnable
}

func (a *App) SetTargetWindow(name string) error {
	a.cfg.TargetWindow = name
	return a.cfg.Save()
}

// --- Twitch Device Code Flow ---

type DeviceAuthInfo struct {
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
}

func (a *App) StartTwitchAuth() (*DeviceAuthInfo, error) {
	a.twClient = twitch.NewClient(&a.cfg.Twitch)
	a.setupTwitchCallbacks()

	dcr, err := a.twClient.RequestDeviceCode()
	if err != nil {
		return nil, err
	}

	// Open the verification URL in the browser
	runtime.BrowserOpenURL(a.ctx, dcr.VerificationURI)

	// Poll for token in background, then auto-connect EventSub
	go func() {
		err := a.twClient.PollForToken(dcr.DeviceCode, dcr.Interval, dcr.ExpiresIn)
		if err != nil {
			runtime.EventsEmit(a.ctx, "twitch-error", err.Error())
			return
		}
		a.cfg.Save()
		runtime.EventsEmit(a.ctx, "twitch-authenticated", a.cfg.Twitch.ChannelName)

		// Auto-connect to EventSub
		if err := a.twClient.Connect(); err != nil {
			runtime.EventsEmit(a.ctx, "twitch-log",
				fmt.Sprintf("EventSub Auto-Connect fehlgeschlagen: %s", err.Error()))
			return
		}
		a.twClient.StartTokenRefresher()
		runtime.EventsEmit(a.ctx, "twitch-log", "EventSub verbunden")

		// Sync rewards
		if a.cfg.GlobalEnable {
			a.SyncRewards()
			runtime.EventsEmit(a.ctx, "twitch-log", "Rewards synchronisiert")
		}
	}()

	return &DeviceAuthInfo{
		UserCode:        dcr.UserCode,
		VerificationURI: dcr.VerificationURI,
	}, nil
}

func (a *App) ConnectTwitch() error {
	return a.connectTwitch()
}

func (a *App) DisconnectTwitch() {
	if a.twClient != nil {
		a.twClient.Disconnect()
		// Verworfen, nicht wiederverwendet: der Client ist nach Disconnect
		// endgueltig gestoppt. Ein erneutes Verbinden legt einen neuen an.
		a.twClient = nil
		runtime.EventsEmit(a.ctx, "twitch-disconnected", nil)
	}
}

func (a *App) IsTwitchConnected() bool {
	return a.twClient != nil && a.twClient.IsConnected()
}

func (a *App) GetTwitchChannel() string {
	return a.cfg.Twitch.ChannelName
}

// TwitchApp beschreibt die verwendete Twitch-Anwendung fuer die Oberflaeche.
// Das Secret wird nur als "gesetzt/nicht gesetzt" gemeldet, nicht im Klartext.
type TwitchApp struct {
	ClientID  string `json:"client_id"`
	HasSecret bool   `json:"has_secret"`
	IsDefault bool   `json:"is_default"`
}

func (a *App) GetTwitchApp() TwitchApp {
	return TwitchApp{
		ClientID:  a.cfg.Twitch.ClientID,
		HasSecret: a.cfg.Twitch.ClientSecret != "",
		IsDefault: a.cfg.Twitch.ClientID == twitch.DefaultClientID,
	}
}

// SetTwitchApp hinterlegt eine eigene Twitch-Anwendung.
//
// Ein Wechsel der Client-ID macht die bisherige Anmeldung ungueltig -- Tokens
// gehoeren immer zu genau einer App. Rewards, die unter der alten ID angelegt
// wurden, lassen sich danach ausserdem nicht mehr verwalten und muessen neu
// erstellt werden.
func (a *App) SetTwitchApp(clientID, clientSecret string) error {
	clientID = strings.TrimSpace(clientID)
	clientSecret = strings.TrimSpace(clientSecret)

	if clientID == "" {
		clientID = twitch.DefaultClientID
	}

	changedApp := clientID != a.cfg.Twitch.ClientID
	if changedApp {
		debuglog.Log("SetTwitchApp: Client-ID gewechselt — Anmeldung wird zurückgesetzt")
		a.cfg.Twitch.AccessToken = ""
		a.cfg.Twitch.RefreshToken = ""
		a.cfg.Twitch.ExpiresAt = time.Time{}
		for _, act := range a.cfg.GetActions() {
			if act.RewardID != "" {
				act.RewardID = ""
				a.cfg.SetAction(act)
			}
		}
		if a.twClient != nil {
			a.twClient.Disconnect()
			a.twClient = nil
		}
		runtime.EventsEmit(a.ctx, "twitch-disconnected", "App gewechselt")
	}

	a.cfg.Twitch.ClientID = clientID
	a.cfg.Twitch.ClientSecret = clientSecret

	if err := a.cfg.Save(); err != nil {
		return err
	}
	runtime.EventsEmit(a.ctx, "twitch-log", map[bool]string{
		true:  "Twitch-App gewechselt — bitte neu verbinden",
		false: "Twitch-App aktualisiert",
	}[changedApp])
	return nil
}

// autoConnect stellt die Anmeldung beim Start wieder her.
//
// Es wird so lange erneut versucht, wie der Fehler voruebergehend sein kann --
// beim Systemstart ist das Netz oft noch nicht da. Nur wenn Twitch den Refresh
// Token endgueltig ablehnt, ist eine neue Anmeldung noetig.
func (a *App) autoConnect() {
	a.twClient = twitch.NewClient(&a.cfg.Twitch)
	a.setupTwitchCallbacks()

	runtime.EventsEmit(a.ctx, "twitch-log", "Anmeldung wird wiederhergestellt...")

	delays := []time.Duration{0, 3 * time.Second, 10 * time.Second, 30 * time.Second, time.Minute}
	for i, d := range delays {
		if d > 0 {
			time.Sleep(d)
		}

		err := a.twClient.Connect()
		if err == nil {
			a.cfg.Save()
			a.twClient.StartTokenRefresher()
			runtime.EventsEmit(a.ctx, "twitch-authenticated", a.cfg.Twitch.ChannelName)
			runtime.EventsEmit(a.ctx, "twitch-log",
				fmt.Sprintf("Angemeldet als %s", a.cfg.Twitch.ChannelName))
			return
		}

		// Falsch registrierte App: erneutes Versuchen bringt nichts, eine
		// Neuanmeldung genauso wenig. Das muss der Nutzer einmal einrichten.
		if errors.Is(err, twitch.ErrClientSecretRequired) {
			debuglog.Log("autoConnect: %s", err)
			runtime.EventsEmit(a.ctx, "twitch-log", err.Error())
			runtime.EventsEmit(a.ctx, "twitch-needs-secret", err.Error())
			runtime.EventsEmit(a.ctx, "twitch-disconnected", "App-Einrichtung unvollständig")
			return
		}

		if errors.Is(err, twitch.ErrLoginRequired) {
			debuglog.Log("autoConnect: Anmeldung abgelaufen: %s", err)
			runtime.EventsEmit(a.ctx, "twitch-log", err.Error())
			runtime.EventsEmit(a.ctx, "twitch-disconnected", "Anmeldung abgelaufen")
			return
		}

		debuglog.Log("autoConnect: Versuch %d fehlgeschlagen: %s", i+1, err)
		if i < len(delays)-1 {
			runtime.EventsEmit(a.ctx, "twitch-log",
				fmt.Sprintf("Verbindung fehlgeschlagen (%s) — neuer Versuch in %s", err, delays[i+1]))
		}
	}

	runtime.EventsEmit(a.ctx, "twitch-log",
		"Twitch nicht erreichbar — Anmeldung bleibt gespeichert, im Twitch-Tab neu verbinden")
	runtime.EventsEmit(a.ctx, "twitch-disconnected", "nicht erreichbar")
}

func (a *App) connectTwitch() error {
	if a.twClient == nil {
		a.twClient = twitch.NewClient(&a.cfg.Twitch)
		a.setupTwitchCallbacks()
	}

	// Connect erneuert den Token selbst, falls noetig.
	if err := a.twClient.Connect(); err != nil {
		return err
	}
	a.twClient.StartTokenRefresher()
	return a.cfg.Save()
}

func (a *App) reconnectLoop() {
	retries := 0
	maxRetries := 10
	for retries < maxRetries {
		retries++
		delay := time.Duration(min(retries*5, 30)) * time.Second
		runtime.EventsEmit(a.ctx, "twitch-log",
			fmt.Sprintf("Reconnect in %ds... (Versuch %d/%d)", int(delay.Seconds()), retries, maxRetries))
		time.Sleep(delay)

		if a.twClient == nil {
			return
		}

		if err := a.twClient.Connect(); err != nil {
			// Bei abgelaufener Anmeldung oder falsch registrierter App hilft
			// kein weiterer Versuch.
			if errors.Is(err, twitch.ErrClientSecretRequired) {
				runtime.EventsEmit(a.ctx, "twitch-log", err.Error())
				runtime.EventsEmit(a.ctx, "twitch-needs-secret", err.Error())
				runtime.EventsEmit(a.ctx, "twitch-disconnected", "App-Einrichtung unvollständig")
				return
			}
			if errors.Is(err, twitch.ErrLoginRequired) {
				runtime.EventsEmit(a.ctx, "twitch-log", err.Error())
				runtime.EventsEmit(a.ctx, "twitch-disconnected", "Anmeldung abgelaufen")
				return
			}
			runtime.EventsEmit(a.ctx, "twitch-log",
				fmt.Sprintf("Reconnect fehlgeschlagen: %s", err.Error()))
			continue
		}
		a.cfg.Save()

		runtime.EventsEmit(a.ctx, "twitch-log", "Reconnect erfolgreich!")
		return
	}
	runtime.EventsEmit(a.ctx, "twitch-log", "Reconnect aufgegeben — bitte manuell verbinden")
}

func (a *App) setupTwitchCallbacks() {
	a.twClient.SetOnRedemption(func(rewardID, redemptionID, userName, rewardTitle string) {
		debuglog.Log("Redemption: user=%s reward=%q id=%s redemption=%s", userName, rewardTitle, rewardID, redemptionID)
		runtime.EventsEmit(a.ctx, "twitch-log",
			fmt.Sprintf("Einlösung von %s: %s", userName, rewardTitle))

		for _, action := range a.cfg.GetActions() {
			if action.RewardID != rewardID {
				continue
			}
			debuglog.Log("Redemption matched: action=%s", action.ID)

			err := a.executor.Execute(action.ID, userName)
			if err == nil {
				a.finishRedemption(rewardID, redemptionID, "FULFILLED")
				return
			}

			// Cooldown, Spiel nicht im Vordergrund, Aktion aus, Queue voll --
			// in all diesen Faellen ist nichts passiert, also Punkte zurueck.
			debuglog.Log("Redemption execute error: %s", err)
			runtime.EventsEmit(a.ctx, "action-error", map[string]string{
				"action": action.ID,
				"error":  err.Error(),
			})
			runtime.EventsEmit(a.ctx, "twitch-log",
				fmt.Sprintf("%s nicht ausgeführt (%s) — Punkte zurück", rewardTitle, err.Error()))
			a.finishRedemption(rewardID, redemptionID, "CANCELED")
			return
		}

		debuglog.Log("Redemption: NO MATCH for rewardID=%s", rewardID)
	})

	a.twClient.SetOnConnect(func() {
		runtime.EventsEmit(a.ctx, "twitch-connected", nil)
	})

	a.twClient.SetOnDisconnect(func(err error) {
		msg := "disconnected"
		if err != nil {
			msg = err.Error()
		}
		runtime.EventsEmit(a.ctx, "twitch-disconnected", msg)

		// Auto-reconnect if we have tokens
		if a.cfg.Twitch.RefreshToken != "" && err != nil {
			go a.reconnectLoop()
		}
	})

	a.twClient.SetOnLog(func(msg string) {
		runtime.EventsEmit(a.ctx, "twitch-log", msg)
	})

	a.twClient.SetOnTokenRefresh(func() {
		_ = a.cfg.Save()
		debuglog.Log("Twitch: access token auto-refreshed and saved")
	})
}

// finishRedemption schliesst eine Einloesung ab. CANCELED erstattet die
// Kanalpunkte. Schlaegt fehl, wenn der Reward nicht von dieser App stammt --
// das ist kein Grund fuer eine Fehlermeldung im UI, deshalb nur ins Log.
func (a *App) finishRedemption(rewardID, redemptionID, status string) {
	if a.twClient == nil || redemptionID == "" {
		return
	}
	if err := a.twClient.SetRedemptionStatus(rewardID, redemptionID, status); err != nil {
		debuglog.Log("finishRedemption(%s): %s", status, err)
	}
}

// --- Reward Management ---

func (a *App) SyncRewards() error {
	a.rewardMu.Lock()
	defer a.rewardMu.Unlock()

	if a.twClient == nil || !a.twClient.IsConnected() {
		debuglog.Log("SyncRewards: nicht verbunden")
		return fmt.Errorf("not connected to Twitch")
	}

	debuglog.Log("=== SyncRewards START ===")

	// Fetch existing rewards to avoid duplicates
	existing, err := a.twClient.GetExistingRewards()
	if err != nil {
		debuglog.Log("SyncRewards: GetExistingRewards error: %s", err)
		runtime.EventsEmit(a.ctx, "twitch-log",
			fmt.Sprintf("Konnte bestehende Rewards nicht laden: %s", err.Error()))
		existing = nil
	}

	// Build a title -> ID map of existing rewards
	existingByTitle := make(map[string]string)
	existingByID := make(map[string]bool)
	for _, r := range existing {
		existingByTitle[r.Title] = r.ID
		existingByID[r.ID] = true
	}
	debuglog.Log("SyncRewards: %d Rewards auf Twitch gefunden", len(existing))

	actns := a.cfg.GetActions()
	debuglog.Log("SyncRewards: %d Aktionen in Config", len(actns))

	for _, action := range actns {
		debuglog.Log("SyncRewards: action=%s enabled=%v rewardID=%q title=%q",
			action.ID, action.Enabled, action.RewardID, action.RewardTitle)

		if !action.Enabled {
			// Wenn disabled aber noch eine RewardID hat, aufräumen
			if action.RewardID != "" {
				debuglog.Log("SyncRewards: %s ist disabled aber hat RewardID=%s — lösche", action.ID, action.RewardID)
				a.twClient.DeleteReward(action.RewardID)
				action.RewardID = ""
				a.cfg.SetAction(action)
			}
			continue
		}

		// Check if already linked AND reward still exists on Twitch
		if action.RewardID != "" {
			if existingByID[action.RewardID] {
				debuglog.Log("SyncRewards: %s bereits verknüpft und existiert (ID: %s)", action.ID, action.RewardID)
				runtime.EventsEmit(a.ctx, "twitch-log",
					fmt.Sprintf("Reward '%s' bereits verknüpft (ID: %s)", action.RewardTitle, action.RewardID))
				continue
			}
			// RewardID saved but doesn't exist on Twitch anymore — clear it
			debuglog.Log("SyncRewards: %s hat RewardID=%s aber existiert NICHT auf Twitch — wird neu erstellt", action.ID, action.RewardID)
			action.RewardID = ""
		}

		// Check if reward already exists on Twitch by title
		if existingID, ok := existingByTitle[action.RewardTitle]; ok {
			debuglog.Log("SyncRewards: %s gefunden über Titel: %q → ID=%s", action.ID, action.RewardTitle, existingID)
			action.RewardID = existingID
			a.cfg.SetAction(action)
			runtime.EventsEmit(a.ctx, "twitch-log",
				fmt.Sprintf("Bestehender Reward gefunden: %s (ID: %s)", action.RewardTitle, existingID))
			continue
		}

		// Create new reward
		twCooldown := action.Cooldown
		if action.TwitchCooldown > 0 {
			twCooldown = action.TwitchCooldown * 1000
		}
		debuglog.Log("SyncRewards: erstelle neuen Reward für %s: title=%q cost=%d twCooldown=%dms", action.ID, action.RewardTitle, action.RewardCost, twCooldown)
		rewardID, err := a.twClient.CreateReward(action.RewardTitle, action.RewardCost, twCooldown, action.RewardColor)
		if err != nil {
			debuglog.Log("SyncRewards: FEHLER beim Erstellen von %s: %s", action.ID, err)
			runtime.EventsEmit(a.ctx, "twitch-log",
				fmt.Sprintf("Fehler beim Erstellen von '%s': %s", action.RewardTitle, err.Error()))
			continue
		}

		action.RewardID = rewardID
		a.cfg.SetAction(action)
		debuglog.Log("SyncRewards: %s erstellt → ID=%s", action.ID, rewardID)
		runtime.EventsEmit(a.ctx, "twitch-log",
			fmt.Sprintf("Reward erstellt: %s (ID: %s)", action.RewardTitle, rewardID))
	}

	debuglog.Log("=== SyncRewards ENDE ===")
	return a.cfg.Save()
}

func (a *App) DeleteAllRewards() error {
	a.rewardMu.Lock()
	defer a.rewardMu.Unlock()

	if a.twClient == nil {
		return fmt.Errorf("not connected to Twitch")
	}

	debuglog.Log("=== DeleteAllRewards START ===")
	actns := a.cfg.GetActions()
	for _, action := range actns {
		if action.RewardID == "" {
			continue
		}
		debuglog.Log("DeleteAllRewards: lösche %s (RewardID=%s)", action.ID, action.RewardID)
		a.twClient.DeleteReward(action.RewardID)
		action.RewardID = ""
		a.cfg.SetAction(action)
	}
	debuglog.Log("=== DeleteAllRewards ENDE ===")

	return a.cfg.Save()
}

func (a *App) GetDebugLogPath() string {
	return debuglog.GetLogPath()
}

// --- Language ---

func (a *App) GetLanguage() string {
	if a.cfg.Language == "" {
		return "de"
	}
	return a.cfg.Language
}

func (a *App) SetLanguage(lang string) error {
	a.cfg.Language = lang
	return a.cfg.Save()
}

// --- Star Citizen: Installation und Tastenbelegungen ---

// BindStatus vergleicht fuer eine Aktion die Taste, die sctroll drueckt, mit
// der, die im Spiel tatsaechlich gilt.
//
// Die gueltige Taste ergibt sich aus zwei Quellen: was in der actionmaps.xml
// steht (eigene Belegung) und sonst aus Star Citizens defaultProfile.xml
// (Standardbelegung). Manche Aktionen haben ab Werk gar keine Taste -- Tueren
// und Emotes zum Beispiel -- die muss man im Spiel selbst binden.
type BindStatus struct {
	ActionID  string `json:"action_id"`
	Name      string `json:"name"`
	Enabled   bool   `json:"enabled"`
	SCAction  string `json:"sc_action"`
	Key       string `json:"key"`       // was sctroll drueckt
	Effective string `json:"effective"` // was im Spiel gilt
	Source    string `json:"source"`    // "profil" | "standard" | "unbelegt"
	Mismatch  bool   `json:"mismatch"`  // sctroll drueckt etwas anderes als im Spiel gilt
}

// effectiveBind loest die im Spiel gueltige Taste einer Aktion auf.
func effectiveBind(act config.Action, overrides map[string]string) (key, source string, hold int) {
	id := act.SCActionMap + "|" + act.SCAction
	def := starcitizen.DefaultBinds()[id]

	if k, ok := overrides[id]; ok && k != "" {
		// Eigene Belegung. Die Aktivierungsart kommt weiterhin aus dem
		// Standardprofil -- die aendert sich beim Umbinden nicht.
		return k, "profil", def.HoldMs
	}
	if def.Key != "" {
		return def.Key, "standard", def.HoldMs
	}
	return "", "unbelegt", def.HoldMs
}

func (a *App) GetSCInstalls() []starcitizen.Install {
	return starcitizen.FindInstalls()
}

func (a *App) GetSCDir() string {
	return a.cfg.SCDir
}

func (a *App) GetSCChannel() string {
	return a.cfg.SCChannel
}

// SetSCDir uebernimmt einen manuell gewaehlten Ordner. Akzeptiert sowohl den
// Channel-Ordner (...\LIVE) als auch den StarCitizen-Ordner darueber.
func (a *App) SetSCDir(dir string) error {
	inst, ok := starcitizen.InstallFromDir(dir)
	if !ok {
		return fmt.Errorf("in %q steckt keine Star-Citizen-Installation", dir)
	}
	a.cfg.SCDir = inst.Dir
	a.cfg.SCChannel = inst.Channel
	runtime.EventsEmit(a.ctx, "sc-detected", inst)
	return a.cfg.Save()
}

// BrowseSCDir oeffnet den Ordnerdialog. Notwendig, weil das Spiel auf einem
// beliebigen Laufwerk in einem beliebigen Ordner liegen kann.
func (a *App) BrowseSCDir() (string, error) {
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title:            "Star-Citizen-Ordner wählen (z.B. ...\\StarCitizen\\LIVE)",
		DefaultDirectory: a.cfg.SCDir,
	})
	if err != nil || dir == "" {
		return "", err
	}
	if err := a.SetSCDir(dir); err != nil {
		return "", err
	}
	return a.cfg.SCDir, nil
}

func (a *App) IsGameRunning() bool {
	return starcitizen.IsGameRunning()
}

func (a *App) actionMapsPath() (string, error) {
	if a.cfg.SCDir == "" {
		return "", fmt.Errorf("kein Star-Citizen-Ordner gesetzt")
	}
	path := starcitizen.Install{Dir: a.cfg.SCDir}.ActionMapsPath()
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("actionmaps.xml nicht gefunden — Star Citizen einmal starten und beenden")
	}
	return path, nil
}

// GetBindStatus zeigt pro Aktion, welche Taste sctroll drueckt und was dazu in
// der actionmaps.xml steht.
//
// Ein leeres InProfile heisst NICHT "unbelegt", sondern "auf Standardbelegung":
// die Datei enthaelt ausschliesslich Abweichungen vom Standard. Eine Aktion auf
// den Joystick zu legen loescht ihre Tastaturbelegung nicht.
func (a *App) GetBindStatus() ([]BindStatus, error) {
	path, err := a.actionMapsPath()
	if err != nil {
		return nil, err
	}
	am, err := starcitizen.Load(path)
	if err != nil {
		return nil, err
	}
	overrides := am.AllKeyboardBinds()

	var out []BindStatus
	for _, act := range a.cfg.GetActions() {
		if act.SCAction == "" {
			continue // z.B. der 360-Spin, der nur die Maus bewegt
		}
		effective, source, _ := effectiveBind(act, overrides)
		out = append(out, BindStatus{
			ActionID:  act.ID,
			Name:      act.Name,
			Enabled:   act.Enabled,
			SCAction:  act.SCAction,
			Key:       act.Key,
			Effective: effective,
			Source:    source,
			Mismatch:  effective != "" && !strings.EqualFold(effective, act.Key),
		})
	}
	return out, nil
}

// ImportSCBinds gleicht die Tasten der Aktionen mit dem Spiel ab: eigene
// Belegung aus der actionmaps.xml, sonst Star Citizens Standardbelegung.
// Beide Dateien werden nur gelesen.
//
// Aktionen ohne jede Belegung (Tueren und Emotes haben ab Werk keine) bleiben
// unangetastet und werden in der Liste als "unbelegt" gefuehrt.
func (a *App) ImportSCBinds() (int, error) {
	path, err := a.actionMapsPath()
	if err != nil {
		return 0, err
	}
	am, err := starcitizen.Load(path)
	if err != nil {
		return 0, err
	}
	overrides := am.AllKeyboardBinds()

	updated := 0
	for _, act := range a.cfg.GetActions() {
		if act.SCAction == "" {
			continue
		}

		key, source, hold := effectiveBind(act, overrides)
		if key == "" {
			debuglog.Log("ImportSCBinds: %s (%s) im Spiel unbelegt — übersprungen", act.ID, act.SCAction)
			continue
		}

		changed := false
		if !strings.EqualFold(key, act.Key) {
			debuglog.Log("ImportSCBinds: %s (%s) %q -> %q [%s]", act.ID, act.SCAction, act.Key, key, source)
			act.Key = key
			changed = true
		}

		// Halteaktionen brauchen echte Haltezeit, sonst passiert im Spiel
		// nichts. Nur verlaengern, nie verkuerzen -- eigene Werte bleiben.
		if hold > act.HoldMs {
			debuglog.Log("ImportSCBinds: %s Haltezeit %dms -> %dms", act.ID, act.HoldMs, hold)
			act.HoldMs = hold
			changed = true
		}

		if changed {
			a.cfg.SetAction(act)
			updated++
		}
	}

	if updated > 0 {
		if err := a.cfg.Save(); err != nil {
			return 0, err
		}
		runtime.EventsEmit(a.ctx, "actions-updated", nil)
		runtime.EventsEmit(a.ctx, "twitch-log",
			fmt.Sprintf("%d Aktionen mit den Belegungen aus dem Spiel abgeglichen", updated))
	}
	return updated, nil
}

// RemoveSCBinds nimmt Tastaturbelegungen zurueck, die eine frühere sctroll-
// Version in die actionmaps.xml geschrieben hat.
//
// Das ist kein Kosmetikproblem: ein Eintrag in der Datei ueberschreibt Star
// Citizens Standardbelegung. Wer also vorher mit der Standardtaste gespielt hat,
// dem hat der Eintrag genau die weggenommen. Entfernt wird nur, was exakt einer
// mitgelieferten sctroll-Taste entspricht -- eigene Belegungen bleiben.
func (a *App) RemoveSCBinds() (int, error) {
	if starcitizen.IsGameRunning() {
		return 0, fmt.Errorf("Star Citizen läuft — bitte erst beenden, sonst schreibt das Spiel die Datei zurück")
	}

	path, err := a.actionMapsPath()
	if err != nil {
		return 0, err
	}
	am, err := starcitizen.Load(path)
	if err != nil {
		return 0, err
	}

	shipped := map[string]string{}
	for _, d := range config.DefaultActions() {
		if d.SCAction != "" {
			shipped[d.SCActionMap+"|"+d.SCAction] = d.Key
		}
	}

	removed := 0
	for key, current := range am.AllKeyboardBinds() {
		want, ok := shipped[key]
		if !ok || !strings.EqualFold(current, want) {
			continue
		}
		parts := strings.SplitN(key, "|", 2)
		if am.RemoveKeyboardBind(parts[0], parts[1]) {
			debuglog.Log("RemoveSCBinds: %s (kb1_%s) entfernt", key, current)
			removed++
		}
	}

	if removed == 0 {
		return 0, nil
	}
	if err := am.Save(); err != nil {
		return 0, err
	}
	runtime.EventsEmit(a.ctx, "twitch-log",
		fmt.Sprintf("%d von sctroll gesetzte Belegungen entfernt — Standardbelegung gilt wieder", removed))
	return removed, nil
}

// --- Test und Eingabeverfahren ---

// GetInputMode / SetInputMode steuern, wie Tastendruecke gesendet werden.
func (a *App) GetInputMode() string { return input.Mode() }

func (a *App) SetInputMode(mode string) error {
	input.SetMode(mode)
	a.cfg.InputMode = input.Mode()
	return a.cfg.Save()
}

// TestAction loest eine Aktion nach einer Wartezeit aus, damit man in der
// Zwischenzeit ins Spiel wechseln kann. Ohne das laesst sich nicht pruefen, ob
// ein Tastendruck ueberhaupt ankommt -- eine Einloesung braucht sonst einen
// echten Zuschauer.
//
// Das Ergebnis steht danach im Event Log und ausfuehrlich im Debug-Log.
func (a *App) TestAction(actionID string, delaySeconds int) {
	if delaySeconds < 0 {
		delaySeconds = 0
	}

	go func() {
		for left := delaySeconds; left > 0; left-- {
			runtime.EventsEmit(a.ctx, "test-countdown", map[string]any{
				"action": actionID, "left": left,
			})
			time.Sleep(time.Second)
		}
		runtime.EventsEmit(a.ctx, "test-countdown", map[string]any{
			"action": actionID, "left": 0,
		})

		if err := a.executor.Test(actionID); err != nil {
			debuglog.Log("TestAction %s: %s", actionID, err)
			runtime.EventsEmit(a.ctx, "twitch-log", fmt.Sprintf("Test fehlgeschlagen: %s", err))
			runtime.EventsEmit(a.ctx, "test-result", map[string]any{
				"action": actionID, "ok": false, "error": err.Error(),
			})
			return
		}
		runtime.EventsEmit(a.ctx, "twitch-log",
			fmt.Sprintf("Test gesendet (%s) — kam es im Spiel an?", input.Mode()))
		runtime.EventsEmit(a.ctx, "test-result", map[string]any{
			"action": actionID, "ok": true,
		})
	}()
}

// --- Status ---

func (a *App) GetCooldownRemaining(actionID string) int {
	return a.executor.GetCooldownRemaining(actionID)
}

func (a *App) GetLockedKeys() []string {
	return a.keyLocker.GetLockedKeys()
}
