package version

import (
	"encoding/json"
	"os"
	"testing"
)

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.7", "1.0.7", 0},
		{"v1.0.7", "1.0.7", 0},
		{"1.0.8", "1.0.7", 1},
		{"1.0.7", "1.0.8", -1},
		{"1.1.0", "1.0.9", 1},
		{"2.0.0", "1.9.9", 1},
		{"1.0.10", "1.0.9", 1}, // rein alphabetisch waere das falsch herum
		{"1.1", "1.0.9", 1},    // fehlende Stellen zaehlen als 0
		{"1.0.8-beta1", "1.0.8", 0},
		{"kaputt", "1.0.0", -1},
	}
	for _, c := range cases {
		if got := Compare(c.a, c.b); got != c.want {
			t.Errorf("Compare(%q, %q) = %d, erwartet %d", c.a, c.b, got, c.want)
		}
	}
}

// Eine Version darf sich niemals selbst als Update anbieten -- sonst landet der
// Nutzer in einer Schleife aus Neustarts.
func TestIsNewerRejectsSameAndOlder(t *testing.T) {
	if IsNewer(Current) {
		t.Error("die laufende Version gilt als Update")
	}
	if IsNewer("v" + Current) {
		t.Error("dieselbe Version mit v-Präfix gilt als Update")
	}
	if IsNewer("0.0.1") {
		t.Error("ältere Version gilt als Update")
	}
	if !IsNewer("99.0.0") {
		t.Error("neuere Version wird nicht erkannt")
	}
}

// Die Version steht auch in wails.json, weil Wails daraus die Dateieigenschaften
// der Exe baut. Laufen die auseinander, bietet die App Updates auf sich selbst
// an oder meldet eine falsche Version.
func TestVersionMatchesWailsConfig(t *testing.T) {
	data, err := os.ReadFile("../../wails.json")
	if err != nil {
		t.Skip("wails.json nicht gefunden")
	}
	var cfg struct {
		Info struct {
			ProductVersion string `json:"productVersion"`
		} `json:"info"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Info.ProductVersion != Current {
		t.Errorf("wails.json sagt %q, version.Current sagt %q — beide anpassen",
			cfg.Info.ProductVersion, Current)
	}
}

func TestAssetName(t *testing.T) {
	if got := AssetName("v1.0.8"); got != "SCTroll-1.0.8-windows-amd64.exe" {
		t.Errorf("AssetName = %q", got)
	}
}
