// Hytale Launcher - Reverse Engineered
// This is the main entry point for the Wails application.
package main

import (
	"embed"
	"log/slog"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"hytale-launcher/internal/app"
	"hytale-launcher/internal/build"
	"hytale-launcher/internal/logging"
)

//go:embed frontend/dist
var assets embed.FS

func main() {
	// Initialize logging
	logging.Init()

	slog.Info("starting Hytale Launcher",
		"version", build.Version,
		"release", build.Release,
		"platform", build.OS(),
		"arch", build.Arch(),
	)

	// Create the application instance
	application := app.New()

	// Run the Wails application
	err := wails.Run(&options.App{
		Title:     "Hytale Launcher",
		Width:     1280,
		Height:    800,
		MinWidth:  1024,
		MinHeight: 700,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        application.Startup,
		OnDomReady:       application.DomReady,
		Bind: []interface{}{
			application,
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:    false,
		},
		Mac: &mac.Options{
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: true,
				HideTitle:                  false,
				HideTitleBar:               false,
				FullSizeContent:            true,
				UseToolbar:                 false,
				HideToolbarSeparator:       true,
			},
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
		},
		Linux: &linux.Options{
			ProgramName: "Hytale Launcher",
		},
	})

	if err != nil {
		slog.Error("application error", "error", err)
		os.Exit(1)
	}
}
