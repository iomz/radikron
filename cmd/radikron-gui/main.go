package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	defaultWindowWidth  = 1200
	defaultWindowHeight = 800
	backgroundR         = 27
	backgroundG         = 38
	backgroundB         = 54
	backgroundA         = uint8(255 / 2) // 50% opacity (127)
)

//go:embed all:frontend/dist
var assets embed.FS

// main is the entry point for the Wails application
func main() {
	// Create an instance of the app structure
	app := NewApp()

	wailsApp := application.New(application.Options{
		Name:        "Radikron",
		Description: "GUI for radikron - automated radio program downloader for radiko",
		Services: []application.Service{
			application.NewService(app),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})
	app.attach(wailsApp)

	wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "Radikron",
		Width:            defaultWindowWidth,
		Height:           defaultWindowHeight,
		BackgroundColour: application.NewRGBA(backgroundR, backgroundG, backgroundB, backgroundA),
		URL:              "/",
	})

	if err := wailsApp.Run(); err != nil {
		log.Fatalf("failed to start Wails application: %v", err)
	}
}
