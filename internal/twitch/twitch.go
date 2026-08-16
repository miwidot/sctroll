package twitch

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"sctroll/internal/config"
	"sctroll/internal/debuglog"
)

// DefaultClientID ist die Twitch-App, die SCTroll ohne weitere Einrichtung nutzt.
// Registriert als Client Type "Public" -- nur solche Apps duerfen den Access
// Token ohne Client Secret erneuern. Ein Secret gibt es dafuer gar nicht, es
// kann also auch keins mitgeliefert werden oder abhanden kommen.
//
// Ueberschreibbar im Twitch-Tab, etwa wenn jemand seine eigene App benutzen will.
const DefaultClientID = "o9awit9ajxzn0s1ci2yyxjy8lv17h4"

// legacyClientIDs sind frueher mitgelieferte Apps, die abgeloest wurden.
//
// recddye... war als "Confidential" registriert und stammte aus dem
// Tarkov-Vorlaeufer. Solche Apps lehnen den Refresh ohne Secret ab -- die
// Anmeldung ging damit nach wenigen Stunden verloren.
var legacyClientIDs = map[string]bool{
	"recddyemjfl0xbklhcnukacerbz11n": true,
}

// MigrateLegacyApp stellt eine gespeicherte Konfiguration auf die aktuelle
// Standard-App um und meldet, ob etwas geaendert wurde.
//
// Tokens und Rewards gehoeren immer zu genau einer Client-ID: die Anmeldung wird
// deshalb zurueckgesetzt, und der Aufrufer muss die Reward-Verknuepfungen loesen.
// Ohne das benutzt eine bestehende Installation still die kaputte alte App weiter.
func MigrateLegacyApp(cfg *config.TwitchConfig) bool {
	if !legacyClientIDs[cfg.ClientID] {
		return false
	}

	debuglog.Log("twitch: alte App %s wird durch %s ersetzt", cfg.ClientID, DefaultClientID)
	cfg.ClientID = DefaultClientID
	cfg.ClientSecret = ""
	cfg.AccessToken = ""
	cfg.RefreshToken = ""
	cfg.ExpiresAt = time.Time{}
	return true
}

// Als Variablen, damit Tests sie auf einen lokalen Stub-Server zeigen lassen
// koennen -- die Token-Behandlung laesst sich sonst nicht pruefen.
var (
	twitchDeviceURL   = "https://id.twitch.tv/oauth2/device"
	twitchTokenURL    = "https://id.twitch.tv/oauth2/token"
	twitchValidateURL = "https://id.twitch.tv/oauth2/validate"
	twitchAPIURL      = "https://api.twitch.tv/helix"
)

const (
	eventSubWSURL = "wss://eventsub.wss.twitch.tv/ws?keepalive_timeout_seconds=30"
	scopes        = "channel:manage:redemptions channel:read:redemptions"
)

type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type Client struct {
	mu             sync.RWMutex
	cfg            *config.TwitchConfig
	clientID       string
	httpClient     *http.Client
	ws             *websocket.Conn
	sessionID      string
	connected      bool
	keepalive      time.Duration
	onRedemption   func(rewardID, redemptionID, userName, rewardTitle string)
	onConnect      func()
	onDisconnect   func(err error)
	onLog            func(msg string)
	onTokenRefresh   func()
	stopCh           chan struct{}
	stopOnce         sync.Once
	refresherRunning bool
}

type eventSubMessage struct {
	Metadata struct {
		MessageID   string `json:"message_id"`
		MessageType string `json:"message_type"`
	} `json:"metadata"`
	Payload json.RawMessage `json:"payload"`
}

type welcomePayload struct {
	Session struct {
		ID           string `json:"id"`
		Keepalive    int    `json:"keepalive_timeout_seconds"`
		ReconnectURL string `json:"reconnect_url"`
	} `json:"session"`
}

