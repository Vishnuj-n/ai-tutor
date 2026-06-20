package main

import (
	"embed"
	"net/http"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Create an instance of the app structure
	app := NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:  "ai-tutor",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets:  assets,
			Handler: notebookHandler(app),
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

func notebookHandler(app *App) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if app == nil || app.notebookUploadDir == "" {
			http.Error(rw, "notebook directory unavailable", http.StatusServiceUnavailable)
			return
		}

		// Serve only GET requests under /notebooks/.
		// http.FileServer handles path cleaning, URL unescaping, and directory
		// traversal prevention automatically; no manual path manipulation needed.
		if req.Method != http.MethodGet {
			rw.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		fs := http.FileServer(http.Dir(app.notebookUploadDir))
		http.StripPrefix("/notebooks", fs).ServeHTTP(rw, req)
	})
}
