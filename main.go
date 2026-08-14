package main

import (
	"embed"
	"os"
	"syscall"
	"time"
	"unsafe"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"sctroll/internal/updater"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Nach einem Selbstupdate läuft die alte Instanz noch einen Moment weiter.
	// Dann etwas warten, statt sofort "läuft bereits" zu melden -- sonst bliebe
	// nach dem Update gar nichts laufen.
	wait := time.Duration(0)
	for _, arg := range os.Args[1:] {
		if arg == updater.UpdatedFlag {
			wait = 15 * time.Second
			break
		}
	}

	if !acquireSingleInstance(wait) {
		user32 := syscall.NewLazyDLL("user32.dll")
		msgBox := user32.NewProc("MessageBoxW")
		title, _ := syscall.UTF16PtrFromString("SCTroll")
		msg, _ := syscall.UTF16PtrFromString("SCTroll läuft bereits!")
		msgBox.Call(0, uintptr(unsafe.Pointer(msg)), uintptr(unsafe.Pointer(title)), 0x30) // MB_ICONWARNING
		os.Exit(0)
	}
	defer releaseSingleInstance()

	app := NewApp()

	err := wails.Run(&options.App{
		Title:  "SCTroll",
		Width:  1100,
		Height: 750,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 18, G: 18, B: 18, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
