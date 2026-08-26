package input

import (
	"strings"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"sctroll/internal/debuglog"
)

var (
	user32            = syscall.NewLazyDLL("user32.dll")
	procSendInput     = user32.NewProc("SendInput")
	procMapVirtualKey = user32.NewProc("MapVirtualKeyW")
)

const (
	INPUT_KEYBOARD        = 1
	INPUT_MOUSE_VAL       = 0
	KEYEVENTF_EXTENDEDKEY = 0x0001
	KEYEVENTF_KEYUP       = 0x0002
	KEYEVENTF_SCANCODE    = 0x0008

	MOUSEEVENTF_LEFTDOWN   = 0x0002
	MOUSEEVENTF_LEFTUP     = 0x0004
	MOUSEEVENTF_RIGHTDOWN  = 0x0008
	MOUSEEVENTF_RIGHTUP    = 0x0010
	MOUSEEVENTF_MIDDLEDOWN = 0x0020
	MOUSEEVENTF_MIDDLEUP   = 0x0040
	MOUSEEVENTF_XDOWN      = 0x0080
	MOUSEEVENTF_XUP        = 0x0100
	MOUSEEVENTF_MOVE       = 0x0001
	XBUTTON1               = 0x0001
	XBUTTON2               = 0x0002
)

type KEYBDINPUT struct {
	Vk        uint16
	Scan      uint16
	Flags     uint32
	Time      uint32
	ExtraInfo uintptr
}

type MOUSEINPUT struct {
	Dx        int32
	Dy        int32
	MouseData uint32
	Flags     uint32
	Time      uint32
	ExtraInfo uintptr
}

type INPUT_KB struct {
	Type uint32
	Ki   KEYBDINPUT
	_    [8]byte // pad to 40 bytes (same as Windows INPUT struct on 64-bit)
}

type INPUT_MOUSE struct {
	Type uint32
	Mi   MOUSEINPUT
	// No extra padding needed — already 40 bytes (4 type + 4 alignment + 32 MOUSEINPUT)
}

// keyMap benutzt die Tastennamen aus Star Citizens actionmaps.xml. Dadurch
// laesst sich ein "kb1_lshift+np_9" aus dem Spielprofil direkt weiterverwenden,
// ohne zweite Uebersetzungstabelle.
//
// F13-F24 fehlen absichtlich: Star Citizen (CryEngine) kennt diese Keycodes
// nicht und kann sie nicht binden.
var keyMap = map[string]uint16{
	"a": 0x41, "b": 0x42, "c": 0x43, "d": 0x44, "e": 0x45, "f": 0x46,
	"g": 0x47, "h": 0x48, "i": 0x49, "j": 0x4A, "k": 0x4B, "l": 0x4C,
	"m": 0x4D, "n": 0x4E, "o": 0x4F, "p": 0x50, "q": 0x51, "r": 0x52,
	"s": 0x53, "t": 0x54, "u": 0x55, "v": 0x56, "w": 0x57, "x": 0x58,
	"y": 0x59, "z": 0x5A,
	"0": 0x30, "1": 0x31, "2": 0x32, "3": 0x33, "4": 0x34,
	"5": 0x35, "6": 0x36, "7": 0x37, "8": 0x38, "9": 0x39,
	"f1": 0x70, "f2": 0x71, "f3": 0x72, "f4": 0x73, "f5": 0x74,
	"f6": 0x75, "f7": 0x76, "f8": 0x77, "f9": 0x78, "f10": 0x79,
	"f11": 0x7A, "f12": 0x7B,

	"space": 0x20, "enter": 0x0D, "tab": 0x09, "escape": 0x1B, "esc": 0x1B,
	"backspace": 0x08, "capslock": 0x14, "numlock": 0x90, "printscreen": 0x2C,

	// Modifier -- Star Citizen unterscheidet links und rechts, deshalb beides.
	"shift": 0xA0, "ctrl": 0xA2, "alt": 0xA4,
	"lshift": 0xA0, "rshift": 0xA1,
	"lctrl": 0xA2, "rctrl": 0xA3,
	"lalt": 0xA4, "ralt": 0xA5,

	"up": 0x26, "down": 0x28, "left": 0x25, "right": 0x27,
	"delete": 0x2E, "insert": 0x2D, "home": 0x24, "end": 0x23,
	"pgup": 0x21, "pgdn": 0x22, "pageup": 0x21, "pagedown": 0x22,

	// Satzzeichen -- SC-Schreibweise, dazu die Symbole als Alias.
	"minus": 0xBD, "equals": 0xBB, "lbracket": 0xDB, "rbracket": 0xDD,
	"backslash": 0xDC, "semicolon": 0xBA, "apostrophe": 0xDE,
	"comma": 0xBC, "period": 0xBE, "slash": 0xBF, "grave": 0xC0,
	"-": 0xBD, "=": 0xBB, "[": 0xDB, "]": 0xDD, "\\": 0xDC,
	";": 0xBA, "'": 0xDE, ",": 0xBC, ".": 0xBE, "/": 0xBF, "`": 0xC0,

	// Ziffernblock.
	"np_0": 0x60, "np_1": 0x61, "np_2": 0x62, "np_3": 0x63, "np_4": 0x64,
	"np_5": 0x65, "np_6": 0x66, "np_7": 0x67, "np_8": 0x68, "np_9": 0x69,
	"np_multiply": 0x6A, "np_add": 0x6B, "np_subtract": 0x6D,
	"np_period": 0x6E, "np_divide": 0x6F,
}

