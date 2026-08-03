package main

import (
	"embed"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"ai-tutor/internal/utils"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Create an instance of the app structure
	app := NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:            "Studyloop",
		Width:            1024,
		Height:           768,
		WindowStartState: options.Maximised,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "8f4e2a1b-9c3d-4e5f-b6a7-1c2d3e4f5a6b",
			OnSecondInstanceLaunch: func(secondInstanceData options.SecondInstanceData) {
				// ponytail: focus existing window if user opens executable again
				if app.ctx != nil {
					wailsruntime.WindowUnminimise(app.ctx)
					wailsruntime.Show(app.ctx)
				}
			},
		},
		AssetServer: &assetserver.Options{
			Assets:  assets,
			Handler: notebookHandler(app),
		},
		BackgroundColour: &options.RGBA{R: 249, G: 249, B: 251, A: 255},
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
			utils.Warnf("[notebookHandler] Service unavailable: app is nil or upload dir empty")
			http.Error(rw, "notebook directory unavailable", http.StatusServiceUnavailable)
			return
		}

		// Only handle requests under /notebooks/
		if !strings.HasPrefix(req.URL.Path, "/notebooks/") {
			return
		}

		utils.Debugf("[notebookHandler] Top of handler: path=%s, dir=%s", req.URL.Path, app.notebookUploadDir)

		// Serve only GET requests.
		if req.Method != http.MethodGet {
			utils.Warnf("[notebookHandler] Rejected method: %s", req.Method)
			rw.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		// Unescape the URL path to handle URL-encoded characters (like spaces %20)
		unescapedPath, err := url.PathUnescape(req.URL.Path)
		if err != nil {
			utils.Warnf("[notebookHandler] Error unescaping path: %s", err.Error())
			http.Error(rw, "invalid URL path encoding", http.StatusBadRequest)
			return
		}
		utils.Debugf("[notebookHandler] Unescaped path: %s", unescapedPath)

		// Clean the path and prevent directory traversal
		relPath := strings.TrimPrefix(unescapedPath, "/notebooks/")
		relPath = filepath.Clean("/" + relPath)
		
		uploadDirClean := filepath.Clean(app.notebookUploadDir)
		fullPath := filepath.Clean(filepath.Join(uploadDirClean, relPath))
		utils.Debugf("[notebookHandler] uploadDirClean: %s, fullPath: %s", uploadDirClean, fullPath)

		if !strings.HasPrefix(fullPath, uploadDirClean) {
			utils.Warnf("[notebookHandler] Access denied. Prefix check failed.")
			http.Error(rw, "access denied", http.StatusForbidden)
			return
		}

		// Verify the file actually exists on disk to prevent Wails SPA html fallback
		info, err := os.Stat(fullPath)
		if err != nil || info.IsDir() {
			utils.Warnf("[notebookHandler] File not found or is dir. Stat err: %v", err)
			http.Error(rw, "file not found", http.StatusNotFound)
			return
		}

		utils.Debugf("[notebookHandler] Serving file: %s", fullPath)
		// Serve the file directly using http.ServeFile which handles HTTP Range headers correctly
		http.ServeFile(rw, req, fullPath)
	})
}