type redemptionPayload struct {
	Subscription struct {
		Type string `json:"type"`
	} `json:"subscription"`
	Event struct {
		ID       string `json:"id"` // Einloesungs-ID, noetig zum Erstatten
		UserName string `json:"user_name"`
		Reward   struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"reward"`
	} `json:"event"`
}

func NewClient(cfg *config.TwitchConfig) *Client {
	if cfg.ClientID == "" {
		cfg.ClientID = DefaultClientID
	}
	return &Client{
		cfg:        cfg,
		clientID:   cfg.ClientID,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		keepalive:  30 * time.Second,
		stopCh:     make(chan struct{}),
	}
}

func (c *Client) SetOnRedemption(fn func(rewardID, redemptionID, userName, rewardTitle string)) {
	c.onRedemption = fn
}

func (c *Client) SetOnConnect(fn func()) {
	c.onConnect = fn
}

func (c *Client) SetOnDisconnect(fn func(err error)) {
	c.onDisconnect = fn
}

func (c *Client) SetOnLog(fn func(msg string)) {
	c.onLog = fn
}

func (c *Client) SetOnTokenRefresh(fn func()) {
	c.onTokenRefresh = fn
}

func (c *Client) log(msg string) {
	if c.onLog != nil {
		c.onLog(msg)
	}
}

func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// --- Device Code Flow ---

// RequestDeviceCode initiates the Device Code Flow.
// Returns the user code and verification URI for the user to visit.
func (c *Client) RequestDeviceCode() (*DeviceCodeResponse, error) {
	data := url.Values{
		"client_id": {c.clientID},
		"scopes":    {scopes},
	}

	resp, err := c.httpClient.PostForm(twitchDeviceURL, data)
	if err != nil {
		return nil, fmt.Errorf("device code request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("device code request failed (status %d): %s", resp.StatusCode, string(body))
	}

	var dcr DeviceCodeResponse
	if err := json.Unmarshal(body, &dcr); err != nil {
		return nil, fmt.Errorf("failed to decode device code response: %w", err)
	}

	return &dcr, nil
}

// PollForToken polls Twitch until the user authorizes the device code.
func (c *Client) PollForToken(deviceCode string, interval int, expiresIn int) error {
	if interval < 1 {
		interval = 5
	}

	deadline := time.Now().Add(time.Duration(expiresIn) * time.Second)

	for time.Now().Before(deadline) {
		time.Sleep(time.Duration(interval) * time.Second)

		data := url.Values{
			"client_id":   {c.clientID},
			"scopes":      {scopes},
			"device_code": {deviceCode},
			"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
		}

		resp, err := c.httpClient.PostForm(twitchTokenURL, data)
		if err != nil {
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == 200 {
			var result struct {
				AccessToken  string `json:"access_token"`
				RefreshToken string `json:"refresh_token"`
				ExpiresIn    int    `json:"expires_in"`
			}
			if err := json.Unmarshal(body, &result); err != nil {
				return fmt.Errorf("failed to decode token: %w", err)
			}
			c.mu.Lock()
			c.cfg.AccessToken = result.AccessToken
			c.cfg.RefreshToken = result.RefreshToken
			c.cfg.ExpiresAt = expiryFrom(result.ExpiresIn)
			c.mu.Unlock()
			debuglog.Log("PollForToken: angemeldet, Token gültig bis %s",
				c.cfg.ExpiresAt.Format(time.RFC3339))
			return c.fetchBroadcasterID()
		}

		// Check for pending/slow_down
		var errResp struct {
			Message string `json:"message"`
		}
		json.Unmarshal(body, &errResp)

		if strings.Contains(errResp.Message, "authorization_pending") {
			c.log("Warte auf Autorisierung...")
			continue
		}
		if strings.Contains(errResp.Message, "slow_down") {
			interval += 5
			continue
		}

		// Some other error
		return fmt.Errorf("token poll error: %s", string(body))
	}

	return fmt.Errorf("device code expired")
}

func (c *Client) fetchBroadcasterID() error {
	req, _ := http.NewRequest("GET", twitchAPIURL+"/users", nil)
	req.Header.Set("Authorization", "Bearer "+c.cfg.AccessToken)
	req.Header.Set("Client-Id", c.clientID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		Data []struct {
			ID    string `json:"id"`
			Login string `json:"login"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}
	if len(result.Data) == 0 {
		return fmt.Errorf("no user data returned")
	}

	c.cfg.BroadcasterID = result.Data[0].ID
	c.cfg.ChannelName = result.Data[0].Login
	return nil
}

