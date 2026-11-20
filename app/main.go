// Command katafighter is the KataFighter desktop app (Wails v3).
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
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "KataFighter",
		Width:            1280,
		Height:           800,
		MinWidth:         800,
		MinHeight:        500,
		BackgroundColour: application.NewRGB(24, 24, 28),
		URL:              "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
