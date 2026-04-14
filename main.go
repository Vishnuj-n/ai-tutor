package main

import (
	"embed"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

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
		if req.Method != http.MethodGet {
			rw.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if !strings.HasPrefix(req.URL.Path, "/notebooks/") {
			http.NotFound(rw, req)
			return
		}

		if app == nil || strings.TrimSpace(app.notebookUploadDir) == "" {
			http.Error(rw, "notebook directory unavailable", http.StatusServiceUnavailable)
			return
		}

		escapedName := strings.TrimPrefix(req.URL.Path, "/notebooks/")
		fileName, err := url.PathUnescape(escapedName)
		// Explicitly reject path traversal attempts: empty, current dir, parent dir, or multi-component paths
		if err != nil || fileName == "" || fileName == "." || fileName == ".." || filepath.Base(fileName) != fileName {
			http.NotFound(rw, req)
			return
		}

		filePath := filepath.Join(app.notebookUploadDir, fileName)
		info, err := os.Stat(filePath)
		if err != nil || info.IsDir() {
			http.NotFound(rw, req)
			return
		}

		http.ServeFile(rw, req, filePath)
	})
}