// --- Channel Point Rewards ---

type ExistingReward struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// GetExistingRewards fetches all custom rewards created by this app (only manageable ones).
func (c *Client) GetExistingRewards() ([]ExistingReward, error) {
	debuglog.Log("GetExistingRewards: broadcaster_id=%s", c.cfg.BroadcasterID)
	resp, err := c.doAuthorized(func() (*http.Request, error) {
		return http.NewRequest("GET",
			fmt.Sprintf("%s/channel_points/custom_rewards?broadcaster_id=%s&only_manageable_rewards=true",
				twitchAPIURL, c.cfg.BroadcasterID), nil)
	})
	if err != nil {
		debuglog.Log("GetExistingRewards: HTTP error: %s", err)
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	debuglog.Log("GetExistingRewards: status=%d body=%s", resp.StatusCode, string(respBody))
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get rewards (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Data []ExistingReward `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}
	debuglog.Log("GetExistingRewards: found %d rewards", len(result.Data))
	for _, r := range result.Data {
		debuglog.Log("  existing reward: title=%q id=%s", r.Title, r.ID)
	}
	return result.Data, nil
}

// SetAllRewardsEnabled enables or disables all manageable rewards.
func (c *Client) SetAllRewardsEnabled(enabled bool) error {
	rewards, err := c.GetExistingRewards()
	if err != nil {
		return err
	}
	for _, r := range rewards {
		c.UpdateRewardEnabled(r.ID, enabled, "")
	}
	return nil
}

func (c *Client) CreateReward(title string, cost int, cooldownMs int, color string) (string, error) {
	debuglog.Log("CreateReward: title=%q cost=%d cooldown=%dms color=%s broadcaster_id=%s", title, cost, cooldownMs, color, c.cfg.BroadcasterID)
	body := map[string]interface{}{
		"title":                  title,
		"cost":                   cost,
		"is_enabled":             true,
		"is_user_input_required": false,
		// Muss false bleiben: mit true gilt eine Einloesung sofort als erledigt
		// und laesst sich nicht mehr erstatten. Genau das braucht sctroll aber,
		// wenn das Spiel gerade nicht im Vordergrund ist.
		// (Der Feldname lautet ...skip_request_queue -- "skip_queue" ignoriert Twitch.)
		"should_redemptions_skip_request_queue": false,
	}
	// Twitch requires minimum 60 seconds for global cooldown
	cooldownSec := cooldownMs / 1000
	if cooldownSec >= 60 {
		body["is_global_cooldown_enabled"] = true
		body["global_cooldown_seconds"] = cooldownSec
	} else if cooldownSec > 0 {
		body["is_global_cooldown_enabled"] = true
		body["global_cooldown_seconds"] = 60
	}
	// Set reward color if specified (hex format like #9146FF)
	if color != "" {
		body["background_color"] = color
	}
	jsonBody, _ := json.Marshal(body)

	resp, err := c.doAuthorized(func() (*http.Request, error) {
		req, err := http.NewRequest("POST",
			fmt.Sprintf("%s/channel_points/custom_rewards?broadcaster_id=%s", twitchAPIURL, c.cfg.BroadcasterID),
			bytes.NewReader(jsonBody))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	})
	if err != nil {
		debuglog.Log("CreateReward: HTTP error: %s", err)
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	debuglog.Log("CreateReward: status=%d body=%s", resp.StatusCode, string(respBody))
	if resp.StatusCode != 200 {
		// Ein gleichnamiger Reward existiert schon auf dem Kanal, gehoert aber
		// nicht dieser App -- typischerweise ein Rest einer frueheren
		// App-Registrierung. Twitch laesst ihn weder anlegen noch verwalten.
		upper := strings.ToUpper(string(respBody))
		if strings.Contains(upper, "DUPLICATE_REWARD") {
			return "", fmt.Errorf("%w: %q", ErrRewardExists, title)
		}
		if strings.Contains(upper, "TOO_MANY_REWARDS") {
			return "", ErrTooManyRewards
		}
		return "", fmt.Errorf("failed to create reward (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}
	if len(result.Data) == 0 {
		return "", fmt.Errorf("no reward data returned")
	}

	debuglog.Log("CreateReward: success id=%s", result.Data[0].ID)
	return result.Data[0].ID, nil
}

func (c *Client) DeleteReward(rewardID string) error {
	debuglog.Log("DeleteReward: id=%s broadcaster_id=%s", rewardID, c.cfg.BroadcasterID)
	resp, err := c.doAuthorized(func() (*http.Request, error) {
		return http.NewRequest("DELETE",
			fmt.Sprintf("%s/channel_points/custom_rewards?broadcaster_id=%s&id=%s",
				twitchAPIURL, c.cfg.BroadcasterID, rewardID), nil)
	})
	if err != nil {
		debuglog.Log("DeleteReward: HTTP error: %s", err)
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	debuglog.Log("DeleteReward: status=%d body=%s", resp.StatusCode, string(body))
	return nil
}

// SetRedemptionStatus setzt eine Einloesung auf FULFILLED oder CANCELED.
// CANCELED gibt dem Zuschauer seine Kanalpunkte zurueck -- das passiert, wenn
// die Aktion nicht ausgefuehrt werden konnte.
//
// Twitch erlaubt das nur fuer Rewards, die dieselbe Client-ID angelegt hat.
// Von Hand im Dashboard erstellte Rewards lassen sich nicht erstatten.
func (c *Client) SetRedemptionStatus(rewardID, redemptionID, status string) error {
	jsonBody, _ := json.Marshal(map[string]string{"status": status})

	resp, err := c.doAuthorized(func() (*http.Request, error) {
		req, err := http.NewRequest("PATCH",
			fmt.Sprintf("%s/channel_points/custom_rewards/redemptions?broadcaster_id=%s&reward_id=%s&id=%s",
				twitchAPIURL, c.cfg.BroadcasterID, rewardID, redemptionID),
			bytes.NewReader(jsonBody))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		debuglog.Log("SetRedemptionStatus: status=%d body=%s", resp.StatusCode, string(respBody))
		return fmt.Errorf("redemption status update failed (status %d)", resp.StatusCode)
	}
	debuglog.Log("SetRedemptionStatus: %s -> %s", redemptionID, status)
	return nil
}

func (c *Client) UpdateRewardEnabled(rewardID string, enabled bool, color string) error {
	return c.UpdateReward(rewardID, map[string]interface{}{
		"is_enabled": enabled,
	}, color)
}

func (c *Client) UpdateReward(rewardID string, fields map[string]interface{}, color string) error {
	if color != "" {
		fields["background_color"] = color
	}
	jsonBody, _ := json.Marshal(fields)

	resp, err := c.doAuthorized(func() (*http.Request, error) {
		req, err := http.NewRequest("PATCH",
			fmt.Sprintf("%s/channel_points/custom_rewards?broadcaster_id=%s&id=%s",
				twitchAPIURL, c.cfg.BroadcasterID, rewardID),
			bytes.NewReader(jsonBody))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		debuglog.Log("UpdateReward: status=%d body=%s", resp.StatusCode, string(respBody))
		// 404 heisst: die gespeicherte ID gehoert zu keinem Reward mehr. Der
		// Aufrufer muss die Verknuepfung verwerfen statt es erneut zu versuchen.
		if resp.StatusCode == 404 {
			return ErrRewardNotFound
		}
		return fmt.Errorf("update reward failed (status %d)", resp.StatusCode)
	}
	return nil
}

// --- EventSub WebSocket ---

func (c *Client) Connect() error {
	// Erst den Token in Ordnung bringen, dann verbinden. Ohne das schlaegt die
	// Anmeldung der Subscription fehl und der ganze Verbindungsaufbau bricht ab.
	if err := c.EnsureValidToken(); err != nil {
		return err
	}
	c.log("Verbinde mit Twitch EventSub...")
	return c.connectTo(eventSubWSURL, true)
}

// connectTo baut eine EventSub-Verbindung auf und startet den Read-Loop.
//
// subscribe=false wird bei einem session_reconnect benutzt: Twitch uebertraegt
// die bestehenden Subscriptions auf die neue Session, ein erneutes Anmelden
// wuerde in einem Duplikat enden.
func (c *Client) connectTo(wsURL string, subscribe bool) error {
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("websocket connection failed: %w", err)
	}

	// Das Welcome muss zuegig kommen, sonst stimmt etwas nicht.
	ws.SetReadDeadline(time.Now().Add(30 * time.Second))
	_, msgBytes, err := ws.ReadMessage()
	if err != nil {
		ws.Close()
		return fmt.Errorf("failed to read welcome: %w", err)
	}

	var msg eventSubMessage
	if err := json.Unmarshal(msgBytes, &msg); err != nil {
		ws.Close()
		return fmt.Errorf("failed to parse welcome: %w", err)
	}
	if msg.Metadata.MessageType != "session_welcome" {
		ws.Close()
		return fmt.Errorf("expected welcome, got: %s", msg.Metadata.MessageType)
	}

	var welcome welcomePayload
	json.Unmarshal(msg.Payload, &welcome)

	c.mu.Lock()
	c.sessionID = welcome.Session.ID
	if welcome.Session.Keepalive > 0 {
		c.keepalive = time.Duration(welcome.Session.Keepalive) * time.Second
	}
	c.mu.Unlock()

	if subscribe {
		if err := c.subscribeToRedemptions(); err != nil {
			ws.Close()
			return err
		}
	}

	c.mu.Lock()
	c.ws = ws
	c.connected = true
	c.mu.Unlock()

	c.log(fmt.Sprintf("Verbunden (Session %s)", welcome.Session.ID))
	if subscribe && c.onConnect != nil {
		c.onConnect()
	}

	go c.readLoop(ws)
	return nil
}

func (c *Client) subscribeToRedemptions() error {
	body := map[string]interface{}{
		"type":    "channel.channel_points_custom_reward_redemption.add",
		"version": "1",
		"condition": map[string]string{
			"broadcaster_user_id": c.cfg.BroadcasterID,
		},
		"transport": map[string]string{
			"method":     "websocket",
			"session_id": c.sessionID,
		},
	}
	jsonBody, _ := json.Marshal(body)

	// Ueber doAuthorized, damit ein abgelaufener Token einmal erneuert und der
	// Aufruf wiederholt wird. Frueher lief das an der Erneuerung vorbei -- ein
	// abgelaufener Token liess dann den ganzen Verbindungsaufbau scheitern.
	resp, err := c.doAuthorized(func() (*http.Request, error) {
		req, err := http.NewRequest("POST", twitchAPIURL+"/eventsub/subscriptions", bytes.NewReader(jsonBody))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 202 {
		return fmt.Errorf("failed to subscribe (status %d): %s", resp.StatusCode, string(respBody))
	}

	c.log("Auf Kanalpunkt-Einlösungen angemeldet")
	return nil
}

// readLoop verarbeitet eine WebSocket-Verbindung bis zu ihrem Ende.
// ws wird bewusst als Parameter uebergeben und nicht ueber c.ws gelesen: bei
// einem Reconnect zeigt c.ws bereits auf die neue Verbindung, waehrend dieser
// Loop noch die alte leerraeumt.
func (c *Client) readLoop(ws *websocket.Conn) {
	disconnected := func(err error) {
		c.mu.Lock()
		if c.ws == ws {
			c.connected = false
		}
		c.mu.Unlock()
		if c.onDisconnect != nil {
			c.onDisconnect(err)
		}
	}

	for {
		// Twitch schickt spaetestens alle keepalive-Sekunden etwas. Bleibt das
		// aus, ist die Verbindung tot -- ohne Deadline wuerde ReadMessage
		// stundenlang haengen und der Reconnect nie anlaufen.
		c.mu.RLock()
		ka := c.keepalive
		c.mu.RUnlock()
		ws.SetReadDeadline(time.Now().Add(ka + 10*time.Second))

		_, msgBytes, err := ws.ReadMessage()
		if err != nil {
			disconnected(err)
			return
		}

		var msg eventSubMessage
		if err := json.Unmarshal(msgBytes, &msg); err != nil {
			continue
		}

		switch msg.Metadata.MessageType {
		case "session_keepalive":
			// Nichts zu tun -- hat schon die Read-Deadline zurueckgesetzt.

		case "notification":
			c.handleNotification(msg.Payload)

		case "session_reconnect":
			var welcome welcomePayload
			if err := json.Unmarshal(msg.Payload, &welcome); err != nil || welcome.Session.ReconnectURL == "" {
				disconnected(fmt.Errorf("reconnect ohne URL"))
				return
			}
			c.log("Twitch bittet um Reconnect...")
			if err := c.connectTo(welcome.Session.ReconnectURL, false); err != nil {
				disconnected(fmt.Errorf("reconnect fehlgeschlagen: %w", err))
				return
			}
			// Die neue Verbindung laeuft, diese hier wird nur noch geschlossen.
			ws.Close()
			return

		case "revocation":
			disconnected(fmt.Errorf("subscription widerrufen (token oder scope weg?)"))
			return
		}
	}
}

func (c *Client) handleNotification(payload json.RawMessage) {
	var redemption redemptionPayload
	if err := json.Unmarshal(payload, &redemption); err != nil {
		return
	}

	if !strings.Contains(redemption.Subscription.Type, "redemption") {
		return
	}

	c.log(fmt.Sprintf("Redemption: %s by %s", redemption.Event.Reward.Title, redemption.Event.UserName))

	if c.onRedemption != nil {
		c.onRedemption(redemption.Event.Reward.ID, redemption.Event.ID, redemption.Event.UserName, redemption.Event.Reward.Title)
	}
}

func (c *Client) Disconnect() {
	// Beendet auch die Hintergrund-Erneuerung. sync.Once, weil ein zweites
	// Schliessen des Kanals das Programm abbrechen wuerde.
	c.stopOnce.Do(func() { close(c.stopCh) })

	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = false
	if c.ws != nil {
		c.ws.Close()
		c.ws = nil
	}
}

// doAuthorized executes a request with current Bearer token. If the response
// is 401, it tries to refresh the access token once and re-runs the request.
// The provided buildReq function must be able to build the request fresh on retry.
func (c *Client) doAuthorized(buildReq func() (*http.Request, error)) (*http.Response, error) {
	req, err := buildReq()
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.AccessToken)
	req.Header.Set("Client-Id", c.clientID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 401 {
		return resp, nil
	}

	// 401 — einmal erneuern und den Aufruf wiederholen.
	resp.Body.Close()
	if err := c.RefreshAccessToken(); err != nil {
		return nil, err // ErrLoginRequired bleibt erkennbar
	}
	c.log("Access Token war abgelaufen, wurde automatisch erneuert")

	req2, err := buildReq()
	if err != nil {
		return nil, err
	}
	req2.Header.Set("Authorization", "Bearer "+c.cfg.AccessToken)
	req2.Header.Set("Client-Id", c.clientID)
	return c.httpClient.Do(req2)
}

// ErrLoginRequired heisst: der Refresh Token wurde von Twitch endgueltig
// abgelehnt. Nur dann muss sich der Nutzer neu anmelden.
var ErrLoginRequired = errors.New("Twitch-Anmeldung abgelaufen — bitte neu verbinden")

// ErrClientSecretRequired heisst: die Twitch-App ist als "Confidential"
// registriert und verlangt beim Erneuern ein Client Secret.
//
// Das ist kein abgelaufener Token, sondern ein Einrichtungsproblem -- der
// Refresh Token ist voellig in Ordnung. Eine Neuanmeldung hilft nur bis zum
// naechsten Ablauf und faellt dann wieder auf die Nase.
// ErrRewardExists heisst: auf dem Kanal liegt bereits ein Reward mit diesem
// Titel, der einer anderen Anwendung gehoert.
//
// Twitch erlaubt einer App nur, ihre eigenen Rewards zu verwalten. Ein solcher
// Fremd-Reward laesst sich also weder anlegen noch loeschen noch verknuepfen --
// Einloesungen darauf laufen ins Leere. Nur der Kanalinhaber kann ihn im
// Twitch-Dashboard entfernen.
var ErrRewardExists = errors.New("Reward mit diesem Titel existiert bereits auf dem Kanal")

// ErrRewardNotFound heisst: die gespeicherte Reward-ID zeigt ins Leere -- der
// Reward wurde auf Twitch geloescht.
//
// Das passiert im Alltag staendig, etwa wenn jemand im Dashboard aufraeumt. Die
// Verknuepfung ist dann veraltet und muss verworfen werden, sonst bleibt die
// Aktion dauerhaft ohne Reward und laesst sich nie wieder ausloesen.
var ErrRewardNotFound = errors.New("Reward existiert auf Twitch nicht mehr")

// ErrTooManyRewards heisst: der Kanal hat die von Twitch erlaubte Zahl eigener
// Kanalpunkt-Belohnungen erreicht.
//
// Betrifft alle Belohnungen des Kanals, nicht nur die von SCTroll. Wer viele
// eigene angelegt hat, kann nicht alle Aktionen gleichzeitig aktivieren.
var ErrTooManyRewards = errors.New("Twitch erlaubt keine weiteren Kanalpunkt-Belohnungen auf diesem Kanal")

var ErrClientSecretRequired = errors.New(
	"Twitch-App verlangt ein Client Secret zum Erneuern. Entweder den Client Type der App " +
		"auf \"Public\" stellen (dev.twitch.tv/console/apps) oder das Secret im Twitch-Tab hinterlegen")

// refreshSkew ist der Vorlauf, mit dem erneuert wird. Ein Token, der in fuenf
// Minuten ablaeuft, ueberlebt keine laengere Aktion mehr.
const refreshSkew = 15 * time.Minute

// RefreshAccessToken holt einen neuen Access Token.
//
// Wichtig: bei einem Fehler bleiben die gespeicherten Zugangsdaten unveraendert.
// Eine frueherer Fassung hat die Antwort ungeprueft uebernommen und bei jedem
// Fehler -- auch bei einem kurzen Netzausfall oder einer 500 von Twitch -- leere
// Strings gespeichert. Damit war der Refresh Token weg und die Anmeldung bei
// jedem Start faellig.
func (c *Client) RefreshAccessToken() error {
	c.mu.RLock()
	refreshToken := c.cfg.RefreshToken
	secret := c.cfg.ClientSecret
	c.mu.RUnlock()

	if refreshToken == "" {
		return ErrLoginRequired
	}

	form := url.Values{
		"client_id":     {c.clientID},
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}
	// Nur mitschicken, wenn hinterlegt. Apps vom Typ "Public" brauchen keins;
	// "Confidential"-Apps lehnen ohne ab.
	if secret != "" {
		form.Set("client_secret", secret)
	}

	resp, err := c.httpClient.PostForm(twitchTokenURL, form)
	if err != nil {
		return fmt.Errorf("Token-Refresh nicht erreichbar: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		debuglog.Log("RefreshAccessToken: status=%d body=%s", resp.StatusCode, string(body))

		// Fehlendes Secret ist kein abgelaufener Token, sondern eine falsch
		// registrierte App. Eine Neuanmeldung wuerde das Problem nur bis zum
		// naechsten Ablauf verdecken.
		if strings.Contains(strings.ToLower(string(body)), "client secret") {
			return ErrClientSecretRequired
		}

		// 400/401 heisst: der Refresh Token gilt nicht mehr. Alles andere
		// (429, 5xx, Wartungsfenster) ist voruebergehend -- dann bleibt die
		// Anmeldung stehen und der naechste Versuch klappt wieder.
		if resp.StatusCode == 400 || resp.StatusCode == 401 {
			return ErrLoginRequired
		}
		return fmt.Errorf("Token-Refresh fehlgeschlagen (HTTP %d)", resp.StatusCode)
	}

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("Token-Antwort nicht lesbar: %w", err)
	}
	if result.AccessToken == "" {
		return fmt.Errorf("Token-Antwort ohne access_token")
	}

	c.mu.Lock()
	c.cfg.AccessToken = result.AccessToken
	// Twitch rotiert den Refresh Token nicht immer mit. Fehlt er, gilt der alte
	// weiter -- ueberschreiben wuerde die Anmeldung loeschen.
	if result.RefreshToken != "" {
		c.cfg.RefreshToken = result.RefreshToken
	}
	c.cfg.ExpiresAt = expiryFrom(result.ExpiresIn)
	c.mu.Unlock()

	debuglog.Log("RefreshAccessToken: ok, gültig bis %s", c.cfg.ExpiresAt.Format(time.RFC3339))
	if c.onTokenRefresh != nil {
		c.onTokenRefresh()
	}
	return nil
}

// expiryFrom rechnet Twitchs expires_in in einen Zeitpunkt um. Fehlt die Angabe,
// wird konservativ mit einer Stunde gerechnet, damit trotzdem erneuert wird.
func expiryFrom(expiresIn int) time.Time {
	if expiresIn <= 0 {
		expiresIn = 3600
	}
	return time.Now().Add(time.Duration(expiresIn) * time.Second)
}

// EnsureValidToken erneuert den Access Token, wenn er abgelaufen ist oder bald
// ablaeuft. Ist er noch lange gueltig, passiert nichts.
func (c *Client) EnsureValidToken() error {
	c.mu.RLock()
	access := c.cfg.AccessToken
	expires := c.cfg.ExpiresAt
	c.mu.RUnlock()

	if access != "" && !expires.IsZero() && time.Until(expires) > refreshSkew {
		return nil
	}
	return c.RefreshAccessToken()
}

// StartTokenRefresher erneuert den Token im Hintergrund, bevor er ablaeuft.
//
// Ohne das faellt ein Ablauf erst auf, wenn mitten im Stream keine Einloesung
// mehr ankommt: die EventSub-Verbindung meldet keinen 401, sie wird still
// widerrufen.
func (c *Client) StartTokenRefresher() {
	c.mu.Lock()
	if c.refresherRunning {
		c.mu.Unlock()
		return // mehrfaches Starten wuerde parallele Erneuerungen ausloesen
	}
	c.refresherRunning = true
	c.mu.Unlock()

	go func() {
		defer func() {
			c.mu.Lock()
			c.refresherRunning = false
			c.mu.Unlock()
		}()
		for {
			c.mu.RLock()
			expires := c.cfg.ExpiresAt
			c.mu.RUnlock()

			wait := time.Until(expires) - refreshSkew
			if expires.IsZero() || wait < time.Minute {
				wait = time.Minute
			}

			select {
			case <-c.stopCh:
				return
			case <-time.After(wait):
			}

			if err := c.EnsureValidToken(); err != nil {
				c.log(fmt.Sprintf("Token-Erneuerung: %s", err))
				if errors.Is(err, ErrLoginRequired) {
					return
				}
			}
		}
	}()
}

// Validate prueft den gespeicherten Token bei Twitch und holt dabei die
// Restlaufzeit sowie Kanal und Broadcaster-ID.
func (c *Client) Validate() error {
	c.mu.RLock()
	access := c.cfg.AccessToken
	c.mu.RUnlock()
	if access == "" {
		return ErrLoginRequired
	}

	req, _ := http.NewRequest("GET", twitchValidateURL, nil)
	req.Header.Set("Authorization", "OAuth "+access)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return ErrLoginRequired
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("Token-Prüfung fehlgeschlagen (HTTP %d)", resp.StatusCode)
	}

	var v struct {
		UserID    string `json:"user_id"`
		Login     string `json:"login"`
		ExpiresIn int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return err
	}

	c.mu.Lock()
	if v.UserID != "" {
		c.cfg.BroadcasterID = v.UserID
		c.cfg.ChannelName = v.Login
	}
	c.cfg.ExpiresAt = expiryFrom(v.ExpiresIn)
	c.mu.Unlock()

	debuglog.Log("Validate: %s, noch %ds gültig", v.Login, v.ExpiresIn)
	return nil
}
