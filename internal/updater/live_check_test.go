package updater

import (
	"os"
	"testing"
)

// Einmaliger Durchstich gegen das echte Release: finden, laden, Pruefsumme und
// Signatur pruefen. Laeuft nur mit SCTROLL_LIVE_UPDATE_TEST=1, weil es Netz
// braucht und ein paar Megabyte zieht.
func TestLiveDownloadAndVerify(t *testing.T) {
	if os.Getenv("SCTROLL_LIVE_UPDATE_TEST") != "1" {
		t.Skip("nur mit SCTROLL_LIVE_UPDATE_TEST=1")
	}

	rel, err := Check()
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	t.Logf("gefunden: %s (%s), %d Bytes", rel.Version, rel.Tag, rel.Size())

	if rel.assetURL == "" {
		t.Fatal("Release enthält keine .exe")
	}
	if rel.hashURL == "" {
		t.Error("Release enthält keine .sha256 — dann fällt ein kaputter Download nicht auf")
	}

	var last float64
	path, err := Download(rel, func(p float64) { last = p })
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	defer os.Remove(path)

	if last < 1 {
		t.Errorf("Fortschritt endete bei %.2f statt 1.0", last)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Size() != rel.Size() {
		t.Errorf("Größe %d, erwartet %d", st.Size(), rel.Size())
	}
	t.Logf("geladen, Prüfsumme und Signatur in Ordnung: %s", path)
}
