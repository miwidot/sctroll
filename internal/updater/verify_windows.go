package updater

import (
	"fmt"
	"syscall"
	"unsafe"
)

// Signaturpruefung ueber WinVerifyTrust -- dieselbe Pruefung, die Windows beim
// Ausfuehren macht.
//
// Warum das noetig ist: die SHA256-Pruefsumme kommt aus derselben GitHub-Version
// wie die Datei. Sie belegt einen heilen Download, mehr nicht. Erst die Signatur
// belegt, dass die Datei wirklich von uns stammt. Ohne diesen Schritt waere die
// Update-Funktion ein bequemer Weg, fremden Code auszufuehren.
var (
	wintrust           = syscall.NewLazyDLL("wintrust.dll")
	procWinVerifyTrust = wintrust.NewProc("WinVerifyTrust")
)

const (
	wtdUINone            = 2
	wtdRevokeNone        = 0
	wtdChoiceFile        = 1
	wtdStateActionVerify = 1
	wtdStateActionClose  = 2
	wtdSafer             = 0x100
	wtdCacheOnlyURL      = 0x1000 // kein Netzzugriff fuer Sperrlisten
)

// WINTRUST_ACTION_GENERIC_VERIFY_V2: {00AAC56B-CD44-11d0-8CC2-00C04FC295EE}
var actionGenericVerifyV2 = syscall.GUID{
	Data1: 0x00AAC56B,
	Data2: 0xCD44,
	Data3: 0x11D0,
	Data4: [8]byte{0x8C, 0xC2, 0x00, 0xC0, 0x4F, 0xC2, 0x95, 0xEE},
}

type wintrustFileInfo struct {
	cbStruct       uint32
	pcwszFilePath  *uint16
	hFile          syscall.Handle
	pgKnownSubject uintptr
}

type wintrustData struct {
	cbStruct            uint32
	pPolicyCallbackData uintptr
	pSIPClientData      uintptr
	dwUIChoice          uint32
	fdwRevocationChecks uint32
	dwUnionChoice       uint32
	pFile               uintptr
	dwStateAction       uint32
	hWVTStateData       syscall.Handle
	pwszURLReference    *uint16
	dwProvFlags         uint32
	dwUIContext         uint32
	pSignatureSettings  uintptr
}

// verifySignature prueft die Authenticode-Signatur einer Datei.
func verifySignature(path string) error {
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}

	fileInfo := wintrustFileInfo{
		pcwszFilePath: pathPtr,
	}
	fileInfo.cbStruct = uint32(unsafe.Sizeof(fileInfo))

	data := wintrustData{
		dwUIChoice:          wtdUINone,
		fdwRevocationChecks: wtdRevokeNone,
		dwUnionChoice:       wtdChoiceFile,
		pFile:               uintptr(unsafe.Pointer(&fileInfo)),
		dwStateAction:       wtdStateActionVerify,
		dwProvFlags:         wtdSafer | wtdCacheOnlyURL,
	}
	data.cbStruct = uint32(unsafe.Sizeof(data))

	ret, _, _ := procWinVerifyTrust.Call(
		0, // kein Fenster, keine Rueckfrage
		uintptr(unsafe.Pointer(&actionGenericVerifyV2)),
		uintptr(unsafe.Pointer(&data)),
	)

	// Der Zustand muss in jedem Fall wieder freigegeben werden.
	data.dwStateAction = wtdStateActionClose
	procWinVerifyTrust.Call(
		0,
		uintptr(unsafe.Pointer(&actionGenericVerifyV2)),
		uintptr(unsafe.Pointer(&data)),
	)

	if ret == 0 {
		return nil
	}
	return fmt.Errorf("%s", trustError(uint32(ret)))
}

// trustError uebersetzt die haeufigen Rueckgaben in verstaendlichen Text.
func trustError(code uint32) string {
	switch code {
	case 0x800B0100:
		return "die Datei ist nicht signiert"
	case 0x800B0101:
		return "das Signaturzertifikat ist abgelaufen"
	case 0x800B0109:
		return "das Signaturzertifikat stammt von keiner vertrauenswürdigen Stelle"
	case 0x80096010:
		return "die Datei wurde nach dem Signieren verändert"
	case 0x800B010C:
		return "das Signaturzertifikat wurde widerrufen"
	default:
		return fmt.Sprintf("Signaturprüfung fehlgeschlagen (0x%08X)", code)
	}
}
