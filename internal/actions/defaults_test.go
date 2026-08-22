package actions

import (
	"strings"
	"testing"

	"sctroll/internal/config"
	"sctroll/internal/input"
	"sctroll/internal/starcitizen"
)

// Jede mitgelieferte Aktion muss eine Taste haben, die sich tatsaechlich
// druecken laesst. Ein Tippfehler in defaultActions wuerde sonst erst live
// im Stream auffallen -- als Aktion, die einfach nichts tut.
func TestDefaultActionKeysResolve(t *testing.T) {
	for _, a := range config.DefaultActions() {
		// Leer ist zulaessig: die Aktion ist im Spiel ab Werk unbelegt und muss
		// dort erst gebunden werden.
		if a.Key == "" {
			continue
		}
		// Sonderaktionen bewegen nur die Maus und haben keine Taste.
		if strings.HasPrefix(a.Key, "spin360") {
			continue
		}
		for _, part := range strings.Split(strings.ToLower(a.Key), "+") {
			if _, ok := input.ResolveKey(part); !ok && !input.IsMouseButton(part) {
				t.Errorf("%s: Taste %q enthält unbekanntes Element %q", a.ID, a.Key, part)
			}
		}
		for i, s := range a.Steps {
			if s.Key == "" {
				continue
			}
			for _, part := range strings.Split(strings.ToLower(s.Key), "+") {
				if _, ok := input.ResolveKey(part); !ok && !input.IsMouseButton(part) {
					t.Errorf("%s Schritt %d: Taste %q unbekannt", a.ID, i, part)
				}
			}
		}
	}
}

// Zwei Aktionen auf derselben Taste hiessen: eine Einloesung loest beide aus.
func TestDefaultActionKeysAreUnique(t *testing.T) {
	seen := map[string]string{}
	for _, a := range config.DefaultActions() {
		key := strings.ToLower(a.Key)
		if key == "" {
			continue // unbelegte Aktionen kollidieren nicht miteinander
		}
		if other, dup := seen[key]; dup {
			t.Errorf("Taste %q doppelt vergeben: %s und %s", a.Key, other, a.ID)
		}
		seen[key] = a.ID
	}
}

// Alles, was im Spiel gebunden werden muss, braucht auch eine actionmap --
// ohne die weiss sctroll nicht, wohin der Bind geschrieben werden soll.
func TestDefaultActionsHaveActionMaps(t *testing.T) {
	for _, a := range config.DefaultActions() {
		if a.SCAction != "" && a.SCActionMap == "" {
			t.Errorf("%s: SCAction %q ohne SCActionMap", a.ID, a.SCAction)
		}
		if a.SCActionMap != "" && a.SCAction == "" {
			t.Errorf("%s: SCActionMap %q ohne SCAction", a.ID, a.SCActionMap)
		}
	}
}

// Jede mitgelieferte Aktion muss es in Star Citizen wirklich geben. Genau daran
// ist v1.0.2 gescheitert: v_toggle_quantum_mode war seit den Master Modes
// unbelegte Altlast, v_power_set_off ebenso -- gemerkt hat man es erst im Spiel,
// weil nichts passierte.
func TestDefaultActionsExistInGame(t *testing.T) {
	binds := starcitizen.DefaultBinds()

	for _, a := range config.DefaultActions() {
		if a.SCAction == "" {
			continue // reine Mausaktionen wie der 360-Spin
		}
		id := a.SCActionMap + "|" + a.SCAction
		def, ok := binds[id]
		if !ok {
			t.Errorf("%s: %q gibt es in der defaultProfile.xml nicht — Aktion umbenannt oder entfernt?", a.ID, id)
			continue
		}

		// Die mitgelieferte Taste muss die Standardbelegung sein. Ist die Aktion
		// ab Werk unbelegt, muss auch bei uns nichts stehen -- sonst druecken wir
		// eine Taste, die im Spiel nichts tut.
		if !strings.EqualFold(a.Key, def.Key) {
			t.Errorf("%s: Taste %q, Standardbelegung ist %q (%s)", a.ID, a.Key, def.Key, def.ActivationMode)
		}
	}
}

// Halteaktionen brauchen genug Haltezeit, sonst nimmt das Spiel sie nicht an.
func TestDefaultActionsHoldLongEnough(t *testing.T) {
	binds := starcitizen.DefaultBinds()

	for _, a := range config.DefaultActions() {
		if a.SCAction == "" || a.Key == "" {
			continue
		}
		def := binds[a.SCActionMap+"|"+a.SCAction]
		if a.HoldMs < def.HoldMs {
			t.Errorf("%s: %dms zu kurz für %q (%s braucht %dms)",
				a.ID, a.HoldMs, a.SCAction, def.ActivationMode, def.HoldMs)
		}
	}
}

// Gefaehrliche Aktionen duerfen nicht versehentlich aktiv ausgeliefert werden.
func TestDangerousActionsDisabledByDefault(t *testing.T) {
	for _, a := range config.DefaultActions() {
		if a.Category == "danger" && a.Enabled {
			t.Errorf("%s ist als 'danger' eingestuft, aber standardmäßig aktiv", a.ID)
		}
	}
}

// Mehrstufige Aktionen muessen in sich stimmig sein: der Wurf beim Granaten-
// Ablauf braucht die Maustaste, nicht die Auswahltaste.
func TestGrenadeSequence(t *testing.T) {
	var grenade *config.Action
	for _, a := range config.DefaultActions() {
		if a.ID == "grenade" {
			a := a
			grenade = &a
		}
	}
	if grenade == nil {
		t.Skip("keine Granaten-Aktion vorhanden")
	}

	if len(grenade.Steps) < 3 {
		t.Fatalf("erwartet Auswahl, Pause und Wurf, gefunden %d Schritte", len(grenade.Steps))
	}
	if !strings.EqualFold(grenade.Steps[0].Key, grenade.Key) {
		t.Errorf("erster Schritt drückt %q, die Aktion ist aber auf %q gebunden",
			grenade.Steps[0].Key, grenade.Key)
	}

	last := grenade.Steps[len(grenade.Steps)-1]
	if !input.IsMouseButton(last.Key) {
		t.Errorf("letzter Schritt ist %q — der Wurf läuft im Spiel über die Maustaste", last.Key)
	}
	if last.HoldMs < 300 {
		t.Errorf("Wurf wird nur %dms gehalten; throw_overhand ist eine Halteaktion", last.HoldMs)
	}
}

// Die Selbstzerstoerung braucht einen langen Halt. Mit einem kurzen Druck
// passiert im Spiel nichts -- das faellt sonst erst live auf.
func TestSelfDestructIsHeldLongEnough(t *testing.T) {
	for _, a := range config.DefaultActions() {
		if a.ID != "self_destruct" {
			continue
		}
		if a.HoldMs < 10000 {
			t.Errorf("Selbstzerstörung hält nur %dms — das Spiel verlangt rund 15 Sekunden", a.HoldMs)
		}
		return
	}
	t.Skip("keine Selbstzerstörung vorhanden")
}
