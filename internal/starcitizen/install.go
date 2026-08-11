package starcitizen

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"unsafe"

	"sctroll/internal/debuglog"
)

// Install ist eine gefundene Star-Citizen-Installation (ein Channel).
type Install struct {
	Channel string `json:"channel"` // LIVE, PTU, EPTU, HOTFIX, TECH-PREVIEW
	Dir     string `json:"dir"`     // ...\Roberts Space Industries\StarCitizen\LIVE
	Source  string `json:"source"`  // wie gefunden (fuer die UI)
}

// ActionMapsPath ist die Datei mit den aktuellen Tastenbelegungen.
func (i Install) ActionMapsPath() string {
	return filepath.Join(i.Dir, "user", "client", "0", "Profiles", "default", "actionmaps.xml")
}

// ExePath ist die Spiel-Exe.
func (i Install) ExePath() string {
	return filepath.Join(i.Dir, "Bin64", "StarCitizen.exe")
}

// knownChannels sind die Unterordner, die der RSI Launcher anlegt.
var knownChannels = []string{"LIVE", "PTU", "EPTU", "HOTFIX", "TECH-PREVIEW"}

// libraryRoots sind Ordner, unter denen ein "StarCitizen"-Verzeichnis liegen kann,
// relativ zu einem Laufwerk. Die Spiele-Library ist frei waehlbar, deshalb wird
// zusaetzlich flach gescannt und der Launcher-Log ausgewertet.
var libraryRoots = []string{
	`Roberts Space Industries`,
	`Program Files\Roberts Space Industries`,
	`Program Files (x86)\Roberts Space Industries`,
	`Games\Roberts Space Industries`,
	`Spiele\Roberts Space Industries`,
	`Games`,
	`Spiele`,
	`SC`,
	`RSI`,
	``,
}

// FindInstalls sucht alle Star-Citizen-Installationen auf dem System.
// Reihenfolge der Quellen: RSI-Launcher-Log (am zuverlaessigsten, kennt auch
// exotische Library-Pfade), dann bekannte Ordner auf allen lokalen Laufwerken,
// dann ein flacher Scan der obersten zwei Ebenen jedes Laufwerks.
func FindInstalls() []Install {
	seen := map[string]Install{}

	add := func(gameDir, source string) {
		// gameDir zeigt auf ...\StarCitizen -- darunter liegen die Channels.
		entries, err := os.ReadDir(gameDir)
		if err != nil {
			return
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			dir := filepath.Join(gameDir, e.Name())
			if !isChannelDir(dir) {
				continue
			}
			key := strings.ToLower(dir)
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = Install{
				Channel: strings.ToUpper(e.Name()),
				Dir:     dir,
				Source:  source,
			}
		}
	}

	for _, p := range pathsFromLauncherLog() {
		add(p, "RSI Launcher")
	}
	for _, drive := range fixedDrives() {
		for _, root := range libraryRoots {
			add(filepath.Join(drive, root, "StarCitizen"), "Laufwerksscan")
		}
		for _, p := range shallowScan(drive) {
			add(p, "Laufwerksscan")
		}
	}

	out := make([]Install, 0, len(seen))
	for _, v := range seen {
		out = append(out, v)
	}
	// LIVE zuerst, danach alphabetisch -- die UI nimmt den ersten Treffer als Default.
	sort.Slice(out, func(a, b int) bool {
		if (out[a].Channel == "LIVE") != (out[b].Channel == "LIVE") {
			return out[a].Channel == "LIVE"
		}
		return out[a].Dir < out[b].Dir
	})
	debuglog.Log("starcitizen: %d Installation(en) gefunden", len(out))
	for _, i := range out {
		debuglog.Log("  %s: %s (%s)", i.Channel, i.Dir, i.Source)
	}
	return out
}

// InstallFromDir baut ein Install aus einem manuell gewaehlten Ordner.
// Akzeptiert sowohl den Channel-Ordner (...\LIVE) als auch den Ordner darueber.
func InstallFromDir(dir string) (Install, bool) {
	dir = strings.TrimRight(dir, `\/`)
	if isChannelDir(dir) {
		return Install{
			Channel: strings.ToUpper(filepath.Base(dir)),
			Dir:     dir,
			Source:  "manuell",
		}, true
	}
	for _, ch := range knownChannels {
		if sub := filepath.Join(dir, ch); isChannelDir(sub) {
			return Install{Channel: ch, Dir: sub, Source: "manuell"}, true
		}
		if sub := filepath.Join(dir, "StarCitizen", ch); isChannelDir(sub) {
			return Install{Channel: ch, Dir: sub, Source: "manuell"}, true
		}
	}
	return Install{}, false
}

