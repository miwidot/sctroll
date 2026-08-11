package keylock

import (
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"sctroll/internal/debuglog"
	"sctroll/internal/input"
)

var (
	user32              = syscall.NewLazyDLL("user32.dll")
	kernel32            = syscall.NewLazyDLL("kernel32.dll")
	procSetWindowsHookEx = user32.NewProc("SetWindowsHookExW")
	procCallNextHookEx   = user32.NewProc("CallNextHookEx")
	procUnhookWindowsHookEx = user32.NewProc("UnhookWindowsHookEx")
	procGetMessage       = user32.NewProc("GetMessageW")
	procPeekMessage      = user32.NewProc("PeekMessageW")
	procTranslateMessage = user32.NewProc("TranslateMessage")
	procDispatchMessage  = user32.NewProc("DispatchMessageW")
	procGetModuleHandle  = kernel32.NewProc("GetModuleHandleW")
	procGetForegroundWindow = user32.NewProc("GetForegroundWindow")
	procGetWindowText       = user32.NewProc("GetWindowTextW")
)

const (
	WH_KEYBOARD_LL = 13
	WH_MOUSE_LL    = 14
	WM_KEYDOWN     = 0x0100
	WM_KEYUP       = 0x0101
	WM_SYSKEYDOWN  = 0x0104
	WM_SYSKEYUP    = 0x0105
	WM_MOUSEMOVE   = 0x0200
	WM_LBUTTONDOWN = 0x0201
	WM_LBUTTONUP   = 0x0202
	WM_RBUTTONDOWN = 0x0204
	WM_RBUTTONUP   = 0x0205
)

