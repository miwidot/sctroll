package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type KeyLockConfig struct {
	Enabled  bool     `json:"enabled"`
	Keys     []string `json:"keys"`
	Duration int      `json:"duration_ms"`
}

type ActionStep struct {
	Key      string `json:"key"`
	HoldMs   int    `json:"hold_ms"`
	DelayMs  int    `json:"delay_ms,omitempty"`
	HoldDown bool   `json:"hold_down,omitempty"` // true = druecken und halten bis zum "release"-Schritt
	Release  string `json:"release,omitempty"`   // Taste, die ein frueherer hold_down-Schritt haelt
}

// Action verbindet einen Twitch-Reward mit einer Taste und der zugehoerigen
// Star-Citizen-Aktion.
//
// Key ist die Taste, die sctroll drueckt. Sie soll mit der uebereinstimmen, die
// im Spiel fuer SCAction gilt -- "Aus dem Spiel übernehmen" gleicht das ab.
// Geschrieben wird ins Spiel nichts.
type Action struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`

	RewardTitle string `json:"reward_title"`
	RewardCost  int    `json:"reward_cost"`
	RewardID    string `json:"reward_id,omitempty"`
	RewardColor string `json:"reward_color,omitempty"`

	Key           string       `json:"key"`
	HoldMs        int          `json:"hold_ms"`
	Repeat        int          `json:"repeat,omitempty"`
	RepeatDelayMs int          `json:"repeat_delay_ms,omitempty"`
	Steps         []ActionStep `json:"steps,omitempty"`

	KeyLock        KeyLockConfig `json:"key_lock"`
	Cooldown       int           `json:"cooldown_ms"`
	TwitchCooldown int           `json:"twitch_cooldown_sec,omitempty"`

	Category string `json:"category"`

	// SCActionMap/SCAction verweisen auf die Aktion in actionmaps.xml,
	// z.B. "spaceship_general" / "v_toggle_all_doors". Leer = kein Bind
	// noetig (etwa beim 360-Spin, der nur die Maus bewegt).
	SCActionMap string `json:"sc_actionmap,omitempty"`
	SCAction    string `json:"sc_action,omitempty"`

	Custom bool `json:"custom,omitempty"`
}

type TwitchConfig struct {
	ClientID string `json:"client_id"`

	// ClientSecret ist nur noetig, wenn die Twitch-App als "Confidential"
	// registriert ist. Solche Apps lehnen den Token-Refresh ohne Secret ab
	// ("missing client secret"), obwohl die Anmeldung selbst ohne funktioniert.
	//
	// Wird bewusst NICHT mitgeliefert: ein Secret in einer verteilten App waere
	// keins. Der saubere Weg ist eine App vom Typ "Public" -- die kann ohne
	// Secret erneuern. Dieses Feld ist fuer alle, die eine bestehende
	// Confidential-App weiterbenutzen wollen.
	ClientSecret string `json:"client_secret,omitempty"`

	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`

	// ExpiresAt ist der Ablauf des Access Tokens. Twitch gibt Nutzer-Tokens
	// nur ein paar Stunden -- ohne diesen Zeitpunkt merkt man den Ablauf erst,
	// wenn mitten im Stream nichts mehr ausgeloest wird.
	ExpiresAt time.Time `json:"expires_at,omitempty"`

	ChannelName   string `json:"channel_name"`
	BroadcasterID string `json:"broadcaster_id,omitempty"`
}

// HasLogin meldet, ob eine Anmeldung wiederhergestellt werden kann.
// Entscheidend ist allein der Refresh Token -- ein abgelaufener Access Token
// ist kein Grund, den Nutzer erneut anzumelden.
func (t TwitchConfig) HasLogin() bool {
	return t.RefreshToken != ""
}

