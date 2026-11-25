package main

import (
	"log"

	desktop "github.com/JacobBananalDev/openconnect-client/internal/gui"
	"github.com/JacobBananalDev/openconnect-client/internal/ipc"
	"github.com/wailsapp/wails/v3/pkg/application"
)

func main() {
	assets, err := desktop.DistAssets()
	if err != nil {
		log.Fatalf("load embedded frontend assets: %v", err)
	}
	connectionService, err := desktop.NewConnectionService(ipc.Client{SocketPath: ipc.DefaultSocketPath})
	if err != nil {
		log.Fatalf("create connection service: %v", err)
	}

	app := application.New(application.Options{
		Name:        "OpenConnect Client",
		Description: "OpenConnect Client desktop application",
		Services: []application.Service{
			application.NewService(connectionService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "OpenConnect Client",
		Width:  1000,
		Height: 700,
		URL:    "/",
	})

	if err := app.Run(); err != nil {
		log.Fatalf("run desktop application: %v", err)
	}
}
