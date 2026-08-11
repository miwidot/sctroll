package starcitizen

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"sctroll/internal/debuglog"
)

// node ist ein generischer XML-Knoten. actionmaps.xml enthaelt neben den
// Tastenbelegungen auch Joystick-Deadzones, Geraeteoptionen und Modifier --
// nichts davon wird modelliert, aber alles muss beim Speichern erhalten bleiben.
// Deshalb wird das Dokument generisch ein- und wieder ausgelesen.
type node struct {
	XMLName  xml.Name
	Attrs    []xml.Attr `xml:",any,attr"`
	Children []*node    `xml:",any"`
	Text     string     `xml:",chardata"`
}

func (n *node) attr(name string) string {
	for _, a := range n.Attrs {
		if strings.EqualFold(a.Name.Local, name) {
			return a.Value
		}
	}
	return ""
}

func (n *node) setAttr(name, value string) {
	for i := range n.Attrs {
		if strings.EqualFold(n.Attrs[i].Name.Local, name) {
			n.Attrs[i].Value = value
			return
		}
	}
	n.Attrs = append(n.Attrs, xml.Attr{Name: xml.Name{Local: name}, Value: value})
}

// child sucht ein Kindelement mit passendem Tag und optional passendem name-Attribut.
func (n *node) child(tag, nameAttr string) *node {
	for _, c := range n.Children {
		if !strings.EqualFold(c.XMLName.Local, tag) {
			continue
		}
		if nameAttr == "" || c.attr("name") == nameAttr {
			return c
		}
	}
	return nil
}

// tidy entfernt reinen Whitespace-Text. Ohne das landet die urspruengliche
// Einrueckung als Textinhalt im Dokument und wird beim Neuschreiben vor die
// Kindelemente gezogen.
func (n *node) tidy() {
	if len(n.Children) > 0 || strings.TrimSpace(n.Text) == "" {
		n.Text = ""
	}
	for _, c := range n.Children {
		c.tidy()
	}
}

// ActionMaps ist die geladene actionmaps.xml.
type ActionMaps struct {
	Path string
	root *node
}

// Load liest actionmaps.xml. Die Datei existiert erst, nachdem Star Citizen
// einmal gestartet wurde.
func Load(path string) (*ActionMaps, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("actionmaps.xml nicht lesbar: %w", err)
	}
	var root node
	if err := xml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("actionmaps.xml ist kaputt: %w", err)
	}
	root.tidy()
	return &ActionMaps{Path: path, root: &root}, nil
}

// profiles liefert das <ActionProfiles>-Element, unter dem alle actionmaps haengen.
func (a *ActionMaps) profiles() *node {
	if p := a.root.child("ActionProfiles", ""); p != nil {
		return p
	}
	// Sollte nie passieren, aber lieber anlegen als crashen.
	p := &node{XMLName: xml.Name{Local: "ActionProfiles"}}
	p.setAttr("profileName", "default")
	a.root.Children = append(a.root.Children, p)
	return p
}

// KeyboardBind liefert die Tastaturbelegung einer Aktion, z.B. "ralt+1".
// Leerer String heisst: keine Tastaturbelegung gesetzt. Achtung -- das bedeutet
// nicht "unbelegt": Aktionen mit Standardbelegung tauchen in actionmaps.xml
// gar nicht auf, dort steht nur, was der Spieler geaendert hat.
func (a *ActionMaps) KeyboardBind(actionmap, action string) string {
	am := a.profiles().child("actionmap", actionmap)
	if am == nil {
		return ""
	}
	act := am.child("action", action)
	if act == nil {
		return ""
	}
	for _, rb := range act.Children {
		if !strings.EqualFold(rb.XMLName.Local, "rebind") {
			continue
		}
		if key, ok := parseKeyboardInput(rb.attr("input")); ok {
			return key
		}
	}
	return ""
}

// AllKeyboardBinds liefert alle Tastaturbelegungen als "actionmap|action" -> Taste.
//
// Was hier NICHT drinsteht, liegt auf Star Citizens Standardbelegung: die Datei
// enthaelt ausschliesslich Abweichungen. Eine Aktion nur auf Joystick zu legen
// entfernt ihre Tastaturbelegung nicht -- die bleibt daneben bestehen.
func (a *ActionMaps) AllKeyboardBinds() map[string]string {
	out := map[string]string{}
	for _, am := range a.profiles().Children {
		if !strings.EqualFold(am.XMLName.Local, "actionmap") {
			continue
		}
		mapName := am.attr("name")
		for _, act := range am.Children {
			if !strings.EqualFold(act.XMLName.Local, "action") {
				continue
			}
			for _, rb := range act.Children {
				if !strings.EqualFold(rb.XMLName.Local, "rebind") {
					continue
				}
				if key, ok := parseKeyboardInput(rb.attr("input")); ok {
					out[mapName+"|"+act.attr("name")] = key
					break
				}
			}
		}
	}
	return out
}

