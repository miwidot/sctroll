package twitch

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"sctroll/internal/config"
)

// stubToken laesst twitchTokenURL auf einen lokalen Server zeigen.
func stubToken(t *testing.T, status int, body string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	old := twitchTokenURL
	twitchTokenURL = srv.URL
	t.Cleanup(func() { twitchTokenURL = old })
}

func testClient(cfg *config.TwitchConfig) *Client {
	c := NewClient(cfg)
	c.httpClient = &http.Client{Timeout: 5 * time.Second}
	return c
}

// Der schlimmste Fehler, den dieses Programm hatte: bei einer Fehlerantwort hat
// der Refresh leere Strings gespeichert und Erfolg gemeldet. Danach war die
// Anmeldung dauerhaft weg -- ausgeloest schon von einem kurzen Netzausfall.
func TestRefreshKeepsCredentialsOnServerError(t *testing.T) {
	stubToken(t, 500, `{"message":"internal"}`)

	cfg := &config.TwitchConfig{
		AccessToken:  "alt-access",
		RefreshToken: "alt-refresh",
	}
	err := testClient(cfg).RefreshAccessToken()

	if err == nil {
		t.Fatal("HTTP 500 muss als Fehler durchschlagen")
	}
	if errors.Is(err, ErrLoginRequired) {
		t.Error("500 ist vorübergehend und darf keine Neuanmeldung verlangen")
	}
	if cfg.AccessToken != "alt-access" || cfg.RefreshToken != "alt-refresh" {
		t.Errorf("Zugangsdaten angetastet: access=%q refresh=%q", cfg.AccessToken, cfg.RefreshToken)
	}
}

// Nur ein von Twitch endgueltig abgelehnter Token rechtfertigt eine Neuanmeldung.
func TestRefreshReportsLoginRequiredOnRejection(t *testing.T) {
	stubToken(t, 400, `{"status":400,"message":"Invalid refresh token"}`)

	cfg := &config.TwitchConfig{AccessToken: "alt-access", RefreshToken: "kaputt"}
	err := testClient(cfg).RefreshAccessToken()

	if !errors.Is(err, ErrLoginRequired) {
		t.Fatalf("erwartet ErrLoginRequired, bekam %v", err)
	}
	// Auch hier nichts loeschen: der Nutzer soll sehen, dass eine Anmeldung
	// existierte, statt kommentarlos abgemeldet zu sein.
	if cfg.RefreshToken != "kaputt" {
		t.Errorf("Refresh Token wurde gelöscht: %q", cfg.RefreshToken)
	}
}

// Twitch rotiert den Refresh Token nicht immer mit. Fehlt er in der Antwort,
// muss der alte weitergelten.
func TestRefreshKeepsOldRefreshTokenWhenOmitted(t *testing.T) {
	stubToken(t, 200, `{"access_token":"neu-access","expires_in":14400}`)

	cfg := &config.TwitchConfig{AccessToken: "alt-access", RefreshToken: "alt-refresh"}
	if err := testClient(cfg).RefreshAccessToken(); err != nil {
		t.Fatal(err)
	}

	if cfg.AccessToken != "neu-access" {
		t.Errorf("Access Token ist %q", cfg.AccessToken)
	}
	if cfg.RefreshToken != "alt-refresh" {
		t.Errorf("Refresh Token ist %q, alter muss erhalten bleiben", cfg.RefreshToken)
	}
	if d := time.Until(cfg.ExpiresAt); d < 3*time.Hour || d > 5*time.Hour {
		t.Errorf("Ablauf in %s, erwartet ~4h", d)
	}
}

func TestRefreshRotatesRefreshToken(t *testing.T) {
	stubToken(t, 200, `{"access_token":"a2","refresh_token":"r2","expires_in":100}`)

	cfg := &config.TwitchConfig{AccessToken: "a1", RefreshToken: "r1"}
	if err := testClient(cfg).RefreshAccessToken(); err != nil {
		t.Fatal(err)
	}
	if cfg.RefreshToken != "r2" {
		t.Errorf("Refresh Token ist %q, erwartet r2", cfg.RefreshToken)
	}
}

// Eine als "Confidential" registrierte Twitch-App lehnt den Refresh ohne Secret
// ab. Das ist ein Einrichtungsproblem, keine abgelaufene Anmeldung -- wer es als
// Ablauf behandelt, schickt den Nutzer in eine Endlosschleife aus Neuanmeldungen,
// die jeweils nur bis zum naechsten Ablauf halten.
func TestRefreshReportsMissingClientSecret(t *testing.T) {
	stubToken(t, 400, `{"status":400,"message":"missing client secret"}`)

	cfg := &config.TwitchConfig{AccessToken: "a", RefreshToken: "r"}
	err := testClient(cfg).RefreshAccessToken()

	if !errors.Is(err, ErrClientSecretRequired) {
		t.Fatalf("erwartet ErrClientSecretRequired, bekam %v", err)
	}
	if errors.Is(err, ErrLoginRequired) {
		t.Error("darf nicht als abgelaufene Anmeldung durchgehen")
	}
	if cfg.RefreshToken != "r" {
		t.Errorf("Refresh Token angetastet: %q", cfg.RefreshToken)
	}
}

