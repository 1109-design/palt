// main.go is the Wails entrypoint for PALT.
//
// Structure matches the established pattern used in this workspace
// (see tardis/main.go). Key points:
//   - The //go:embed directive bundles the compiled React frontend.
//   - linux.Options sets GPU policy to OnDemand (avoids crashes on some GPUs).
//   - Bind: []interface{}{app} exposes all exported App methods to the frontend.
package main

import (
	"embed"
	"os"
	"slices"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

// assets embeds the compiled React frontend into the Go binary so the final
// .deb / binary is fully self-contained with no external web server.
//
//go:embed all:frontend/dist
var assets embed.FS

// instanceID names the single-instance lock shared by every PALT launch.
const instanceID = "com.lads.palt.desktop"

func main() {
	if signalRunningInstance(instanceID) {
		return
	}
	app := NewApp()

	// With a tray to come back from, closing the window keeps PALT running,
	// and the login entry starts it straight in the tray.
	app.trayEnabled = trayAvailable()
	startHidden := app.trayEnabled && slices.Contains(os.Args[1:], startHiddenFlag)

	err := wails.Run(&options.App{
		Title:     "PALT — Local File Transfer",
		Width:     1100,
		Height:    720,
		MinWidth:  800,
		MinHeight: 560,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		// Deep ink — matches Beacon background.default in theme.ts
		BackgroundColour:  &options.RGBA{R: 12, G: 18, B: 32, A: 1},
		OnStartup:         app.startup,
		OnShutdown:        app.shutdown,
		StartHidden:       startHidden,
		HideWindowOnClose: app.trayEnabled,
		// Launching PALT again (or from the menu while it sits in the tray)
		// brings up the running window instead of fighting it for the port.
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               instanceID,
			OnSecondInstanceLaunch: func(options.SecondInstanceData) { app.showWindow() },
		},
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop: true,
		},
		Bind: []interface{}{
			app,
		},
		Linux: &linux.Options{
			WindowIsTranslucent: false,
			WebviewGpuPolicy:    linux.WebviewGpuPolicyOnDemand,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