type Config struct {
	mu           sync.RWMutex
	Twitch       TwitchConfig `json:"twitch"`
	Actions      []Action     `json:"actions"`
	TargetWindow string       `json:"target_window"`
	GlobalEnable bool         `json:"global_enable"`
	Language     string       `json:"language"`

	// InputMode legt fest, wie Tastendruecke gesendet werden:
	// "scancode" (Standard), "virtual" oder "both". Welches Verfahren ein Spiel
	// annimmt, laesst sich nicht vorhersagen -- deshalb umschaltbar.
	InputMode string `json:"input_mode,omitempty"`

	// SCDir ist der Channel-Ordner der Installation, z.B.
	// "D:\Games\Roberts Space Industries\StarCitizen\LIVE".
	// Leer = beim Start automatisch suchen.
	SCDir     string `json:"sc_dir,omitempty"`
	SCChannel string `json:"sc_channel,omitempty"`

	// MigratedRealKeys markiert, dass die einmalige Umstellung auf Star Citizens
	// echte Standardbelegungen gelaufen ist. Davor standen ausgedachte
	// RAlt-Kombinationen in der Konfiguration.
	MigratedRealKeys bool `json:"migrated_real_keys,omitempty"`

	// MigratedHolds markiert, dass zu kurze Haltezeiten einmalig angehoben
	// wurden. Betrifft vor allem die Selbstzerstoerung, die das Spiel rund
	// fuenfzehn Sekunden gehalten sehen will.
	MigratedHolds bool `json:"migrated_holds,omitempty"`
}