type KBDLLHOOKSTRUCT struct {
	VkCode      uint32
	ScanCode    uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

type MSG struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

type KeyLocker struct {
	mu              sync.RWMutex
	lockedKeys      map[uint16]bool
	lockMouse       bool
	hookHandle      uintptr
	mouseHookHandle uintptr
	pumpRunning     bool
	hookInstalled   bool
	stopCh          chan struct{}
	onKeyBlocked    func(key string)
	targetWindow    string
	targetActive    bool // cached: is target window in foreground?
	lockCount       int32 // atomic: how many active locks
}

var instance *KeyLocker

func New(targetWindow string) *KeyLocker {
	kl := &KeyLocker{
		lockedKeys:   make(map[uint16]bool),
		stopCh:       make(chan struct{}),
		targetWindow: targetWindow,
	}
	instance = kl
	return kl
}

func (kl *KeyLocker) Start() error {
	kl.mu.Lock()
	if kl.pumpRunning {
		kl.mu.Unlock()
		return nil
	}
	kl.pumpRunning = true
	kl.mu.Unlock()

	go kl.messagePump()
	return nil
}

func (kl *KeyLocker) Stop() {
	kl.mu.Lock()
	defer kl.mu.Unlock()
	if !kl.pumpRunning {
		return
	}
	kl.pumpRunning = false
	// Hook will be removed in the message pump
}

func (kl *KeyLocker) LockKeys(keys []string, durationMs int) {
	debuglog.Log("KeyLock: locking keys %v for %dms", keys, durationMs)
	kl.mu.Lock()
	var lockedVKs []uint16
	for _, key := range keys {
		if key == "mouse" {
			kl.lockMouse = true
			debuglog.Log("KeyLock: mouse locked")
			continue
		}
		if vk, ok := input.ResolveKey(key); ok {
			kl.lockedKeys[vk] = true
			lockedVKs = append(lockedVKs, vk)
			debuglog.Log("KeyLock: locked vk=0x%X (%s)", vk, key)
		} else {
			debuglog.Log("KeyLock: UNKNOWN key %q — kann nicht locken!", key)
		}
	}
	kl.mu.Unlock()

	// Increment lock counter — message pump will install hook when count > 0
	atomic.AddInt32(&kl.lockCount, 1)

	// Wait briefly for hook to actually be installed (non-blocking poll)
	for i := 0; i < 100; i++ {
		kl.mu.RLock()
		installed := kl.hookInstalled
		kl.mu.RUnlock()
		if installed {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	// Send key-up events multiple times to force-release in-game
	for round := 0; round < 3; round++ {
		for _, vk := range lockedVKs {
			input.SendKeyUpVK(vk)
		}
		if round < 2 {
			time.Sleep(20 * time.Millisecond)
		}
	}
	debuglog.Log("KeyLock: sent 3x key-up for %d keys (count=%d)", len(lockedVKs), atomic.LoadInt32(&kl.lockCount))

	go func() {
		time.Sleep(time.Duration(durationMs) * time.Millisecond)
		kl.mu.Lock()
		kl.lockMouse = false
		for _, key := range keys {
			if vk, ok := input.ResolveKey(key); ok {
				delete(kl.lockedKeys, vk)
			}
		}
		kl.mu.Unlock()
		// Decrement counter — pump will uninstall hook when count reaches 0
		atomic.AddInt32(&kl.lockCount, -1)
		debuglog.Log("KeyLock: keys unlocked after %dms (count=%d)", durationMs, atomic.LoadInt32(&kl.lockCount))
	}()
}

func (kl *KeyLocker) IsMouseLocked() bool {
	kl.mu.RLock()
	defer kl.mu.RUnlock()
	return kl.lockMouse
}

func (kl *KeyLocker) IsLocked(vk uint16) bool {
	kl.mu.RLock()
	defer kl.mu.RUnlock()
	return kl.lockedKeys[vk]
}

func (kl *KeyLocker) GetLockedKeys() []string {
	kl.mu.RLock()
	defer kl.mu.RUnlock()
	var keys []string
	for vk := range kl.lockedKeys {
		keys = append(keys, vkToName(vk))
	}
	return keys
}

func (kl *KeyLocker) SetOnKeyBlocked(fn func(key string)) {
	kl.mu.Lock()
	defer kl.mu.Unlock()
	kl.onKeyBlocked = fn
}

// updateTargetWindowCache checks if the target window is active and caches the result.
// Called periodically from the message pump, NOT from the hook callback.
func (kl *KeyLocker) updateTargetWindowCache() {
	if kl.targetWindow == "" {
		kl.mu.Lock()
		kl.targetActive = true
		kl.mu.Unlock()
		return
	}
	hwnd, _, _ := procGetForegroundWindow.Call()
	active := false
	if hwnd != 0 {
		buf := make([]uint16, 256)
		procGetWindowText.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), 256)
		title := strings.ToLower(syscall.UTF16ToString(buf))
		active = strings.Contains(title, strings.ToLower(kl.targetWindow))
	}
	kl.mu.Lock()
	kl.targetActive = active
	kl.mu.Unlock()
}

func (kl *KeyLocker) isTargetActive() bool {
	kl.mu.RLock()
	defer kl.mu.RUnlock()
	return kl.targetActive
}

func hookCallback(nCode int, wParam uintptr, lParam uintptr) uintptr {
	if instance == nil || nCode < 0 {
		ret, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
		return ret
	}

	// CRITICAL: This callback must return FAST (<300ms) or Windows kills the hook.
	// Only check cached values — no API calls, no file I/O, no logging here.
	if wParam == WM_KEYDOWN || wParam == WM_SYSKEYDOWN {
		kbs := (*KBDLLHOOKSTRUCT)(unsafe.Pointer(lParam))
		vk := uint16(kbs.VkCode)

		if instance.IsLocked(vk) && instance.isTargetActive() {
			if instance.onKeyBlocked != nil {
				go instance.onKeyBlocked(vkToName(vk))
			}
			return 1 // Block the key
		}
	}

	ret, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
	return ret
}

func mouseHookCallback(nCode int, wParam uintptr, lParam uintptr) uintptr {
	if instance == nil || nCode < 0 {
		ret, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
		return ret
	}

	if instance.IsMouseLocked() {
		if wParam == WM_MOUSEMOVE || wParam == WM_LBUTTONDOWN || wParam == WM_LBUTTONUP ||
			wParam == WM_RBUTTONDOWN || wParam == WM_RBUTTONUP {
			return 1
		}
	}

	ret, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
	return ret
}

func (kl *KeyLocker) installHook() {
	if kl.hookInstalled {
		return
	}
	modHandle, _, _ := procGetModuleHandle.Call(0)
	cb := syscall.NewCallback(hookCallback)
	hook, _, err := procSetWindowsHookEx.Call(WH_KEYBOARD_LL, cb, modHandle, 0)
	if hook == 0 {
		debuglog.Log("KeyLock: FEHLER SetWindowsHookEx: %s", err.Error())
		return
	}
	kl.mu.Lock()
	kl.hookHandle = hook
	kl.hookInstalled = true
	kl.mu.Unlock()
	debuglog.Log("KeyLock: Hook installiert")
}

func (kl *KeyLocker) removeHook() {
	if !kl.hookInstalled {
		return
	}
	if kl.hookHandle != 0 {
		procUnhookWindowsHookEx.Call(kl.hookHandle)
	}
	kl.mu.Lock()
	kl.hookHandle = 0
	kl.hookInstalled = false
	kl.mu.Unlock()
	debuglog.Log("KeyLock: Hook entfernt")
}

func (kl *KeyLocker) messagePump() {
	// Pin to OS thread — Windows hooks are tied to the thread that installs them
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	var msg MSG
	lastWindowCheck := time.Now()

	for {
		kl.mu.RLock()
		running := kl.pumpRunning
		kl.mu.RUnlock()
		if !running {
			kl.removeHook()
			break
		}

		// Ref-counted hook lifecycle: install when count > 0, remove when count == 0
		count := atomic.LoadInt32(&kl.lockCount)
		kl.mu.RLock()
		installed := kl.hookInstalled
		kl.mu.RUnlock()
		if count > 0 && !installed {
			kl.installHook()
			kl.updateTargetWindowCache()
		} else if count == 0 && installed {
			kl.removeHook()
		}

		// Update target window cache every 500ms (only when hook is active)
		if kl.hookInstalled && time.Since(lastWindowCheck) > 500*time.Millisecond {
			kl.updateTargetWindowCache()
			lastWindowCheck = time.Now()
		}

		// PeekMessage with PM_REMOVE (0x0001) to keep the pump running
		ret, _, _ := procPeekMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0, 1)
		if ret != 0 {
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
			procDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
		} else {
			time.Sleep(5 * time.Millisecond)
		}
	}
}

func vkToName(vk uint16) string {
	names := map[uint16]string{
		0x41: "a", 0x42: "b", 0x43: "c", 0x44: "d", 0x45: "e", 0x46: "f",
		0x47: "g", 0x48: "h", 0x49: "i", 0x4A: "j", 0x4B: "k", 0x4C: "l",
		0x4D: "m", 0x4E: "n", 0x4F: "o", 0x50: "p", 0x51: "q", 0x52: "r",
		0x53: "s", 0x54: "t", 0x55: "u", 0x56: "v", 0x57: "w", 0x58: "x",
		0x59: "y", 0x5A: "z", 0x20: "space", 0x0D: "enter", 0x09: "tab",
		0x1B: "escape", 0xA0: "shift", 0xA2: "ctrl", 0xA4: "alt",
	}
	if name, ok := names[vk]; ok {
		return name
	}
	return "unknown"
}
