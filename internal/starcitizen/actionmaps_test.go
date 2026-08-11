package starcitizen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// realActionMaps liefert eine Kopie einer echten actionmaps.xml im Temp-Ordner.
// Ohne installiertes Star Citizen wird der Test uebersprungen.
func realActionMaps(t *testing.T) string {
	t.Helper()

	installs := FindInstalls()
	var src string
	for _, i := range installs {
		if _, err := os.Stat(i.ActionMapsPath()); err == nil {
			src = i.ActionMapsPath()
			break
		}
	}
	if src == "" {
		t.Skip("keine Star-Citizen-Installation mit actionmaps.xml gefunden")
	}

	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("lesen: %v", err)
	}
	dst := filepath.Join(t.TempDir(), "actionmaps.xml")
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatalf("kopieren: %v", err)
	}
	t.Logf("Testkopie von %s (%d bytes)", src, len(data))
	return dst
}

// Der Round-Trip darf nichts verlieren -- Joystick-Belegungen, Deadzones und
// Geraeteoptionen muessen die Speicherung ueberleben.
func TestRoundTripPreservesEverything(t *testing.T) {
	path := realActionMaps(t)

	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	am, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := am.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	for _, tag := range []string{"<action ", "<rebind ", "<actionmap ", "<option ", "<options ", "<deviceoptions "} {
		b, a := strings.Count(string(before), tag), strings.Count(string(after), tag)
		if b != a {
			t.Errorf("%q: vorher %d, nachher %d", tag, b, a)
		}
	}

	// Stichprobe: eine Joystick-Belegung muss unveraendert dastehen.
	if strings.Contains(string(before), `js1_button2`) && !strings.Contains(string(after), `js1_button2`) {
		t.Error("Joystick-Belegung js1_button2 ist beim Speichern verlorengegangen")
	}

	// Die Datei muss danach wieder ladbar sein.
	if _, err := Load(path); err != nil {
		t.Fatalf("neu laden nach Save: %v", err)
	}
}

func TestSetAndReadKeyboardBind(t *testing.T) {
	path := realActionMaps(t)

	am, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	// Neue Aktion in einer bestehenden actionmap.
	am.SetKeyboardBind("spaceship_general", "v_toggle_all_doors", "ralt+1")
	// Aktion in einer actionmap, die es evtl. noch nicht gibt.
	am.SetKeyboardBind("lights_controller", "v_lights", "ralt+2")

	if err := am.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load nach Save: %v", err)
	}
	if got := reloaded.KeyboardBind("spaceship_general", "v_toggle_all_doors"); got != "ralt+1" {
		t.Errorf("v_toggle_all_doors = %q, erwartet %q", got, "ralt+1")
	}
	if got := reloaded.KeyboardBind("lights_controller", "v_lights"); got != "ralt+2" {
		t.Errorf("v_lights = %q, erwartet %q", got, "ralt+2")
	}

	// Ein zweiter Aufruf darf keinen doppelten kb1_-Eintrag erzeugen, sondern
	// muss den bestehenden ersetzen.
	reloaded.SetKeyboardBind("spaceship_general", "v_toggle_all_doors", "ralt+3")
	if err := reloaded.Save(); err != nil {
		t.Fatal(err)
	}
	final, _ := os.ReadFile(path)
	if n := strings.Count(string(final), `kb1_ralt+1`); n != 0 {
		t.Errorf("alte Belegung ralt+1 steht noch %dx drin", n)
	}
	if n := strings.Count(string(final), `kb1_ralt+3`); n != 1 {
		t.Errorf("erwartet genau 1x kb1_ralt+3, gefunden %d", n)
	}
}

// Eine Belegung entfernen muss die Aktion auf Star Citizens Standardbelegung
// zurueckfallen lassen -- und Joystick-Belegungen dabei stehen lassen.
func TestRemoveKeyboardBindKeepsOtherDevices(t *testing.T) {
	path := realActionMaps(t)

	am, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	// Eine Aktion suchen, die sowohl Joystick- als auch Tastaturbelegung hat.
	am.SetKeyboardBind("spaceship_movement", "v_afterburner", "ralt+9")
	if err := am.Save(); err != nil {
		t.Fatal(err)
	}

	am2, _ := Load(path)
	if got := am2.KeyboardBind("spaceship_movement", "v_afterburner"); got != "ralt+9" {
		t.Fatalf("Vorbedingung: erwartet ralt+9, bekam %q", got)
	}
	if !am2.RemoveKeyboardBind("spaceship_movement", "v_afterburner") {
		t.Fatal("RemoveKeyboardBind meldet, dass nichts entfernt wurde")
	}
	if err := am2.Save(); err != nil {
		t.Fatal(err)
	}

	am3, _ := Load(path)
	if got := am3.KeyboardBind("spaceship_movement", "v_afterburner"); got != "" {
		t.Errorf("Belegung nicht entfernt, steht noch auf %q", got)
	}

	// Die Joystick-Belegungen des Profils muessen unversehrt sein.
	after, _ := os.ReadFile(path)
	if !strings.Contains(string(after), "js2_button4") {
		t.Error("Joystick-Belegung js2_button4 ist beim Entfernen verlorengegangen")
	}
}

// AllKeyboardBinds muss genau die Tastaturbelegungen liefern und Joystick-,
// Gamepad- und Maus-Eintraege ignorieren.
func TestAllKeyboardBinds(t *testing.T) {
	path := realActionMaps(t)
	am, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	binds := am.AllKeyboardBinds()
	if len(binds) == 0 {
		t.Skip("Profil enthält keine Tastaturbelegungen")
	}
	for key, val := range binds {
		if !strings.Contains(key, "|") {
			t.Errorf("Schlüssel ohne actionmap: %q", key)
		}
		if val == "" || strings.HasPrefix(val, "js") || strings.HasPrefix(val, "kb1_") {
			t.Errorf("%s: unsauberer Wert %q", key, val)
		}
	}
	t.Logf("%d Tastaturbelegungen gelesen", len(binds))
}

func TestParseKeyboardInput(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"kb1_lbracket", "lbracket", true},
		{"kb1_lshift+np_9", "lshift+np_9", true},
		{"kb1_ ", "", false},  // bewusst entfernte Belegung
		{"js1_button2", "", false},
		{"js1_rctrl+button5", "", false},
		{"mo1_mouse1", "", false},
	}
	for _, c := range cases {
		got, ok := parseKeyboardInput(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("parseKeyboardInput(%q) = %q,%v -- erwartet %q,%v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestFindInstalls(t *testing.T) {
	for _, i := range FindInstalls() {
		t.Logf("%s -> %s (%s)", i.Channel, i.Dir, i.Source)
		if !filepath.IsAbs(i.Dir) {
			t.Errorf("kein absoluter Pfad: %s", i.Dir)
		}
	}
}