// Die Tasten hier sind Star Citizens echte Standardbelegungen aus der
// defaultProfile.xml, nicht ausgedacht. Haltezeiten folgen dem activationMode:
// press/tap kurz, delayed_press und Halteaktionen entsprechend laenger.
//
// Aktionen mit leerer Taste sind im Spiel ab Werk unbelegt (Tueren, Emotes) --
// die muss jeder selbst binden, "Aus dem Spiel übernehmen" holt sie dann aus
// der actionmaps.xml. Sie sind deshalb standardmaessig aus.
var defaultActions = []Action{
	// --- Schiff ---
	{ID: "doors", Name: "Türen auf/zu", Description: "Öffnet oder schließt alle Türen am Schiff.", Enabled: false,
		RewardTitle: "Türen auf/zu!", RewardCost: 300, Key: "", HoldMs: 80,
		Cooldown: 30000, Category: "ship", SCActionMap: "spaceship_general", SCAction: "v_toggle_all_doors"},
	{ID: "doorlocks", Name: "Türschlösser", Description: "Ver- oder entriegelt alle Türen am Schiff.", Enabled: false,
		RewardTitle: "Türschlösser!", RewardCost: 400, Key: "", HoldMs: 80,
		Cooldown: 60000, Category: "ship", SCActionMap: "spaceship_general", SCAction: "v_toggle_all_doorlocks"},
	{ID: "lights", Name: "Schiffslichter", Description: "Schaltet die Außenbeleuchtung des Schiffs an oder aus.", Enabled: true,
		RewardTitle: "Licht an/aus!", RewardCost: 150, Key: "l", HoldMs: 80,
		Cooldown: 20000, Category: "ship", SCActionMap: "lights_controller", SCAction: "v_lights"},
	{ID: "landing_gear", Name: "Fahrwerk", Description: "Fährt das Landegestell aus oder ein — auch mitten im Flug.", Enabled: true,
		RewardTitle: "Fahrwerk!", RewardCost: 250, Key: "n", HoldMs: 80,
		Cooldown: 30000, Category: "ship", SCActionMap: "spaceship_movement", SCAction: "v_toggle_landing_system"},
	{ID: "vtol", Name: "VTOL umschalten", Description: "Klappt die VTOL-Triebwerke um.", Enabled: false,
		RewardTitle: "VTOL!", RewardCost: 500, Key: "k", HoldMs: 80,
		Cooldown: 60000, Category: "ship", SCActionMap: "spaceship_movement", SCAction: "v_vtol_toggle"},
	{ID: "flightready", Name: "Flight Ready", Description: "Fährt die Systeme des Schiffs hoch oder runter.", Enabled: false,
		RewardTitle: "Flight Ready!", RewardCost: 800, Key: "ralt+r", HoldMs: 80,
		Cooldown: 120000, Category: "ship", SCActionMap: "spaceship_general", SCAction: "v_flightready"},

	// --- Flug ---
	// Master Modes haben die alten Quantum-/SCM-Umschalter abgeloest: kurz B
	// wechselt den Modus, lang B geht auf NAV/Quantum. Das alte
	// v_toggle_quantum_mode ist im Spiel unbelegte Altlast.
	{ID: "master_mode", Name: "Master Mode (NAV/Quantum)", Description: "Wechselt in den NAV-/Quantum-Modus. Mitten im Kampf eine schlechte Idee.", Enabled: true,
		RewardTitle: "NAV-Modus!", RewardCost: 400, Key: "b", HoldMs: 500,
		Cooldown: 60000, Category: "flight", SCActionMap: "spaceship_movement", SCAction: "v_master_mode_cycle_long"},
	{ID: "decoupled", Name: "Decoupled Mode", Description: "Entkoppelt die Steuerung — das Schiff driftet weiter, statt von allein abzubremsen.", Enabled: true,
		RewardTitle: "Decoupled!", RewardCost: 700, Key: "c", HoldMs: 80,
		Cooldown: 90000, Category: "flight", SCActionMap: "spaceship_movement", SCAction: "v_ifcs_vector_decoupling_toggle"},
	{ID: "afterburner", Name: "Boost", Description: "Zündet den Nachbrenner.", Enabled: false,
		RewardTitle: "Boost!", RewardCost: 500, Key: "lshift", HoldMs: 1500,
		Cooldown: 60000, Category: "flight", SCActionMap: "spaceship_movement", SCAction: "v_afterburner"},
	{ID: "decoy", Name: "Decoy werfen", Description: "Feuert Täuschkörper ab.", Enabled: false,
		RewardTitle: "Decoys raus!", RewardCost: 400, Key: "h", HoldMs: 800, Repeat: 3, RepeatDelayMs: 250,
		Cooldown: 60000, Category: "flight", SCActionMap: "spaceship_defensive", SCAction: "v_weapon_countermeasure_decoy_launch"},
	{ID: "gravity_comp", Name: "G-Kompensation", Description: "Schaltet die Gravitationskompensation um — das Schiff wird spürbar behäbiger.", Enabled: false,
		RewardTitle: "G-Comp!", RewardCost: 600, Key: "", HoldMs: 80,
		Cooldown: 90000, Category: "flight", SCActionMap: "spaceship_movement", SCAction: "v_ifcs_gravity_compensation_toggle"},

	// --- Energie ---
	{ID: "power_all", Name: "Energie togglen", Description: "Schaltet die gesamte Schiffsenergie an oder aus.", Enabled: false,
		RewardTitle: "Strom aus!", RewardCost: 3000, Key: "u", HoldMs: 80,
		Cooldown: 300000, Category: "power", SCActionMap: "spaceship_power", SCAction: "v_power_toggle"},
	{ID: "thrusters", Name: "Thruster togglen", Description: "Schaltet die Triebwerke an oder aus.", Enabled: false,
		RewardTitle: "Thruster aus!", RewardCost: 1500, Key: "i", HoldMs: 80,
		Cooldown: 180000, Category: "power", SCActionMap: "spaceship_power", SCAction: "v_power_toggle_thrusters"},
	{ID: "shields", Name: "Schilde togglen", Description: "Schaltet die Schilde an oder aus.", Enabled: false,
		RewardTitle: "Schilde aus!", RewardCost: 2000, Key: "o", HoldMs: 80,
		Cooldown: 180000, Category: "power", SCActionMap: "spaceship_power", SCAction: "v_power_toggle_shields"},
	{ID: "weapons", Name: "Waffen togglen", Description: "Schaltet die Waffen an oder aus.", Enabled: false,
		RewardTitle: "Waffen aus!", RewardCost: 2000, Key: "p", HoldMs: 80,
		Cooldown: 180000, Category: "power", SCActionMap: "spaceship_power", SCAction: "v_power_toggle_weapons"},

	// --- Sicht & Modi ---
	{ID: "scan_mode", Name: "Scan-Modus", Description: "Wechselt in den Scan-Modus.", Enabled: true,
		RewardTitle: "Scan-Modus!", RewardCost: 200, Key: "v", HoldMs: 80,
		Cooldown: 30000, Category: "view", SCActionMap: "seat_general", SCAction: "v_toggle_scan_mode"},
	{ID: "look_behind", Name: "Nach hinten schauen", Description: "Dreht die Kamera für einen Moment nach hinten.", Enabled: true,
		RewardTitle: "Umschauen!", RewardCost: 150, Key: "comma", HoldMs: 2000,
		Cooldown: 20000, Category: "view", SCActionMap: "seat_general", SCAction: "v_view_look_behind"},

	// --- Spieler ---
	{ID: "helmet", Name: "Helm ab", Description: "Nimmt den Helm ab oder setzt ihn auf. Im Vakuum eher ungünstig.", Enabled: true,
		RewardTitle: "Helm ab!", RewardCost: 500, Key: "lalt+h", HoldMs: 80,
		Cooldown: 60000, Category: "player", SCActionMap: "player", SCAction: "toggleAttachHelmet"},
	{ID: "flashlight", Name: "Taschenlampe", Description: "Schaltet die Taschenlampe an oder aus.", Enabled: true,
		RewardTitle: "Taschenlampe!", RewardCost: 100, Key: "t", HoldMs: 80,
		Cooldown: 15000, Category: "player", SCActionMap: "player", SCAction: "toggle_flashlight"},
	{ID: "exit_seat", Name: "Aus dem Sitz", Description: "Lässt den Piloten aufstehen beziehungsweise aussteigen.", Enabled: false,
		RewardTitle: "Aufstehen!", RewardCost: 2000, Key: "y", HoldMs: 900,
		Cooldown: 300000, Category: "player", SCActionMap: "default", SCAction: "pl_exit"},
	{ID: "reload", Name: "Nachladen", Description: "Lädt die Waffe nach — mitten im Gefecht besonders unpraktisch.", Enabled: true,
		RewardTitle: "Nachladen!", RewardCost: 300, Key: "r", HoldMs: 80,
		Cooldown: 30000, Category: "player", SCActionMap: "player", SCAction: "reload"},
	// Zweistufig: G wählt die Granate, die linke Maustaste wirft sie. Gehalten
	// wird beim Wurf, weil throw_overhand im Spiel eine Halteaktion ist --
	// je länger, desto weiter fliegt sie.
	{ID: "grenade", Name: "Granate werfen", Description: "Zieht eine Granate und wirft sie. Im Schiff eine ganz schlechte Idee.", Enabled: false,
		RewardTitle: "Granate!", RewardCost: 1500, Key: "g", HoldMs: 80,
		Steps: []ActionStep{
			{Key: "g", HoldMs: 80},
			{DelayMs: 700},
			{Key: "mouse1", HoldMs: 700},
		},
		Cooldown: 300000, Category: "player", SCActionMap: "player_choice", SCAction: "pc_qs_grenades"},
	{ID: "spin360", Name: "360 Spin", Description: "Dreht die Ansicht einmal komplett im Kreis.", Enabled: false,
		RewardTitle: "360 Spin!", RewardCost: 400, Key: "spin360", HoldMs: 8000,
		Cooldown: 30000, Category: "fun"},

	// --- Emotes: im Spiel alle ab Werk unbelegt ---
	{ID: "emote_dance", Name: "Emote: Tanzen", Description: "Der Charakter legt eine Tanzeinlage hin.", Enabled: false,
		RewardTitle: "Tanz!", RewardCost: 200, Key: "", HoldMs: 80,
		Cooldown: 30000, Category: "emote", SCActionMap: "player_emotes", SCAction: "emote_dance"},
	{ID: "emote_wave", Name: "Emote: Winken", Description: "Der Charakter winkt.", Enabled: false,
		RewardTitle: "Winken!", RewardCost: 100, Key: "", HoldMs: 80,
		Cooldown: 15000, Category: "emote", SCActionMap: "player_emotes", SCAction: "emote_wave"},
	{ID: "emote_salute", Name: "Emote: Salutieren", Description: "Der Charakter salutiert.", Enabled: false,
		RewardTitle: "Salut!", RewardCost: 100, Key: "", HoldMs: 80,
		Cooldown: 15000, Category: "emote", SCActionMap: "player_emotes", SCAction: "emote_salute"},
	{ID: "emote_chicken", Name: "Emote: Huhn", Description: "Der Charakter macht das Huhn.", Enabled: false,
		RewardTitle: "Huhn!", RewardCost: 300, Key: "", HoldMs: 80,
		Cooldown: 30000, Category: "emote", SCActionMap: "player_emotes", SCAction: "emote_chicken"},
	{ID: "emote_taunt", Name: "Emote: Verhöhnen", Description: "Der Charakter verhöhnt sein Gegenüber.", Enabled: false,
		RewardTitle: "Taunt!", RewardCost: 300, Key: "", HoldMs: 80,
		Cooldown: 30000, Category: "emote", SCActionMap: "player_emotes", SCAction: "emote_taunt"},

	// --- Gefaehrlich: standardmaessig aus, teuer, langer Cooldown ---
	{ID: "eject", Name: "Schleudersitz", Description: "Katapultiert den Piloten aus dem Schiff.", Enabled: false,
		RewardTitle: "EJECT!", RewardCost: 10000, Key: "ralt+y", HoldMs: 200,
		Cooldown: 900000, Category: "danger", SCActionMap: "seat_general", SCAction: "v_eject"},
	// Star Citizen will die Taste hier rund fünfzehn Sekunden gehalten sehen,
	// nicht bloß kurz gedrückt. Mit den 900 ms aus dem activationMode passierte
	// schlicht nichts.
	{ID: "self_destruct", Name: "Selbstzerstörung", Description: "Startet den Countdown zur Selbstzerstörung des Schiffs.", Enabled: false,
		RewardTitle: "SELBSTZERSTÖRUNG!", RewardCost: 25000, Key: "backspace", HoldMs: 15000,
		Cooldown: 1800000, Category: "danger", SCActionMap: "spaceship_general", SCAction: "v_self_destruct"},
}

