package main

import (
	"syscall"
	"time"
	"unsafe"
)

// Einzelinstanz-Sperre über einen benannten Mutex.
//
// Beim Selbstupdate ist die Reihenfolge heikel: die neue Programmdatei wird
// gestartet, während die alte noch läuft. Ohne Rücksicht darauf träfe der
// Nachfolger auf die Sperre der noch laufenden Instanz, meldete "läuft bereits"
// und beendete sich -- die alte verabschiedet sich danach planmäßig, und am Ende
// läuft gar nichts mehr.
//
// Deshalb zweifach abgesichert: die alte Instanz gibt die Sperre vor dem Start
// des Nachfolgers frei, und der Nachfolger wartet zusätzlich kurz, falls sie
// noch gehalten wird.
const singleInstanceName = `Global\SCTroll_SingleInstance`

const errAlreadyExists = syscall.Errno(183) // ERROR_ALREADY_EXISTS

var (
	kernel32DLL     = syscall.NewLazyDLL("kernel32.dll")
	procCreateMutex = kernel32DLL.NewProc("CreateMutexW")

	singleInstanceHandle syscall.Handle
)

// acquireSingleInstance versucht die Sperre zu belegen und wartet dabei bis zu
// waitFor, falls sie noch von einer beendenden Instanz gehalten wird.
// Rückgabe false heißt: es läuft bereits eine andere Instanz.
func acquireSingleInstance(waitFor time.Duration) bool {
	name, err := syscall.UTF16PtrFromString(singleInstanceName)
	if err != nil {
		return true // ohne Sperre weitermachen ist besser als gar nicht starten
	}

	deadline := time.Now().Add(waitFor)
	for {
		handle, _, callErr := procCreateMutex.Call(0, 0, uintptr(unsafe.Pointer(name)))
		if handle != 0 && callErr != errAlreadyExists {
			singleInstanceHandle = syscall.Handle(handle)
			return true
		}
		if handle != 0 {
			// Existierte schon: unser Handle darauf wieder schließen.
			syscall.CloseHandle(syscall.Handle(handle))
		}
		if !time.Now().Before(deadline) {
			return false
		}
		time.Sleep(150 * time.Millisecond)
	}
}

// releaseSingleInstance gibt die Sperre frei, damit der Nachfolger sie
// übernehmen kann. Muss vor dem Start der neuen Programmdatei passieren.
func releaseSingleInstance() {
	if singleInstanceHandle != 0 {
		syscall.CloseHandle(singleInstanceHandle)
		singleInstanceHandle = 0
	}
}
