// Package autostart traegt SCTroll in den Windows-Autostart ein.
package autostart

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"

	"sctroll/internal/debuglog"
)

// Eintrag unter HKEY_CURRENT_USER, nicht HKEY_LOCAL_MACHINE: das gilt nur fuer
// den angemeldeten Nutzer und braucht keine Administratorrechte.
//
// Bewusst die Registry statt einer Verknuepfung im Autostart-Ordner: der
// Eintrag enthaelt den Pfad direkt, es entsteht keine zweite Datei, die nach
// einem Verschieben ins Leere zeigt.
const (
	runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`
	valueName  = "SCTroll"
)

// command ist der Befehl, der beim Anmelden ausgefuehrt wird.
// In Anfuehrungszeichen, weil der Pfad Leerzeichen enthalten kann.
func command() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return `"` + exe + `"`, nil
}

// Enabled meldet, ob SCTroll beim Anmelden startet.
//
// Zeigt der Eintrag auf eine andere Programmdatei -- etwa nach einem Verschieben
// des Ordners -- gilt der Autostart als aus. Sonst meldete die Oberflaeche "an",
// waehrend beim Anmelden nichts oder das Falsche startet.
func Enabled() bool {
	current, err := currentValue()
	if err != nil {
		return false
	}
	want, err := command()
	if err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(current), strings.TrimSpace(want))
}

func currentValue() (string, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return "", err
	}
	defer k.Close()

	value, _, err := k.GetStringValue(valueName)
	return value, err
}

// Set schaltet den Autostart ein oder aus.
func Set(enabled bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("Autostart-Schlüssel nicht erreichbar: %w", err)
	}
	defer k.Close()

	if !enabled {
		// Nicht vorhanden ist kein Fehler -- das Ziel ist ja erreicht.
		if err := k.DeleteValue(valueName); err != nil && !errors.Is(err, registry.ErrNotExist) {
			return fmt.Errorf("Autostart konnte nicht entfernt werden: %w", err)
		}
		debuglog.Log("autostart: entfernt")
		return nil
	}

	cmd, err := command()
	if err != nil {
		return err
	}
	if err := k.SetStringValue(valueName, cmd); err != nil {
		return fmt.Errorf("Autostart konnte nicht gesetzt werden: %w", err)
	}
	debuglog.Log("autostart: eingetragen -> %s", cmd)
	return nil
}

// Refresh zieht einen bestehenden Eintrag auf die aktuelle Programmdatei nach.
//
// Noetig nach einem Selbstupdate oder wenn der Ordner verschoben wurde: der
// alte Pfad zeigte sonst ins Leere und der Autostart liefe stillschweigend
// nicht mehr.
func Refresh() {
	current, err := currentValue()
	if err != nil {
		return // Autostart ist aus, nichts nachzuziehen
	}
	want, err := command()
	if err != nil {
		return
	}
	if strings.EqualFold(strings.TrimSpace(current), strings.TrimSpace(want)) {
		return
	}

	debuglog.Log("autostart: Pfad veraltet (%s) — wird auf %s gesetzt", current, want)
	if err := Set(true); err != nil {
		debuglog.Log("autostart: nachziehen fehlgeschlagen: %s", err)
	}
}
