package actions

import (
	"fmt"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"sctroll/internal/config"
	"sctroll/internal/debuglog"
	"sctroll/internal/input"
	"sctroll/internal/keylock"
)

var (
	user32                  = syscall.NewLazyDLL("user32.dll")
	kernel32                = syscall.NewLazyDLL("kernel32.dll")
	procGetForegroundWindow = user32.NewProc("GetForegroundWindow")
	procGetWindowText       = user32.NewProc("GetWindowTextW")
	procGetWindowThreadPID  = user32.NewProc("GetWindowThreadProcessId")
	procOpenProcess         = kernel32.NewProc("OpenProcess")
	procCloseHandle         = kernel32.NewProc("CloseHandle")
	procQueryFullProcessImageName = kernel32.NewProc("QueryFullProcessImageNameW")
)

type queuedAction struct {
	action   *config.Action
	userName string
}

type Executor struct {
	mu         sync.Mutex
	cfg        *config.Config
	keyLocker  *keylock.KeyLocker
	cooldowns  map[string]time.Time
	onAction   func(actionID, userName string)
	onCooldown func(actionID string, remainingMs int)
	queue      chan queuedAction
}

func NewExecutor(cfg *config.Config, kl *keylock.KeyLocker) *Executor {
	e := &Executor{
		cfg:       cfg,
		keyLocker: kl,
		cooldowns: make(map[string]time.Time),
		queue:     make(chan queuedAction, 8), // up to 8 queued actions
	}
	go e.runQueue()
	return e
}

// runQueue serializes action execution — only one action runs at a time.
// This prevents key lock conflicts and overlapping key sends.
func (e *Executor) runQueue() {
	for q := range e.queue {
		e.executeAction(q.action)
	}
}

func (e *Executor) SetOnAction(fn func(actionID, userName string)) {
	e.onAction = fn
}

func (e *Executor) SetOnCooldown(fn func(actionID string, remainingMs int)) {
	e.onCooldown = fn
}

func (e *Executor) Execute(actionID string, userName string) error {
	if !e.cfg.GlobalEnable {
		return fmt.Errorf("SCTroll is disabled")
	}

	actions := e.cfg.GetActions()
	var action *config.Action
	for i, a := range actions {
		if a.ID == actionID {
			action = &actions[i]
			break
		}
	}
	if action == nil {
		return fmt.Errorf("action not found: %s", actionID)
	}
	if !action.Enabled {
		return fmt.Errorf("action disabled: %s", actionID)
	}

	// Check cooldown
	e.mu.Lock()
	if cd, ok := e.cooldowns[actionID]; ok {
		remaining := time.Until(cd)
		if remaining > 0 {
			e.mu.Unlock()
			if e.onCooldown != nil {
				e.onCooldown(actionID, int(remaining.Milliseconds()))
			}
			return fmt.Errorf("action on cooldown: %dms remaining", int(remaining.Milliseconds()))
		}
	}
	e.cooldowns[actionID] = time.Now().Add(time.Duration(action.Cooldown) * time.Millisecond)
	e.mu.Unlock()

	// Check target window (by process exe name + window title)
	if !e.isTargetWindowActive() {
		activeInfo := e.getActiveWindowTitle()
		debuglog.Log("Execute: target window not active. Active: %q, looking for: %q",
			activeInfo, e.cfg.TargetWindow)
		return fmt.Errorf("target window not active (active: %s)", activeInfo)
	}

	if e.onAction != nil {
		e.onAction(actionID, userName)
	}

	// Queue the action — runQueue() will execute serially
	select {
	case e.queue <- queuedAction{action: action, userName: userName}:
		debuglog.Log("Execute: %s queued (queue len=%d/%d)", actionID, len(e.queue), cap(e.queue))
	default:
		debuglog.Log("Execute: %s DROPPED — queue full", actionID)
		return fmt.Errorf("queue full — try again in a moment")
	}
	return nil
}