// RemoveKeyboardBind entfernt die Tastaturbelegung einer Aktion wieder, sodass
// Star Citizens Standardbelegung wieder greift. Rebinds anderer Geraete bleiben.
// Meldet, ob etwas entfernt wurde.
func (a *ActionMaps) RemoveKeyboardBind(actionmap, action string) bool {
	am := a.profiles().child("actionmap", actionmap)
	if am == nil {
		return false
	}
	act := am.child("action", action)
	if act == nil {
		return false
	}

	kept := act.Children[:0]
	removed := false
	for _, rb := range act.Children {
		isKeyboard := strings.EqualFold(rb.XMLName.Local, "rebind") &&
			strings.HasPrefix(strings.ToLower(rb.attr("input")), "kb1_")
		if isKeyboard {
			removed = true
			continue
		}
		kept = append(kept, rb)
	}
	act.Children = kept

	// Eine Aktion ohne jedes Rebind hat in der Datei nichts mehr verloren --
	// leer stehengelassen wuerde sie die Belegung als "entfernt" festschreiben.
	if len(act.Children) == 0 {
		siblings := am.Children[:0]
		for _, c := range am.Children {
			if c != act {
				siblings = append(siblings, c)
			}
		}
		am.Children = siblings
	}
	return removed
}

// SetKeyboardBind setzt die Tastaturbelegung einer Aktion. Belegungen anderer
// Geraete (Joystick, Gamepad, Maus) bleiben unangetastet -- fuer eine Aktion
// koennen mehrere <rebind> nebeneinander stehen.
func (a *ActionMaps) SetKeyboardBind(actionmap, action, key string) {
	profiles := a.profiles()

	am := profiles.child("actionmap", actionmap)
	if am == nil {
		am = &node{XMLName: xml.Name{Local: "actionmap"}}
		am.setAttr("name", actionmap)
		profiles.Children = append(profiles.Children, am)
	}

	act := am.child("action", action)
	if act == nil {
		act = &node{XMLName: xml.Name{Local: "action"}}
		act.setAttr("name", action)
		am.Children = append(am.Children, act)
	}

	input := "kb1_" + key
	for _, rb := range act.Children {
		if !strings.EqualFold(rb.XMLName.Local, "rebind") {
			continue
		}
		if strings.HasPrefix(strings.ToLower(rb.attr("input")), "kb1_") {
			rb.setAttr("input", input)
			return
		}
	}
	rb := &node{XMLName: xml.Name{Local: "rebind"}}
	rb.setAttr("input", input)
	act.Children = append(act.Children, rb)
}

// Save schreibt die Datei zurueck und legt vorher eine Sicherung an.
func (a *ActionMaps) Save() error {
	if err := a.backup(); err != nil {
		return err
	}

	out, err := xml.MarshalIndent(a.root, "", " ")
	if err != nil {
		return err
	}
	// Star Citizen schreibt die Datei ohne XML-Deklaration -- das bleibt so.
	if err := os.WriteFile(a.Path, append(out, '\n'), 0o644); err != nil {
		return err
	}
	debuglog.Log("actionmaps: gespeichert -> %s", a.Path)
	return nil
}

// backup kopiert die aktuelle actionmaps.xml mit Zeitstempel daneben.
// Star Citizen ueberschreibt die Datei beim Beenden, deshalb ist ein
// wiederherstellbarer Stand hier wichtiger als anderswo.
func (a *ActionMaps) backup() error {
	data, err := os.ReadFile(a.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	dst := filepath.Join(filepath.Dir(a.Path),
		fmt.Sprintf("actionmaps.sctroll-backup-%s.xml", time.Now().Format("20060102-150405")))
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		return fmt.Errorf("backup fehlgeschlagen: %w", err)
	}
	debuglog.Log("actionmaps: backup -> %s", dst)
	return nil
}

// parseKeyboardInput zerlegt einen actionmaps-Input wie "kb1_ralt+1".
// Nur kb1_ ist relevant; js1_/js2_/gp1_/mo1_ sind Joystick, Gamepad und Maus
// und lassen sich nicht per SendInput ausloesen.
func parseKeyboardInput(input string) (string, bool) {
	if !strings.HasPrefix(strings.ToLower(input), "kb1_") {
		return "", false
	}
	key := input[len("kb1_"):]
	// "kb1_ " (mit Leerzeichen) ist die Schreibweise fuer "bewusst entfernt".
	if strings.TrimSpace(key) == "" {
		return "", false
	}
	return strings.ToLower(key), true
}
