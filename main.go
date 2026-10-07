package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

// version is stamped at release time with -ldflags "-X main.version=vX.Y.Z"; local builds stay "dev".
var version = "dev"

func main() {
	profiles, err := NewProfileService()
	if err != nil {
		log.Fatal(err)
	}

	app := application.New(application.Options{
		Name:        "Utilidades XORCOM",
		Description: "Herramientas de configuración para Xorcom CompletePBX 5",
		Services: []application.Service{
			// Tool registry (Go side): one service per form.
			application.NewService(profiles),
			application.NewService(&PinService{profiles: profiles}),
			application.NewService(&UpdateService{}),
		},
		Assets: application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "Utilidades XORCOM",
		Width:            1240,
		Height:           700,
		MinWidth:         1024,
		MinHeight:        680,
		BackgroundColour: application.NewRGB(0xfb, 0xfc, 0xfd),
		URL:              "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
