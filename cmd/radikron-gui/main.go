package main

import (
	"embed"
	"fmt"
	"log"
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	defaultWindowWidth  = 1200
	defaultWindowHeight = 800
	// The dark "Tuner" background, shown until the page paints.
	backgroundR = 15
	backgroundG = 17
	backgroundB = 21
	backgroundA = 255
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

	updatesEnabled, err := setupUpdater(wailsApp)
	if err != nil {
		wailsApp.Logger.Error(fmt.Sprintf("Updates are disabled: %v", err))
	}
	app.updatesEnabled = updatesEnabled
	if updatesEnabled && runtime.GOOS == "darwin" {
		wailsApp.Menu.SetApplicationMenu(applicationMenu(app))
	}

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

// applicationMenu returns the default macOS menu with "Check for Updates…"
// below "About Radikron", where macOS apps conventionally place it.
func applicationMenu(app *App) *application.Menu {
	menu := application.NewMenu()
	appMenu := menu.AddSubmenu("Radikron")
	appMenu.AddRole(application.About)
	appMenu.Add("Check for Updates…").OnClick(func(*application.Context) {
		app.CheckForUpdates()
	})
	appMenu.AddSeparator()
	appMenu.AddRole(application.ServicesMenu)
	appMenu.AddSeparator()
	appMenu.AddRole(application.Hide)
	appMenu.AddRole(application.HideOthers)
	appMenu.AddRole(application.UnHide)
	appMenu.AddSeparator()
	appMenu.AddRole(application.Quit)
	menu.AddRole(application.FileMenu)
	menu.AddRole(application.EditMenu)
	menu.AddRole(application.ViewMenu)
	menu.AddRole(application.WindowMenu)
	menu.AddRole(application.HelpMenu)
	return menu
}