// Ist ein Secret hinterlegt, muss es mitgeschickt werden -- sonst laesst sich
// eine bestehende Confidential-App gar nicht weiterbenutzen.
func TestRefreshSendsClientSecretWhenSet(t *testing.T) {
	var gotSecret string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotSecret = r.PostForm.Get("client_secret")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"a2","expires_in":14400}`))
	}))
	defer srv.Close()
	old := twitchTokenURL
	twitchTokenURL = srv.URL
	defer func() { twitchTokenURL = old }()

	cfg := &config.TwitchConfig{RefreshToken: "r", ClientSecret: "geheim"}
	if err := testClient(cfg).RefreshAccessToken(); err != nil {
		t.Fatal(err)
	}
	if gotSecret != "geheim" {
		t.Errorf("client_secret kam als %q an", gotSecret)
	}
}

// Ohne hinterlegtes Secret darf das Feld gar nicht erst mitgeschickt werden:
// ein leeres client_secret laesst Public-Apps scheitern.
func TestRefreshOmitsEmptyClientSecret(t *testing.T) {
	var present bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		_, present = r.PostForm["client_secret"]
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"a2","expires_in":14400}`))
	}))
	defer srv.Close()
	old := twitchTokenURL
	twitchTokenURL = srv.URL
	defer func() { twitchTokenURL = old }()

	if err := testClient(&config.TwitchConfig{RefreshToken: "r"}).RefreshAccessToken(); err != nil {
		t.Fatal(err)
	}
	if present {
		t.Error("client_secret wurde ohne Wert mitgeschickt")
	}
}

// Ohne Refresh Token gibt es nichts zu erneuern -- das ist eine Neuanmeldung,
// kein Netzwerkfehler.
func TestRefreshWithoutTokenNeedsLogin(t *testing.T) {
	if err := testClient(&config.TwitchConfig{}).RefreshAccessToken(); !errors.Is(err, ErrLoginRequired) {
		t.Fatalf("erwartet ErrLoginRequired, bekam %v", err)
	}
}

// Ein noch lange gueltiger Token darf keinen Netzaufruf ausloesen.
func TestEnsureValidTokenSkipsWhenFresh(t *testing.T) {
	stubToken(t, 500, `{"message":"darf nicht aufgerufen werden"}`)

	cfg := &config.TwitchConfig{
		AccessToken:  "frisch",
		RefreshToken: "r",
		ExpiresAt:    time.Now().Add(2 * time.Hour),
	}
	if err := testClient(cfg).EnsureValidToken(); err != nil {
		t.Fatalf("gültiger Token sollte unangetastet bleiben, bekam %v", err)
	}
}

// Kurz vor Ablauf muss vorsorglich erneuert werden, nicht erst beim ersten 401.
func TestEnsureValidTokenRefreshesBeforeExpiry(t *testing.T) {
	stubToken(t, 200, `{"access_token":"erneuert","expires_in":14400}`)

	cfg := &config.TwitchConfig{
		AccessToken:  "laeuft-ab",
		RefreshToken: "r",
		ExpiresAt:    time.Now().Add(2 * time.Minute),
	}
	if err := testClient(cfg).EnsureValidToken(); err != nil {
		t.Fatal(err)
	}
	if cfg.AccessToken != "erneuert" {
		t.Errorf("Access Token ist %q, hätte erneuert werden müssen", cfg.AccessToken)
	}
}

// Bestehende Installationen tragen die alte, als "Confidential" registrierte App
// in ihrer Konfiguration. Ohne Umstellung benutzen sie die still weiter und
// verlieren die Anmeldung weiterhin bei jedem Token-Ablauf.
func TestMigrateLegacyApp(t *testing.T) {
	cfg := &config.TwitchConfig{
		ClientID:     "recddyemjfl0xbklhcnukacerbz11n",
		ClientSecret: "altes-secret",
		AccessToken:  "a",
		RefreshToken: "r",
		ExpiresAt:    time.Now().Add(time.Hour),
	}

	if !MigrateLegacyApp(cfg) {
		t.Fatal("alte App wurde nicht erkannt")
	}
	if cfg.ClientID != DefaultClientID {
		t.Errorf("Client-ID ist %q, erwartet %q", cfg.ClientID, DefaultClientID)
	}
	// Tokens gehoeren zur alten App und sind fuer die neue wertlos.
	if cfg.AccessToken != "" || cfg.RefreshToken != "" || cfg.ClientSecret != "" {
		t.Error("Zugangsdaten der alten App wurden nicht entfernt")
	}
	if !cfg.ExpiresAt.IsZero() {
		t.Error("Ablaufzeitpunkt wurde nicht zurückgesetzt")
	}
}