// Test fuehrt eine Aktion sofort aus -- ohne Twitch, ohne Cooldown und ohne
// Ruecksicht auf den Globalschalter. Die Vordergrundpruefung bleibt, denn genau
// die ist bei "es passiert nichts" mit die haeufigste Ursache.
//
// Gedacht zum Nachvollziehen: das Debug-Log zeigt danach pro Taste, ob der
// Tastendruck angenommen oder blockiert wurde.
func (e *Executor) Test(actionID string) error {
	var action *config.Action
	actions := e.cfg.GetActions()
	for i, a := range actions {
		if a.ID == actionID {
			action = &actions[i]
			break
		}
	}
	if action == nil {
		return fmt.Errorf("Aktion nicht gefunden: %s", actionID)
	}
	if action.Key == "" {
		return fmt.Errorf("%s hat keine Taste gesetzt", action.Name)
	}

	if !e.isTargetWindowActive() {
		return fmt.Errorf("%s ist nicht im Vordergrund (aktiv: %s)",
			e.cfg.TargetWindow, e.getActiveWindowTitle())
	}

	debuglog.Log("=== TEST %s: Taste %q, %dms, Verfahren=%s ===",
		action.ID, action.Key, action.HoldMs, input.Mode())
	e.executeAction(action)
	return nil
}

// applyPreActionLock gibt die konfigurierten Tasten frei und blockiert sie,
// solange die Aktion laeuft. Ohne das kann eine physisch gehaltene Taste des
// Streamers die Aktion ueberschreiben -- etwa wenn er beim Aufstehen gerade
// noch W haelt.
//
// Anders als beim Tarkov-Vorlaeufer gibt es hier keine Sonderbehandlung fuer
// Waffenaktionen: Star Citizen senkt die Waffe nicht beim Sprinten, es zaehlt
// nur, was pro Aktion konfiguriert ist.
func (e *Executor) applyPreActionLock(action *config.Action) {
	if !action.KeyLock.Enabled || len(action.KeyLock.Keys) == 0 {
		return
	}

	debuglog.Log("executeAction: pre-action lock %v for %dms", action.KeyLock.Keys, action.KeyLock.Duration)
	e.keyLocker.LockKeys(action.KeyLock.Keys, action.KeyLock.Duration)
	time.Sleep(200 * time.Millisecond) // dem Spiel Zeit geben, die Freigabe zu verarbeiten
}

