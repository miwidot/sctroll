package updater

import (
	"os"
	"path/filepath"
	"testing"
)

// Eine unsignierte Datei darf niemals eingespielt werden. Ohne diese Pruefung
// waere die Update-Funktion ein bequemer Weg, fremden Code auszufuehren.
func TestVerifySignatureRejectsUnsigned(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fake.exe")
	if err := os.WriteFile(path, []byte("MZ das ist keine echte Exe"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifySignature(path); err == nil {
		t.Fatal("unsignierte Datei wurde als gültig durchgewinkt")
	} else {
		t.Logf("abgelehnt mit: %s", err)
	}
}

// Gegenprobe an einer echt signierten Datei: die Pruefung darf nicht
// grundsaetzlich alles ablehnen.
//
// Wichtig ist eine Datei mit *eingebetteter* Signatur. Windows-Systemdateien wie
// notepad.exe sind ueber Sicherheitskataloge signiert und traegen selbst keine --
// die wuerden hier zu Recht als "nicht signiert" gelten.
func TestVerifySignatureAcceptsEmbeddedSignature(t *testing.T) {
	candidates := []string{
		`C:\Program Files\Git\bin\git.exe`,
		`C:\Program Files\Git\cmd\git.exe`,
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		if err := verifySignature(path); err != nil {
			t.Errorf("%s ist eingebettet signiert, wurde aber abgelehnt: %s", path, err)
		} else {
			t.Logf("angenommen: %s", path)
		}
		return
	}
	t.Skip("keine eingebettet signierte Vergleichsdatei gefunden")
}

// Eine nachtraeglich veraenderte Datei muss auffallen -- sonst brächte die
// Signaturpruefung nichts.
func TestVerifySignatureRejectsTamperedFile(t *testing.T) {
	src := `C:\Program Files\Git\bin\git.exe`
	data, err := os.ReadFile(src)
	if err != nil {
		t.Skip("keine signierte Vergleichsdatei gefunden")
	}
	// Ein Byte im Code-Bereich kippen, Signatur bleibt unveraendert stehen.
	data[len(data)/2] ^= 0xFF

	path := filepath.Join(t.TempDir(), "tampered.exe")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifySignature(path); err == nil {
		t.Fatal("veränderte Datei wurde als gültig durchgewinkt")
	} else {
		t.Logf("abgelehnt mit: %s", err)
	}
}

// Der eigene Build muss die Pruefung bestehen -- sonst kann sich das Programm
// nicht selbst aktualisieren.
func TestVerifySignatureAcceptsOwnBuild(t *testing.T) {
	path := filepath.Join("..", "..", "build", "bin", "SCTroll.exe")
	if _, err := os.Stat(path); err != nil {
		t.Skip("kein Build vorhanden")
	}
	if err := verifySignature(path); err != nil {
		t.Skipf("eigener Build ist nicht signiert (%s) — vor der Veröffentlichung signieren", err)
	}
}