// isChannelDir prueft, ob der Ordner wirklich eine Spielinstallation ist.
// Die Exe ist das verlaesslichste Merkmal; actionmaps.xml existiert erst,
// nachdem das Spiel einmal lief.
func isChannelDir(dir string) bool {
	if st, err := os.Stat(filepath.Join(dir, "Bin64", "StarCitizen.exe")); err == nil && !st.IsDir() {
		return true
	}
	if st, err := os.Stat(filepath.Join(dir, "Data.p4k")); err == nil && !st.IsDir() {
		return true
	}
	return false
}

// launcherPathRe findet Windows-Pfade, die auf StarCitizen enden. Der Launcher-Log
// ist JSON, Backslashes sind dort verdoppelt -- beide Formen werden akzeptiert.
var launcherPathRe = regexp.MustCompile(`(?i)[A-Za-z]:(?:\\{1,2}[^"'<>|?*\r\n\\]+)*\\{1,2}StarCitizen`)

// pathsFromLauncherLog liest die Installationspfade aus dem RSI-Launcher-Log.
// Das ist die einzige Quelle, die eine beliebig einsortierte Library kennt --
// die Registry enthaelt nur EACServiceInstalled, und "launcher store.json" ist
// verschluesselt.
func pathsFromLauncherLog() []string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return nil
	}

	candidates := []string{
		filepath.Join(appData, "rsilauncher", "logs", "log.log"),
		filepath.Join(appData, "rsilauncher", "log.log"),
	}

	seen := map[string]bool{}
	var out []string
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, m := range launcherPathRe.FindAllString(string(data), -1) {
			p := filepath.Clean(strings.ReplaceAll(m, `\\`, `\`))
			if !seen[strings.ToLower(p)] {
				seen[strings.ToLower(p)] = true
				out = append(out, p)
			}
		}
	}
	return out
}

// shallowScan sucht auf einem Laufwerk in den obersten zwei Ebenen nach einem
// StarCitizen-Ordner. Kein rekursiver Volldurchlauf -- das wuerde bei grossen
// Platten ewig dauern.
func shallowScan(drive string) []string {
	var out []string
	top, err := os.ReadDir(drive)
	if err != nil {
		return nil
	}
	for _, e := range top {
		if !e.IsDir() || skipDir(e.Name()) {
			continue
		}
		lvl1 := filepath.Join(drive, e.Name())
		if strings.EqualFold(e.Name(), "StarCitizen") {
			out = append(out, lvl1)
			continue
		}
		sub, err := os.ReadDir(lvl1)
		if err != nil {
			continue
		}
		for _, s := range sub {
			if s.IsDir() && strings.EqualFold(s.Name(), "StarCitizen") {
				out = append(out, filepath.Join(lvl1, s.Name()))
			}
		}
	}
	return out
}

// skipDir ueberspringt Systemordner, in denen das Spiel garantiert nicht liegt.
func skipDir(name string) bool {
	switch strings.ToLower(name) {
	case "windows", "$recycle.bin", "system volume information", "programdata",
		"perflogs", "recovery", "boot", "msocache", "users", "appdata":
		return true
	}
	return strings.HasPrefix(name, "$")
}

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	procGetDriveType = kernel32.NewProc("GetDriveTypeW")
	procGetDrives    = kernel32.NewProc("GetLogicalDrives")
)

const driveFixed = 3

// fixedDrives liefert alle lokalen Festplatten. Netzlaufwerke und Wechselmedien
// werden ausgelassen -- die koennen beim Scannen minutenlang blockieren.
func fixedDrives() []string {
	mask, _, _ := procGetDrives.Call()
	var out []string
	for i := 0; i < 26; i++ {
		if mask&(1<<uint(i)) == 0 {
			continue
		}
		root := string(rune('A'+i)) + `:\`
		p, err := syscall.UTF16PtrFromString(root)
		if err != nil {
			continue
		}
		if t, _, _ := procGetDriveType.Call(uintptr(unsafe.Pointer(p))); t == driveFixed {
			out = append(out, root)
		}
	}
	return out
}
