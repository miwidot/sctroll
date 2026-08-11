package starcitizen

import "testing"

// Stichproben gegen die ausgelieferte defaultProfile.xml. Wenn eine neuere
// Datei eingespielt wird und Star Citizen eine Belegung geaendert hat, faellt
// das hier auf, statt still im Stream.
func TestDefaultBindsKnownActions(t *testing.T) {
	binds := DefaultBinds()
	if len(binds) < 500 {
		t.Fatalf("nur %d Standardbelegungen geladen — defaultProfile.xml kaputt?", len(binds))
	}

	cases := []struct {
		key  string
		want string
	}{
		{"lights_controller|v_lights", "l"},
		{"spaceship_movement|v_toggle_landing_system", "n"},
		{"spaceship_general|v_flightready", "ralt+r"},
		{"player|toggleAttachHelmet", "lalt+h"},
		{"seat_general|v_eject", "ralt+y"},
		{"spaceship_general|v_self_destruct", "backspace"},
		{"spaceship_movement|v_ifcs_vector_decoupling_toggle", "c"},

		// Ohne Standardbelegung -- deshalb muss man diese selbst binden.
		{"spaceship_general|v_toggle_all_doors", ""},
		{"player_emotes|emote_dance", ""},
	}
	for _, c := range cases {
		b, ok := binds[c.key]
		if !ok {
			t.Errorf("%s fehlt in defaultProfile.xml", c.key)
			continue
		}
		if b.Key != c.want {
			t.Errorf("%s: Taste %q, erwartet %q", c.key, b.Key, c.want)
		}
	}
}

// Aus der Aktivierungsart muss eine brauchbare Haltezeit fallen -- ein
// "delayed_press_medium" mit 80ms wuerde im Spiel nichts ausloesen.
func TestHoldForActivationMode(t *testing.T) {
	cases := map[string]int{
		"press":                     80,
		"tap":                       80,
		"hold":                      800,
		"delayed_press":             500,
		"delayed_press_medium":      900,
		"delayed_hold_no_retrigger": 900,
		"delayed_hold_long":         1600,
		"":                          80,
	}
	for mode, want := range cases {
		if got := holdForActivationMode(mode); got != want {
			t.Errorf("%q: %dms, erwartet %dms", mode, got, want)
		}
	}

	// pl_exit hat keinen activationMode, sondern onHold="1" -- das muss als
	// Halteaktion ankommen, sonst steht man nie auf.
	if b := DefaultBinds()["default|pl_exit"]; b.HoldMs < 500 {
		t.Errorf("pl_exit: %dms zu kurz für eine Halteaktion (Modus %q)", b.HoldMs, b.ActivationMode)
	}
}
