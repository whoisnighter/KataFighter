// Command katafighter is the KataFighter desktop app (Wails v3).
//
// Built normally it opens a native window. Built with -tags server it runs
// as an HTTP server with no window (see app/AGENTS.md, "Browser mode").
package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/whoisnighter/KataFighter/app/services"
)

// The built frontend (frontend/dist) is embedded into the binary.
//
//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := application.New(application.Options{
		Name:        "KataFighter",
		Description: "Character editor for Ikemen GO",
		Services: []application.Service{
			application.NewService(&services.ProjectService{}),
			application.NewService(&services.ViewerService{}),
			application.NewService(&services.EditorService{}),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	// The title bar is drawn by the frontend (menus, title, window
	// buttons), like VS Code. Frameless keeps the native shadow, rounded
	// corners and resize edges on Windows.
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "KataFighter",
		Width:            1280,
		Height:           800,
		MinWidth:         800,
		MinHeight:        500,
		Frameless:        true,
		BackgroundColour: application.NewRGB(20, 22, 33),
		URL:              "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
