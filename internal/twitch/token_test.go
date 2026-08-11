package twitch

import (
	"errors"
	"net/http"
	"net/http/httptest"
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
