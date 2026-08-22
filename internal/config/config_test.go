package config

import (
	"strings"
	"testing"
)

// Genau hieran ist v1.0.4 gescheitert: die Umstellung auf Star Citizens echte
// Standardbelegungen hat Tasten von Aktionen geleert, die im Spiel ab Werk
// unbelegt sind. Damit waren zuvor aus dem Spielprofil uebernommene Emote-Tasten
// weg -- und die Emotes taten nichts mehr.
func TestAdoptShippedKeysNeverBlanks(t *testing.T) {
	c := &Config{Actions: []Action{
		// Aus dem Spielprofil übernommen; im Spiel ab Werk unbelegt.
		{ID: "emote_dance", Key: "np_5", HoldMs: 80},
		{ID: "doors", Key: "lbracket", HoldMs: 80},
		// Alte erfundene Taste; muss auf die Standardbelegung gehen.
		{ID: "lights", Key: "ralt+3", HoldMs: 80},
	}}

	c.adoptShippedKeys()

	byID := map[string]Action{}
	for _, a := range c.Actions {
		byID[a.ID] = a
	}

	if got := byID["emote_dance"].Key; got != "np_5" {
		t.Errorf("emote_dance: Taste ist %q, übernommene Belegung muss erhalten bleiben", got)
	}
	if got := byID["doors"].Key; got != "lbracket" {
		t.Errorf("doors: Taste ist %q, übernommene Belegung muss erhalten bleiben", got)
	}
	if got := byID["lights"].Key; got != "l" {
		t.Errorf("lights: Taste ist %q, erwartet Standardbelegung %q", got, "l")
	}
}

// Umbenannte Aktionen muessen samt Reward umziehen, sonst verwaist der Reward
// auf Twitch und der Zuschauer loest ins Nichts aus.
func TestMigrateLegacyActionsCarriesReward(t *testing.T) {
	c := &Config{Actions: []Action{
		{ID: "quantum_mode", RewardID: "abc-123", RewardCost: 777, Enabled: true, Cooldown: 4242},
		{ID: "nightvision", RewardID: "def-456", Enabled: true},
	}}

	c.migrateLegacyActions()

	var master *Action
	for i := range c.Actions {
		if c.Actions[i].ID == "master_mode" {
			master = &c.Actions[i]
		}
		if c.Actions[i].ID == "quantum_mode" {
			t.Error("quantum_mode existiert noch — Altlast nicht migriert")
		}
		if c.Actions[i].ID == "nightvision" {
			t.Error("nightvision existiert noch — ersatzlos gestrichene Aktion nicht entfernt")
		}
	}

	if master == nil {
		t.Fatal("master_mode fehlt — Ersatzaktion nicht angelegt")
	}
	if master.RewardID != "abc-123" {
		t.Errorf("Reward-ID ist %q, erwartet %q", master.RewardID, "abc-123")
	}
	if master.RewardCost != 777 || master.Cooldown != 4242 || !master.Enabled {
		t.Errorf("Nutzereinstellungen verloren: cost=%d cooldown=%d enabled=%v",
			master.RewardCost, master.Cooldown, master.Enabled)
	}
	if master.SCAction != "v_master_mode_cycle_long" {
		t.Errorf("SCAction ist %q, erwartet v_master_mode_cycle_long", master.SCAction)
	}
}

// Die Selbstzerstoerung muss im Spiel rund fuenfzehn Sekunden gehalten werden.
// Bestehende Konfigurationen tragen noch die kurze Zeit aus dem Spielprofil und
// wuerden sonst dauerhaft nichts ausloesen.
func TestRaiseTooShortHolds(t *testing.T) {
	c := &Config{Actions: []Action{
		{ID: "self_destruct", Key: "backspace", HoldMs: 900},
		{ID: "lights", Key: "l", HoldMs: 80},
		// Bewusst länger eingestellt: darf nicht gesenkt werden.
		{ID: "look_behind", Key: "comma", HoldMs: 5000},
		{ID: "custom_1", Custom: true, HoldMs: 10},
	}}

	c.raiseTooShortHolds()

	byID := map[string]Action{}
	for _, a := range c.Actions {
		byID[a.ID] = a
	}
	if got := byID["self_destruct"].HoldMs; got < 10000 {
		t.Errorf("self_destruct hält %dms, erwartet mindestens 10000", got)
	}
	if got := byID["look_behind"].HoldMs; got != 5000 {
		t.Errorf("look_behind wurde von 5000 auf %d geändert — nur anheben, nie senken", got)
	}
	if got := byID["custom_1"].HoldMs; got != 10 {
		t.Errorf("eigene Aktion wurde angefasst: %dms", got)
	}
}

// Alte Konfigurationen tragen Einrichtungshinweise in der Beschreibung. Seit die
// als Text der Belohnung an Twitch geht, bekäme jeder Zuschauer sie zu sehen.
func TestSanitizeDescriptions(t *testing.T) {
	c := &Config{Actions: []Action{
		{ID: "doors", Description: "Schaltet alle Schiffstüren um (im Spiel ab Werk unbelegt)"},
		{ID: "lights", Description: "Mein eigener Text, bitte stehen lassen"},
		{ID: "custom_1", Custom: true, Description: "Eigene Aktion (im Spiel ab Werk unbelegt)"},
	}}

	c.sanitizeDescriptions()

	byID := map[string]Action{}
	for _, a := range c.Actions {
		byID[a.ID] = a
	}
	if strings.Contains(byID["doors"].Description, "unbelegt") {
		t.Errorf("Einrichtungshinweis steht noch drin: %q", byID["doors"].Description)
	}
	if byID["lights"].Description != "Mein eigener Text, bitte stehen lassen" {
		t.Errorf("eigener Text wurde überschrieben: %q", byID["lights"].Description)
	}
	if byID["custom_1"].Description != "Eigene Aktion (im Spiel ab Werk unbelegt)" {
		t.Errorf("eigene Aktion wurde angefasst: %q", byID["custom_1"].Description)
	}
}