// legacyActions sind alte Vorgaben, deren SCAction im Spiel nicht mehr belegt
// ist. Bestehende Konfigurationen werden beim Laden darauf umgezogen.
var legacyActions = map[string]string{
	"quantum_mode":  "master_mode", // v_toggle_quantum_mode -> v_master_mode_cycle_long
	"power_off":     "power_all",   // v_power_set_off -> v_power_toggle
	"speed_limiter": "",            // im Spiel ab Werk unbelegt, ersatzlos
	"nightvision":   "",            // v_light_amplification_toggle gibt es so nicht mehr
	"emote_flex":    "",
	"emote_sleep":   "",
	"emote_cry":     "",
}

func configPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(configDir, "SCTroll")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return DefaultConfig(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := DefaultConfig()
			_ = cfg.Save()
			return cfg, nil
		}
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return DefaultConfig(), nil
	}

	cfg.migrateLegacyActions()
	if !cfg.MigratedRealKeys {
		cfg.adoptShippedKeys()
		cfg.MigratedRealKeys = true
	}
	if !cfg.MigratedHolds {
		cfg.raiseTooShortHolds()
		cfg.MigratedHolds = true
	}
	cfg.sanitizeDescriptions()
	cfg.mergeNewActions()
	_ = cfg.Save()

	return &cfg, nil
}

