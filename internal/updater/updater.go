// Package updater sucht auf GitHub nach neuen Versionen, laedt sie herunter und
// tauscht die laufende Programmdatei aus.
package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"sctroll/internal/debuglog"
	"sctroll/internal/version"
)

// Release ist eine auf GitHub veroeffentlichte Version.
type Release struct {
	Version     string `json:"version"`      // ohne fuehrendes "v"
	Tag         string `json:"tag"`          // wie auf GitHub, z.B. "v1.0.8"
	Notes       string `json:"notes"`        // Release-Beschreibung
	PublishedAt string `json:"published_at"` //
	URL         string `json:"url"`          // Seite im Browser
	Newer       bool   `json:"newer"`        // neuer als die laufende Version?

	assetURL string // Download der Exe
	hashURL  string // zugehoerige .sha256
	assetSz  int64
}

// Size ist die Groesse der Programmdatei in Bytes.
func (r Release) Size() int64 { return r.assetSz }

type ghRelease struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	HTMLURL     string    `json:"html_url"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
		Size int64  `json:"size"`
	} `json:"assets"`
}

var client = &http.Client{Timeout: 30 * time.Second}

// releaseAPI ist die Abfrage der neuesten Veroeffentlichung.
// Als Variable, damit Tests sie auf einen lokalen Stub zeigen lassen koennen.
var releaseAPI = "https://api.github.com/repos/" + version.Repo + "/releases/latest"

// Check fragt die neueste Veroeffentlichung ab.
//
// Ohne Anmeldung erlaubt GitHub 60 Anfragen pro Stunde und IP -- fuer eine
// Pruefung beim Start und gelegentlich von Hand reicht das bei weitem.
func Check() (*Release, error) {
	req, err := http.NewRequest("GET", releaseAPI, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "SCTroll/"+version.Current)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GitHub nicht erreichbar: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("noch keine Veröffentlichung vorhanden")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub antwortet mit HTTP %d", resp.StatusCode)
	}

	var gh ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&gh); err != nil {
		return nil, err
	}

	rel := &Release{
		Version:     strings.TrimPrefix(gh.TagName, "v"),
		Tag:         gh.TagName,
		Notes:       gh.Body,
		PublishedAt: gh.PublishedAt.Format("02.01.2006"),
		URL:         gh.HTMLURL,
	}
	rel.Newer = version.IsNewer(rel.Version)

	for _, a := range gh.Assets {
		switch {
		case strings.HasSuffix(a.Name, ".sha256"):
			rel.hashURL = a.URL
		case strings.HasSuffix(a.Name, ".exe"):
			rel.assetURL = a.URL
			rel.assetSz = a.Size
		}
	}

	debuglog.Log("updater: neueste Version %s (laufend %s, neuer=%v)",
		rel.Version, version.Current, rel.Newer)
	return rel, nil
}

// Download laedt die neue Programmdatei neben die laufende und prueft sie.
// Rueckgabe ist der Pfad der geprueften Datei.
//
// progress wird mit 0..1 aufgerufen, darf nil sein.
func Download(rel *Release, progress func(float64)) (string, error) {
	if rel.assetURL == "" {
		return "", fmt.Errorf("die Veröffentlichung enthält keine Programmdatei")
	}

	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exe, _ = filepath.EvalSymlinks(exe)
	target := exe + ".new"

	// Erst die Prüfsumme holen, damit ein abgebrochener Download auffällt.
	wantHash, err := fetchHash(rel.hashURL)
	if err != nil {
		return "", err
	}

	if err := download(rel.assetURL, target, rel.assetSz, progress); err != nil {
		os.Remove(target)
		return "", err
	}

	gotHash, err := fileHash(target)
	if err != nil {
		os.Remove(target)
		return "", err
	}
	if wantHash != "" && !strings.EqualFold(gotHash, wantHash) {
		os.Remove(target)
		return "", fmt.Errorf("Prüfsumme stimmt nicht — Download verworfen")
	}

	// Signatur prüfen. Das ist der eigentliche Schutz: die Prüfsumme kommt aus
	// derselben Quelle wie die Datei und belegt nur einen heilen Download.
	if err := verifySignature(target); err != nil {
		os.Remove(target)
		return "", fmt.Errorf("Signatur ungültig — Download verworfen: %w", err)
	}

	debuglog.Log("updater: %s geladen und geprüft (sha256 %s)", target, gotHash[:16])
	return target, nil
}

// UpdatedFlag bekommt der Nachfolger beim Start mit. Daran erkennt er, dass die
// Vorgaengerinstanz noch einen Moment laeuft, und wartet auf die Einzelinstanz-
// Sperre, statt sich mit "laeuft bereits" zu beenden.
const UpdatedFlag = "--updated"

// Apply tauscht die laufende Programmdatei aus und startet die neue Version.
//
// Windows laesst eine laufende Exe nicht ueberschreiben, wohl aber umbenennen.
// Also: laufende zur Seite schieben, neue an ihren Platz, neu starten. Die
// beiseitegeschobene Datei raeumt CleanupOld beim naechsten Start weg.
func Apply(newExe string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, _ = filepath.EvalSymlinks(exe)
	old := exe + ".old"

	os.Remove(old) // Rest eines frueheren Updates

	if err := os.Rename(exe, old); err != nil {
		return fmt.Errorf("Programmdatei lässt sich nicht ersetzen (%w) — "+
			"liegt SCTroll in einem geschützten Ordner wie Programme?", err)
	}
	if err := os.Rename(newExe, exe); err != nil {
		// Zurückrollen, sonst steht das Programm ohne Exe da.
		if rbErr := os.Rename(old, exe); rbErr != nil {
			return fmt.Errorf("Update fehlgeschlagen und Rücknahme misslungen: %w (%v)", err, rbErr)
		}
		return fmt.Errorf("Update fehlgeschlagen, alte Version wiederhergestellt: %w", err)
	}

	debuglog.Log("updater: ausgetauscht, starte %s neu", exe)
	cmd := exec.Command(exe, UpdatedFlag)
	cmd.Dir = filepath.Dir(exe)
	return cmd.Start()
}

// CleanupOld entfernt die beim letzten Update beiseitegeschobene Datei.
// Beim Start aufrufen -- vorher ist sie unter Umstaenden noch gesperrt.
func CleanupOld() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	exe, _ = filepath.EvalSymlinks(exe)
	for _, leftover := range []string{exe + ".old", exe + ".new"} {
		if err := os.Remove(leftover); err == nil {
			debuglog.Log("updater: %s aufgeräumt", filepath.Base(leftover))
		}
	}
}

func fetchHash(url string) (string, error) {
	if url == "" {
		return "", nil // ohne veroeffentlichte Pruefsumme bleibt die Signatur
	}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 512))
	if err != nil {
		return "", err
	}
	// Format: "<hash>  <dateiname>"
	return strings.ToLower(strings.Fields(string(body))[0]), nil
}

func download(url, target string, size int64, progress func(float64)) error {
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("Download fehlgeschlagen: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("Download fehlgeschlagen (HTTP %d)", resp.StatusCode)
	}

	f, err := os.Create(target)
	if err != nil {
		return err
	}
	defer f.Close()

	var written int64
	buf := make([]byte, 64*1024)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				return werr
			}
			written += int64(n)
			if progress != nil && size > 0 {
				progress(float64(written) / float64(size))
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return rerr
		}
	}
	if progress != nil {
		progress(1)
	}
	return nil
}

func fileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