// Mouse button definitions: down flag, up flag, mouseData
type mouseButton struct {
	downFlag uint32
	upFlag   uint32
	xButton  uint32
}

// Maustasten in Star Citizens Zaehlweise: mouse1 ist links, mouse2 rechts,
// mouse3 die mittlere. So steht es in defaultProfile.xml, etwa
// throw_overhand mouse="mouse1" fuer den Wurf.
//
// Vorher zaehlte diese Tabelle ab null und mouse1 war die RECHTE Taste -- wer
// eine Belegung aus dem Spiel abschrieb, bekam die falsche. Die Tastennamen
// folgen dem Spiel schon lange, die Maustasten jetzt auch.
var mouseMap = map[string]mouseButton{
	"mouse1": {MOUSEEVENTF_LEFTDOWN, MOUSEEVENTF_LEFTUP, 0},
	"mouse2": {MOUSEEVENTF_RIGHTDOWN, MOUSEEVENTF_RIGHTUP, 0},
	"mouse3": {MOUSEEVENTF_MIDDLEDOWN, MOUSEEVENTF_MIDDLEUP, 0},
	"mouse4": {MOUSEEVENTF_XDOWN, MOUSEEVENTF_XUP, XBUTTON1},
	"mouse5": {MOUSEEVENTF_XDOWN, MOUSEEVENTF_XUP, XBUTTON2},

	// Eindeutige Namen, unabhaengig von der Zaehlweise.
	"lmouse": {MOUSEEVENTF_LEFTDOWN, MOUSEEVENTF_LEFTUP, 0},
	"rmouse": {MOUSEEVENTF_RIGHTDOWN, MOUSEEVENTF_RIGHTUP, 0},
	"mmouse": {MOUSEEVENTF_MIDDLEDOWN, MOUSEEVENTF_MIDDLEUP, 0},

	// Altlast aus der Tarkov-Vorlage, wo ab null gezaehlt wurde.
	"mouse0": {MOUSEEVENTF_LEFTDOWN, MOUSEEVENTF_LEFTUP, 0},
}

func isMouseKey(key string) bool {
	_, ok := mouseMap[strings.ToLower(key)]
	return ok
}