// migrateLegacyActions zieht Aktionen um, deren Star-Citizen-Aktion es so nicht
// mehr gibt. Reward-ID, Preis und Cooldown wandern mit, damit der Reward auf
// Twitch nicht verwaist.
func (c *Config) migrateLegacyActions() {
	shipped := make(map[string]Action, len(defaultActions))
	for _, d := range defaultActions {
		shipped[d.ID] = d
	}
	have := make(map[string]bool, len(c.Actions))
	for _, a := range c.Actions {
		have[a.ID] = true
	}

	kept := make([]Action, 0, len(c.Actions))
	for _, a := range c.Actions {
		replID, isLegacy := legacyActions[a.ID]
		if a.Custom || !isLegacy {
			kept = append(kept, a)
			continue
		}
		if replID == "" {
			continue // ersatzlos gestrichen
		}
		d, ok := shipped[replID]
		if !ok || have[replID] {
			continue // Ersatz existiert schon
		}
		d.RewardID = a.RewardID
		d.RewardCost = a.RewardCost
		d.Cooldown = a.Cooldown
		d.TwitchCooldown = a.TwitchCooldown
		d.RewardColor = a.RewardColor
		d.Enabled = a.Enabled
		have[replID] = true
		kept = append(kept, d)
	}
	c.Actions = kept
}

// setupNotes sind Formulierungen, die sich an den Streamer richten und frueher
// in den Beschreibungen standen. Seit die Beschreibung als Text der Belohnung
// an Twitch geht, bekaeme jeder Zuschauer sie zu sehen.
var setupNotes = []string{
	"im Spiel ab Werk unbelegt",
	"Taste wird 15 Sekunden gehalten",
	"Langer Druck auf B",
}

// sanitizeDescriptions ersetzt Beschreibungen, in denen noch ein solcher
// Einrichtungshinweis steckt, durch die mitgelieferte Fassung.
//
// Bewusst nur diese: eine selbst geschriebene Beschreibung bleibt unangetastet.
// Laeuft bei jedem Laden, ist aber nach dem ersten Durchgang wirkungslos --
// deshalb braucht es keine Merkung.
func (c *Config) sanitizeDescriptions() {
	shipped := make(map[string]Action, len(defaultActions))
	for _, d := range defaultActions {
		shipped[d.ID] = d
	}

	for i := range c.Actions {
		a := &c.Actions[i]
		if a.Custom {
			continue
		}
		d, ok := shipped[a.ID]
		if !ok {
			continue
		}
		for _, note := range setupNotes {
			if strings.Contains(a.Description, note) {
				a.Description = d.Description
				break
			}
		}
	}
}

