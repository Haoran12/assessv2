package main

import (
	"io/fs"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

func main() {
	frontendAssets, err := fs.Sub(embeddedRuntimeAssets, "runtime/frontend/dist")
	if err != nil {
		log.Fatalf("resolve embedded frontend assets: %v", err)
	}

	app := NewApp()

	err = wails.Run(&options.App{
		Title:     "考核系统",
		Width:     1600,
		Height:    960,
		MinWidth:  1366,
		MinHeight: 768,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               "assessv2.desktop.singleinstance",
			OnSecondInstanceLaunch: app.onSecondInstanceLaunch,
		},
		AssetServer: &assetserver.Options{
			Assets:  frontendAssets,
			Handler: app,
		},
		OnStartup:     app.startup,
		OnShutdown:    app.shutdown,
		OnBeforeClose: app.beforeClose,
		Bind:          []interface{}{app},
	})
	if err != nil {
		log.Fatal(err)
	}
}
