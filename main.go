// Phoenix AI: run open-source AI models on your own PC.
package main

import (
	"context"
	"embed"
	"log"
	"os"
	"path/filepath"

	"phoenixai/backend"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := backend.NewApp()
	var appCtx context.Context
	var startErr error

	// Keep the window's own browser data with the rest of Phoenix's files.
	webviewData := ""
	if base := os.Getenv("LOCALAPPDATA"); base != "" {
		webviewData = filepath.Join(base, "PhoenixAI", "webview")
	}

	err := wails.Run(&options.App{
		Title:            "Phoenix AI",
		Width:            1240,
		Height:           820,
		MinWidth:         880,
		MinHeight:        600,
		BackgroundColour: &options.RGBA{R: 29, G: 16, B: 18, A: 255},
		AssetServer:      &assetserver.Options{Assets: assets},
		OnStartup: func(ctx context.Context) {
			appCtx = ctx
			backend.SetQuit(app, func() { runtime.Quit(ctx) })
			startErr = backend.Start(app, ctx, func(event string, data any) {
				runtime.EventsEmit(ctx, event, data)
			})
			if startErr != nil {
				runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
					Type:    runtime.ErrorDialog,
					Title:   "Phoenix AI couldn't start",
					Message: startErr.Error(),
				})
				runtime.Quit(ctx)
			}
		},
		OnShutdown: func(ctx context.Context) {
			if startErr == nil {
				backend.Stop(app)
			}
		},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "phoenix-ai-6f1d2c9e",
			OnSecondInstanceLaunch: func(data options.SecondInstanceData) {
				// Opening Phoenix again brings the existing window forward.
				if appCtx != nil {
					runtime.WindowUnminimise(appCtx)
					runtime.WindowShow(appCtx)
				}
			},
		},
		Bind: []interface{}{app},
		Windows: &windows.Options{
			Theme:                windows.SystemDefault,
			WebviewUserDataPath:  webviewData,
			IsZoomControlEnabled: false,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