// raiseTooShortHolds hebt Haltezeiten an, die unter dem mitgelieferten Wert
// liegen.
//
// Noetig, weil einzelne Aktionen laenger gehalten werden muessen, als die
// Aktivierungsart im Spielprofil vermuten laesst -- die Selbstzerstoerung
// braucht rund fuenfzehn Sekunden. Nur anheben, nie senken: wer bewusst laenger
// eingestellt hat, behaelt seinen Wert.
func (c *Config) raiseTooShortHolds() {
	shipped := make(map[string]Action, len(defaultActions))
	for _, d := range defaultActions {
		shipped[d.ID] = d
	}
	for i := range c.Actions {
		a := &c.Actions[i]
		if a.Custom {
			continue
		}
		d, ok := shipped[a.ID]
		if ok && a.HoldMs < d.HoldMs {
			a.HoldMs = d.HoldMs
		}
	}
}

// adoptShippedKeys setzt Taste und Haltezeit der mitgelieferten Aktionen einmalig
// auf Star Citizens echte Standardwerte.
//
// Bis Version 1.0.2 standen dort erfundene RAlt-Kombinationen, die im Spiel auf
// nichts zeigten. Eigene Belegungen holt man danach mit "Aus dem Spiel
// übernehmen" wieder herein -- die stehen in der actionmaps.xml und gehen
// dadurch nicht verloren.
func (c *Config) adoptShippedKeys() {
	shipped := make(map[string]Action, len(defaultActions))
	for _, d := range defaultActions {
		shipped[d.ID] = d
	}
	for i := range c.Actions {
		a := &c.Actions[i]
		if a.Custom {
			continue
		}
		d, ok := shipped[a.ID]
		if !ok {
			continue
		}
		// Niemals leeren. Aktionen, die im Spiel ab Werk unbelegt sind (Emotes,
		// Tueren), haben hier keine Taste -- eine bereits aus dem Spielprofil
		// uebernommene Belegung darf davon nicht ueberschrieben werden.
		if d.Key != "" {
			a.Key = d.Key
			a.HoldMs = d.HoldMs
		}
	}
}

func DefaultConfig() *Config {
	actions := make([]Action, len(defaultActions))
	copy(actions, defaultActions)
	return &Config{
		Actions:      actions,
		TargetWindow: "StarCitizen",
		GlobalEnable: true,
		Language:     "de",
	}
}

// DefaultActions liefert eine Kopie der mitgelieferten Aktionen.
func DefaultActions() []Action {
	out := make([]Action, len(defaultActions))
	copy(out, defaultActions)
	return out
}

// mergeNewActions ergaenzt Aktionen, die in neueren Versionen dazugekommen sind,
// ohne bestehende Anpassungen des Nutzers zu ueberschreiben.
func (c *Config) mergeNewActions() {
	existing := make(map[string]int)
	for i, a := range c.Actions {
		if a.Custom {
			continue // eigene Aktionen bleiben unangetastet
		}
		existing[a.ID] = i
	}

	for _, da := range defaultActions {
		idx, found := existing[da.ID]
		if !found {
			c.Actions = append(c.Actions, da)
			continue
		}
		// Nur fehlende Verknuepfungen nachtragen, sonst nichts anfassen.
		a := &c.Actions[idx]
		if a.SCAction == "" {
			a.SCAction = da.SCAction
		}
		if a.SCActionMap == "" {
			a.SCActionMap = da.SCActionMap
		}
	}
}

func (c *Config) Save() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	path, err := configPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (c *Config) SetAction(action Action) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, a := range c.Actions {
		if a.ID == action.ID {
			c.Actions[i] = action
			return
		}
	}
	c.Actions = append(c.Actions, action)
}

func (c *Config) GetActions() []Action {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]Action, len(c.Actions))
	copy(result, c.Actions)
	return result
}

func (c *Config) ToggleAction(id string, enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, a := range c.Actions {
		if a.ID == id {
			c.Actions[i].Enabled = enabled
			return
		}
	}
}

func (c *Config) DeleteAction(id string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, a := range c.Actions {
		if a.ID == id {
			c.Actions = append(c.Actions[:i], c.Actions[i+1:]...)
			return true
		}
	}
	return false
}
