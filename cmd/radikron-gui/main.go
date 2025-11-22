package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
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

	// Create application with options
	err := wails.Run(&options.App{
		Title:  "Radikron",
		Width:  defaultWindowWidth,
		Height: defaultWindowHeight,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: backgroundR, G: backgroundG, B: backgroundB, A: backgroundA},
		OnStartup:        app.OnStartup,
		OnShutdown:       app.OnShutdown,
		Bind: []any{
			app,
		},
	})

	if err != nil {
		log.Fatal("Error:", err)
	}
}