func sendMouseDown(btn mouseButton) {
	inp := INPUT_MOUSE{
		Type: INPUT_MOUSE_VAL,
		Mi: MOUSEINPUT{
			Flags:     btn.downFlag,
			MouseData: btn.xButton,
		},
	}
	ret, _, err := procSendInput.Call(1, uintptr(unsafe.Pointer(&inp)), unsafe.Sizeof(inp))
	if ret != 1 {
		debuglog.Log("sendMouseDown: BLOCKED flags=0x%X ret=%d err=%v", btn.downFlag, ret, err)
	} else {
		debuglog.Log("sendMouseDown: flags=0x%X ok", btn.downFlag)
	}
}

func sendMouseUp(btn mouseButton) {
	inp := INPUT_MOUSE{
		Type: INPUT_MOUSE_VAL,
		Mi: MOUSEINPUT{
			Flags:     btn.upFlag,
			MouseData: btn.xButton,
		},
	}
	ret, _, err := procSendInput.Call(1, uintptr(unsafe.Pointer(&inp)), unsafe.Sizeof(inp))
	if ret != 1 {
		debuglog.Log("sendMouseUp: BLOCKED flags=0x%X ret=%d err=%v", btn.upFlag, ret, err)
	} else {
		debuglog.Log("sendMouseUp: flags=0x%X ok", btn.upFlag)
	}
}

// INPUT_MOUSE_TYPE_VAL is the SendInput type value for mouse events

func vkToScanCode(vk uint16) uint16 {
	ret, _, _ := procMapVirtualKey.Call(uintptr(vk), 0)
	return uint16(ret)
}

// extendedKeys are VKs that require the KEYEVENTF_EXTENDEDKEY flag (E0 prefix).
// Without it, DirectInput games receive the wrong scancode.
var extendedKeys = map[uint16]bool{
	0xA3: true, // Right Ctrl
	0xA5: true, // Right Alt
	0x2D: true, // Insert
	0x2E: true, // Delete
	0x24: true, // Home
	0x23: true, // End
	0x21: true, // PageUp
	0x22: true, // PageDown
	0x25: true, // Left arrow
	0x26: true, // Up arrow
	0x27: true, // Right arrow
	0x28: true, // Down arrow
	0x90: true, // NumLock
	0x2C: true, // PrintScreen
	0x6F: true, // Numpad /
}

// Sendeverfahren. Welches ein Spiel akzeptiert, laesst sich nicht zuverlaessig
// vorhersagen -- deshalb umschaltbar statt geraten.
//
//	ModeScancode: nur Scancode, Vk=0. Was DirectInput-Spiele typischerweise wollen.
//	ModeVirtual:  nur Virtual-Key. Das macht InputSimulator, womit das
//	              Stream-Deck-Plugin nachweislich in Star Citizen funktioniert.
//	ModeBoth:     erst Scancode, dann Virtual-Key hinterher.
const (
	ModeScancode = "scancode"
	ModeVirtual  = "virtual"
	ModeBoth     = "both"
)

var sendMode atomic.Value // string

func init() { sendMode.Store(ModeScancode) }

// SetMode legt fest, wie Tastendruecke gesendet werden.
func SetMode(mode string) {
	switch mode {
	case ModeScancode, ModeVirtual, ModeBoth:
		sendMode.Store(mode)
		debuglog.Log("input: Sendeverfahren = %s", mode)
	case "":
		// Nicht gesetzt heisst Voreinstellung, nicht Fehler. Stand vorher als
		// "unbekanntes Sendeverfahren" in jedem Log und liess eine kaputte
		// Konfiguration vermuten.
		sendMode.Store(ModeScancode)
		debuglog.Log("input: Sendeverfahren = %s (Voreinstellung)", ModeScancode)
	default:
		sendMode.Store(ModeScancode)
		debuglog.Log("input: unbekanntes Sendeverfahren %q, benutze %s", mode, ModeScancode)
	}
}

func Mode() string {
	m, _ := sendMode.Load().(string)
	if m == "" {
		return ModeScancode
	}
	return m
}

