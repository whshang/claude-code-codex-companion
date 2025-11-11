package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Create an instance of the app structure
	app := NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:            "cccc-proxy",
		Width:            1024,
		Height:           768,
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		Mac: &mac.Options{
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: true,
				HideTitle:                  true,  // 隐藏标题文字
				HideTitleBar:               false, // 保留标题栏（保留红绿灯按钮）
				FullSizeContent:            false, // 不扩展到红绿灯区域，避免拖拽冲突
				UseToolbar:                 false,
				HideToolbarSeparator:       true,
			},
		},
				OnStartup:  app.startup,
		OnDomReady: app.OnDomReady,
		OnBeforeClose: app.OnBeforeClose,
		OnShutdown: app.OnShutdown,
		Bind: []interface{}{
			app,
		},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}