func (e *Executor) executeAction(action *config.Action) {
	debuglog.Log("executeAction: %s key=%q steps=%d keylock=%v cat=%s",
		action.ID, action.Key, len(action.Steps), action.KeyLock.Enabled, action.Category)

	// Multi-step action
	if len(action.Steps) > 0 {
		heldKeys := make(map[string]func())

		// Release/block sprint + movement before steps (and settle) so the
		// weapon is up before e.g. the grenade sequence runs.
		e.applyPreActionLock(action)

		for i, step := range action.Steps {
			debuglog.Log("executeAction: step %d key=%q holdMs=%d delayMs=%d holdDown=%v release=%q",
				i, step.Key, step.HoldMs, step.DelayMs, step.HoldDown, step.Release)

			if step.DelayMs > 0 {
				time.Sleep(time.Duration(step.DelayMs) * time.Millisecond)
			}
			if step.Release != "" {
				if relFn, ok := heldKeys[step.Release]; ok {
					relFn()
					delete(heldKeys, step.Release)
				}
				continue
			}
			if step.Key == "" {
				continue
			}
			if step.HoldDown {
				relFn := input.HoldKeyDown(step.Key)
				heldKeys[step.Key] = relFn
				continue
			}
			input.PressKey(step.Key, step.HoldMs)
		}
		for k, relFn := range heldKeys {
			debuglog.Log("executeAction: auto-releasing held key %q", k)
			relFn()
		}
		debuglog.Log("executeAction: %s done (multi-step)", action.ID)
		return
	}

	// Special actions
	if strings.HasPrefix(strings.ToLower(action.Key), "spin360") {
		if action.KeyLock.Enabled && len(action.KeyLock.Keys) > 0 {
			e.keyLocker.LockKeys(action.KeyLock.Keys, action.KeyLock.Duration)
			time.Sleep(50 * time.Millisecond)
		}
		direction := 1 // right
		if strings.Contains(action.Key, "left") {
			direction = -1
		}
		pixels := action.HoldMs // reuse hold_ms as pixel count for spin
		if pixels <= 0 {
			pixels = 8000 // default ~360 degrees
		}
		durationMs := 500
		if action.Cooldown > 0 && action.Cooldown < 2000 {
			durationMs = action.Cooldown
		}
		debuglog.Log("executeAction: %s spin360 pixels=%d dir=%d duration=%dms", action.ID, pixels, direction, durationMs)
		input.Spin360(pixels, durationMs, direction)
		debuglog.Log("executeAction: %s done (spin360)", action.ID)
		return
	}

	// Release/block sprint + movement (and settle) so weapon actions like
	// reload / mag-swap register even while the streamer is sprinting.
	e.applyPreActionLock(action)

	// Repeat support
	repeatCount := action.Repeat
	if repeatCount <= 0 {
		repeatCount = 1
	}
	repeatDelay := action.RepeatDelayMs
	if repeatDelay <= 0 {
		repeatDelay = 100
	}

	for i := 0; i < repeatCount; i++ {
		if i > 0 {
			time.Sleep(time.Duration(repeatDelay) * time.Millisecond)
		}
		input.PressKey(action.Key, action.HoldMs)
		debuglog.Log("executeAction: %s press %d/%d", action.ID, i+1, repeatCount)
	}
	debuglog.Log("executeAction: %s done (repeat=%d)", action.ID, repeatCount)
}

const processQueryLimitedInformation = 0x1000

func (e *Executor) getForegroundProcessName() string {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return ""
	}

	var pid uint32
	procGetWindowThreadPID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 {
		return ""
	}

	hProc, _, _ := procOpenProcess.Call(processQueryLimitedInformation, 0, uintptr(pid))
	if hProc == 0 {
		return ""
	}
	defer procCloseHandle.Call(hProc)

	buf := make([]uint16, 1024)
	size := uint32(len(buf))
	ret, _, _ := procQueryFullProcessImageName.Call(hProc, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if ret == 0 {
		return ""
	}

	fullPath := syscall.UTF16ToString(buf[:size])
	// Extract just the filename
	parts := strings.Split(fullPath, "\\")
	return parts[len(parts)-1]
}

func (e *Executor) isTargetWindowActive() bool {
	target := e.cfg.TargetWindow
	if target == "" {
		return true
	}

	// Check by process exe name (more reliable than window title)
	procName := e.getForegroundProcessName()
	if procName != "" && strings.Contains(strings.ToLower(procName), strings.ToLower(target)) {
		return true
	}

	// Fallback: check window title
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return false
	}
	buf := make([]uint16, 256)
	procGetWindowText.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), 256)
	title := syscall.UTF16ToString(buf)

	return strings.Contains(strings.ToLower(title), strings.ToLower(target))
}

func (e *Executor) getActiveWindowTitle() string {
	// Show both process name and window title for debugging
	procName := e.getForegroundProcessName()

	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return procName
	}
	buf := make([]uint16, 256)
	procGetWindowText.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), 256)
	title := syscall.UTF16ToString(buf)

	if procName != "" {
		return fmt.Sprintf("%s [%s]", title, procName)
	}
	return title
}

func (e *Executor) GetCooldownRemaining(actionID string) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	if cd, ok := e.cooldowns[actionID]; ok {
		remaining := time.Until(cd)
		if remaining > 0 {
			return int(remaining.Milliseconds())
		}
	}
	return 0
}