// sendKey sends a single keyboard event.
//
// Beim Scancode-Verfahren muss Vk zwingend 0 sein -- gemischt halten manche
// Spiele die Taste faelschlich fuer dauerhaft gedrueckt. Extended Keys brauchen
// KEYEVENTF_EXTENDEDKEY, sonst kommt die falsche Taste an.
func sendKey(vk uint16, keyUp bool) {
	switch Mode() {
	case ModeVirtual:
		sendVirtual(vk, keyUp)
	case ModeBoth:
		sendScancode(vk, keyUp)
		sendVirtual(vk, keyUp)
	default:
		sendScancode(vk, keyUp)
	}
}

func sendScancode(vk uint16, keyUp bool) {
	scan := vkToScanCode(vk)
	flags := uint32(KEYEVENTF_SCANCODE)
	ext := extendedKeys[vk]
	if ext {
		flags |= KEYEVENTF_EXTENDEDKEY
	}
	if keyUp {
		flags |= KEYEVENTF_KEYUP
	}
	input := INPUT_KB{
		Type: INPUT_KEYBOARD,
		Ki: KEYBDINPUT{
			Vk:    0, // muss 0 sein bei reinem Scancode
			Scan:  scan,
			Flags: flags,
		},
	}
	ret, err := sendOne(&input)
	report("scancode", vk, scan, ext, keyUp, ret, err)
}

func sendVirtual(vk uint16, keyUp bool) {
	flags := uint32(0)
	if keyUp {
		flags |= KEYEVENTF_KEYUP
	}
	input := INPUT_KB{
		Type: INPUT_KEYBOARD,
		Ki: KEYBDINPUT{
			Vk:    vk,
			Scan:  0,
			Flags: flags,
		},
	}
	ret, err := sendOne(&input)
	report("virtual", vk, 0, false, keyUp, ret, err)
}

func sendOne(in *INPUT_KB) (uintptr, error) {
	ret, _, err := procSendInput.Call(1, uintptr(unsafe.Pointer(in)), unsafe.Sizeof(*in))
	return ret, err
}

// report protokolliert das Ergebnis. ret ist die Zahl der eingefuegten Events --
// 0 heisst blockiert (UIPI bei erhoehten Rechten, oder Anti-Cheat) und ist das
// wichtigste Diagnosesignal ueberhaupt.
func report(kind string, vk, scan uint16, ext, keyUp bool, ret uintptr, err error) {
	dir := "down"
	if keyUp {
		dir = "up"
	}
	if ret != 1 {
		debuglog.Log("sendKey[%s]: BLOCKIERT vk=0x%X scan=0x%X ext=%v %s ret=%d err=%v",
			kind, vk, scan, ext, dir, ret, err)
		return
	}
	debuglog.Log("sendKey[%s]: vk=0x%X scan=0x%X ext=%v %s ok", kind, vk, scan, ext, dir)
}

func sendKeyDown(vk uint16) { sendKey(vk, false) }
func sendKeyUp(vk uint16)   { sendKey(vk, true) }

func ResolveKey(key string) (uint16, bool) {
	vk, ok := keyMap[strings.ToLower(strings.TrimSpace(key))]
	return vk, ok
}

// SendKeyUpVK sends a key-up event for a virtual key code.
// Used by KeyLocker to release keys that may be physically held down.
// Sends both scancode-based and VK-based key-up for maximum compatibility.
func SendKeyUpVK(vk uint16) {
	// Scancode-based key-up (works with most games)
	sendKeyUp(vk)
	// Also send VK-only key-up (some games only check VK)
	inp := INPUT_KB{
		Type: INPUT_KEYBOARD,
		Ki: KEYBDINPUT{
			Vk:    vk,
			Flags: KEYEVENTF_KEYUP,
		},
	}
	procSendInput.Call(1, uintptr(unsafe.Pointer(&inp)), unsafe.Sizeof(inp))
	debuglog.Log("SendKeyUpVK: sent dual key-up for vk=0x%X", vk)
}

func IsMouseButton(key string) bool {
	return isMouseKey(key)
}

