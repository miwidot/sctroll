package starcitizen

import (
	"strings"
	"syscall"
	"unsafe"
)

var (
	procCreateToolhelp32Snapshot = kernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32FirstW          = kernel32.NewProc("Process32FirstW")
	procProcess32NextW           = kernel32.NewProc("Process32NextW")
	procCloseHandle              = kernel32.NewProc("CloseHandle")
)

const th32csSnapProcess = 0x00000002

type processEntry32 struct {
	Size            uint32
	Usage           uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	Threads         uint32
	ParentProcessID uint32
	PriClassBase    int32
	Flags           uint32
	ExeFile         [260]uint16
}

// IsGameRunning meldet, ob Star Citizen gerade laeuft. Das Spiel schreibt seine
// Tastenbelegungen beim Beenden zurueck -- Aenderungen an actionmaps.xml bei
// laufendem Spiel waeren also spaetestens dann wieder weg.
func IsGameRunning() bool {
	return processRunning("StarCitizen.exe")
}

func processRunning(exeName string) bool {
	snap, _, _ := procCreateToolhelp32Snapshot.Call(th32csSnapProcess, 0)
	if snap == 0 || snap == uintptr(syscall.InvalidHandle) {
		return false
	}
	defer procCloseHandle.Call(snap)

	var e processEntry32
	e.Size = uint32(unsafe.Sizeof(e))

	ok, _, _ := procProcess32FirstW.Call(snap, uintptr(unsafe.Pointer(&e)))
	for ok != 0 {
		if strings.EqualFold(syscall.UTF16ToString(e.ExeFile[:]), exeName) {
			return true
		}
		ok, _, _ = procProcess32NextW.Call(snap, uintptr(unsafe.Pointer(&e)))
	}
	return false
}
