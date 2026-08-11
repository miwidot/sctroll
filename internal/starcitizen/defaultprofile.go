package starcitizen

import (
	_ "embed"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"sctroll/internal/debuglog"
)

// defaultProfileXML ist Star Citizens Standardbelegung.
//
// Die Datei steckt im Spiel in der Data.p4k und ist dort nicht ohne weiteres
// lesbar. Diese Kopie stammt aus dem Stream-Deck-Plugin von ltmajor42
// (github.com/ltmajor42/StarCitizen_Streamdeck_Plugin), das sie aus der p4k
// entpackt. Sie kann also hinter dem aktuellen Patch zurueckliegen -- eine
// eigene Datei unter %APPDATA%\SCTroll\defaultProfile.xml hat Vorrang.
//
//go:embed data/defaultProfile.xml
var defaultProfileXML []byte

// DefaultBind ist die Standardbelegung einer Aktion.
type DefaultBind struct {
	Key            string `json:"key"`             // "" = im Spiel unbelegt
	ActivationMode string `json:"activation_mode"` // press, tap, hold, delayed_press_medium, ...
	HoldMs         int    `json:"hold_ms"`         // daraus abgeleitete Haltezeit
}

// profileDoc bildet nur ab, was gebraucht wird -- die Datei enthaelt daneben
// noch UI-Texte, Zustaende und Gamepad-/Joystick-Belegungen.
type profileDoc struct {
	ActionMaps []struct {
		Name    string `xml:"name,attr"`
		Actions []struct {
			Name           string `xml:"name,attr"`
			Keyboard       string `xml:"keyboard,attr"`
			ActivationMode string `xml:"activationMode,attr"`
			OnHold         string `xml:"onHold,attr"`
		} `xml:"action"`
	} `xml:"actionmap"`
}

var (
	defaultBindsOnce sync.Once
	defaultBinds     map[string]DefaultBind
)

// DefaultBinds liefert die Standardbelegungen als "actionmap|action" -> Bind.
func DefaultBinds() map[string]DefaultBind {
	defaultBindsOnce.Do(func() {
		data := userDefaultProfile()
		if data == nil {
			data = defaultProfileXML
		}
		defaultBinds = parseDefaultProfile(data)
		debuglog.Log("defaultProfile: %d Standardbelegungen geladen", len(defaultBinds))
	})
	return defaultBinds
}

// userDefaultProfile laedt eine selbst entpackte Datei, falls vorhanden.
// Damit laesst sich nach einem Patch aktualisieren, ohne SCTroll neu zu bauen.
func userDefaultProfile() []byte {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil
	}
	path := filepath.Join(dir, "SCTroll", "defaultProfile.xml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	debuglog.Log("defaultProfile: eigene Datei benutzt (%s)", path)
	return data
}

func parseDefaultProfile(data []byte) map[string]DefaultBind {
	var doc profileDoc
	if err := xml.Unmarshal(data, &doc); err != nil {
		debuglog.Log("defaultProfile: nicht lesbar: %s", err)
		return map[string]DefaultBind{}
	}

	out := make(map[string]DefaultBind, 1200)
	for _, am := range doc.ActionMaps {
		for _, a := range am.Actions {
			// Ein Leerzeichen als keyboard heisst "im Spiel nicht belegt".
			key := strings.TrimSpace(a.Keyboard)
			mode := a.ActivationMode
			if mode == "" && a.OnHold == "1" {
				mode = "hold"
			}
			out[am.Name+"|"+a.Name] = DefaultBind{
				Key:            strings.ToLower(key),
				ActivationMode: mode,
				HoldMs:         holdForActivationMode(mode),
			}
		}
	}
	return out
}

// holdForActivationMode leitet aus Star Citizens Aktivierungsart ab, wie lange
// die Taste gehalten werden muss.
//
// Die Zahlen sind Erfahrungswerte, keine aus dem Spiel ausgelesenen Schwellen --
// im Zweifel bei der Aktion nachjustieren. Zu kurz nimmt Star Citizen ohnehin
// nicht an, deshalb ist die Untergrenze grosszuegig.
func holdForActivationMode(mode string) int {
	switch mode {
	case "hold", "hold_toggle", "hold_no_retrigger":
		return 800
	case "delayed_press":
		return 500
	case "delayed_press_medium", "delayed_hold", "delayed_hold_no_retrigger":
		return 900
	case "delayed_hold_long":
		return 1600
	default: // press, tap, double_tap, smart_toggle, all, ...
		return 80
	}
}