// SendMouseMove sends a relative mouse move event.
func SendMouseMove(dx, dy int32) {
	inp := INPUT_MOUSE{
		Type: INPUT_MOUSE_VAL,
		Mi: MOUSEINPUT{
			Dx:    dx,
			Dy:    dy,
			Flags: MOUSEEVENTF_MOVE,
		},
	}
	procSendInput.Call(1, uintptr(unsafe.Pointer(&inp)), unsafe.Sizeof(inp))
}

// Spin360 performs a 360-degree horizontal spin by moving the mouse in steps.
func Spin360(totalPixels int, durationMs int, direction int) {
	steps := 60
	stepDelay := time.Duration(durationMs/steps) * time.Millisecond
	pixelsPerStep := int32((totalPixels / steps) * direction)

	for i := 0; i < steps; i++ {
		SendMouseMove(pixelsPerStep, 0)
		time.Sleep(stepDelay)
	}
}

// HoldKeyDown presses a key/mouse down and returns a release function.
func HoldKeyDown(keySpec string) func() {
	key := strings.ToLower(strings.TrimSpace(keySpec))
	if btn, ok := mouseMap[key]; ok {
		sendMouseDown(btn)
		return func() { sendMouseUp(btn) }
	}
	if vk, ok := ResolveKey(key); ok {
		sendKeyDown(vk)
		return func() { sendKeyUp(vk) }
	}
	return func() {}
}

// PressKey simulates pressing a key or mouse button for a given duration.
// keySpec can be "g", "alt+t" for key combos, or "mouse0" for mouse buttons.
func PressKey(keySpec string, holdMs int) {
	parts := strings.Split(strings.ToLower(keySpec), "+")
	debuglog.Log("PressKey: spec=%q holdMs=%d parsed=%v", keySpec, holdMs, parts)

	// Press modifiers first
	var modifiers []uint16
	for _, p := range parts[:max(0, len(parts)-1)] {
		if vk, ok := ResolveKey(p); ok {
			modifiers = append(modifiers, vk)
			sendKeyDown(vk)
			time.Sleep(10 * time.Millisecond)
		} else {
			debuglog.Log("PressKey: UNKNOWN modifier %q in spec %q", p, keySpec)
		}
	}

	// Modifier in umgekehrter Reihenfolge freigeben -- per defer, damit sie
	// auch bei einem Abbruch nicht gedrueckt haengen bleiben.
	defer func() {
		for i := len(modifiers) - 1; i >= 0; i-- {
			sendKeyUp(modifiers[i])
			time.Sleep(10 * time.Millisecond)
		}
	}()

	// Press main key (could be keyboard or mouse)
	mainKey := parts[len(parts)-1]
	if btn, ok := mouseMap[mainKey]; ok {
		sendMouseDown(btn)
		defer sendMouseUp(btn)
		holdFor(holdMs, keySpec)
	} else if vk, ok := ResolveKey(mainKey); ok {
		sendKeyDown(vk)
		defer sendKeyUp(vk)
		holdFor(holdMs, keySpec)
	} else {
		debuglog.Log("PressKey: UNKNOWN main key %q in spec %q — nichts gesendet!", mainKey, keySpec)
	}
}

// holdFor haelt die Taste. Lange Halteaktionen -- die Selbstzerstoerung braucht
// rund fuenfzehn Sekunden -- werden protokolliert, damit im Log nachvollziehbar
// ist, warum in dieser Zeit nichts anderes passiert: die Warteschlange laesst
// bewusst nur eine Aktion gleichzeitig zu.
func holdFor(holdMs int, keySpec string) {
	const longHold = 3000

	if holdMs < longHold {
		time.Sleep(time.Duration(holdMs) * time.Millisecond)
		return
	}

	debuglog.Log("PressKey: %q wird %.1fs gehalten", keySpec, float64(holdMs)/1000)
	deadline := time.Now().Add(time.Duration(holdMs) * time.Millisecond)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		if remaining > time.Second {
			remaining = time.Second
		}
		time.Sleep(remaining)
	}
	debuglog.Log("PressKey: %q wieder losgelassen", keySpec)
}