// Eine eigene App des Nutzers darf die Umstellung nicht anfassen.
func TestMigrateLegacyAppLeavesCustomApp(t *testing.T) {
	cfg := &config.TwitchConfig{ClientID: "meine-eigene-app", RefreshToken: "r"}
	if MigrateLegacyApp(cfg) {
		t.Fatal("eigene App wurde fälschlich umgestellt")
	}
	if cfg.ClientID != "meine-eigene-app" || cfg.RefreshToken != "r" {
		t.Error("eigene App wurde verändert")
	}
}

// Die aktuelle Standard-App darf niemals als abgelöst gelten -- das würde bei
// jedem Start die Anmeldung löschen.
func TestDefaultClientIDIsNotLegacy(t *testing.T) {
	cfg := &config.TwitchConfig{ClientID: DefaultClientID, RefreshToken: "r"}
	if MigrateLegacyApp(cfg) {
		t.Fatal("die Standard-App gilt als abgelöst — Endlos-Abmeldung bei jedem Start")
	}
}

// Ein gleichnamiger Reward einer anderen App muss als solcher erkannt werden.
// Twitch antwortet darauf mit DUPLICATE_REWARD; ohne die Unterscheidung bleibt
// die Aktion still unverknüpft und jede Einlösung läuft ins Leere.
func TestCreateRewardDetectsDuplicate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error":"Bad Request","status":400,"message":"CREATE_CUSTOM_REWARD_DUPLICATE_REWARD"}`))
	}))
	defer srv.Close()
	old := twitchAPIURL
	twitchAPIURL = srv.URL
	defer func() { twitchAPIURL = old }()

	c := testClient(&config.TwitchConfig{AccessToken: "a", RefreshToken: "r",
		ExpiresAt: time.Now().Add(time.Hour), BroadcasterID: "1"})

	_, err := c.CreateReward("Helm ab!", 500, 60000, "")
	if !errors.Is(err, ErrRewardExists) {
		t.Fatalf("erwartet ErrRewardExists, bekam %v", err)
	}
	if !strings.Contains(err.Error(), "Helm ab!") {
		t.Errorf("Fehlermeldung nennt den Reward nicht: %s", err)
	}
}

// Eine gespeicherte Reward-ID kann jederzeit ins Leere zeigen, etwa weil im
// Twitch-Dashboard aufgeräumt wurde. Twitch antwortet dann mit 404. Ohne die
// Unterscheidung bliebe die Aktion dauerhaft ohne Reward und wäre nie auslösbar.
func TestUpdateRewardDetectsDeletedReward(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"error":"Not Found","status":404,` +
			`"message":"The custom reward specified in the id query parameter was not found."}`))
	}))
	defer srv.Close()
	old := twitchAPIURL
	twitchAPIURL = srv.URL
	defer func() { twitchAPIURL = old }()

	c := testClient(&config.TwitchConfig{AccessToken: "a", RefreshToken: "r",
		ExpiresAt: time.Now().Add(time.Hour), BroadcasterID: "1"})

	err := c.UpdateRewardEnabled("tote-id", true, "")
	if !errors.Is(err, ErrRewardNotFound) {
		t.Fatalf("erwartet ErrRewardNotFound, bekam %v", err)
	}
}

// Andere Fehler dürfen nicht als gelöschter Reward durchgehen -- sonst würde bei
// einem vorübergehenden Serverfehler ein zweiter Reward angelegt.
func TestUpdateRewardKeepsOtherErrorsDistinct(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()
	old := twitchAPIURL
	twitchAPIURL = srv.URL
	defer func() { twitchAPIURL = old }()

	c := testClient(&config.TwitchConfig{AccessToken: "a", RefreshToken: "r",
		ExpiresAt: time.Now().Add(time.Hour), BroadcasterID: "1"})

	err := c.UpdateRewardEnabled("id", true, "")
	if err == nil {
		t.Fatal("HTTP 500 muss ein Fehler sein")
	}
	if errors.Is(err, ErrRewardNotFound) {
		t.Error("HTTP 500 darf nicht als gelöschter Reward gelten")
	}
}

func TestHasLoginIgnoresAccessToken(t *testing.T) {
	// Access Token abgelaufen und geleert, Refresh Token vorhanden:
	// das ist eine gültige gespeicherte Anmeldung.
	if !(config.TwitchConfig{RefreshToken: "r"}).HasLogin() {
		t.Error("Anmeldung mit Refresh Token muss als vorhanden gelten")
	}
	if (config.TwitchConfig{AccessToken: "a"}).HasLogin() {
		t.Error("ohne Refresh Token lässt sich nichts wiederherstellen")
	}
}
