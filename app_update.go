package main

import (
	"io"
	"net/http"
	"strings"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// AppVersion is the compile-time constant for the current app version.
const AppVersion = "1.0.0"

// CheckForUpdates checks the remote version file and returns if an update is available.
func (a *App) CheckForUpdates() map[string]interface{} {
	// ponytail: simple HTTP GET to check raw text version, minimal overhead
	client := http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := client.Get("https://raw.githubusercontent.com/Vishnuj-n/ai-tutor/main/latest_version.txt")
	if err != nil {
		return map[string]interface{}{
			"update_available": false,
			"error":            err.Error(),
		}
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return map[string]interface{}{
			"update_available": false,
			"error":            "unexpected status code: " + resp.Status,
		}
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return map[string]interface{}{
			"update_available": false,
			"error":            err.Error(),
		}
	}

	remoteVersion := strings.TrimSpace(string(bodyBytes))
	remoteVersionClean := strings.TrimPrefix(remoteVersion, "v")
	currentVersionClean := strings.TrimPrefix(AppVersion, "v")

	// ponytail: simple string comparison. Since we are doing sequential releases,
	// if the remote tag differs from current, we flag update available.
	if remoteVersionClean != "" && remoteVersionClean != currentVersionClean {
		return map[string]interface{}{
			"update_available": true,
			"latest_version":   remoteVersion,
			"current_version":  AppVersion,
			"url":              "https://github.com/Vishnuj-n/ai-tutor",
		}
	}

	return map[string]interface{}{
		"update_available": false,
		"latest_version":   remoteVersion,
		"current_version":  AppVersion,
	}
}

// OpenRepoURL opens the GitHub repository releases page in the user's default system browser.
func (a *App) OpenRepoURL() {
	// ponytail: use native OS browser via Wails runtime wrapper
	wailsruntime.BrowserOpenURL(a.ctx, "https://github.com/Vishnuj-n/ai-tutor")
}
