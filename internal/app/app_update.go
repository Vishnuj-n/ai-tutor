package app

import (
	_ "embed"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed VERSION
var embeddedVersion string

// getAppVersion retrieves current app version dynamically from the embedded VERSION file.
func getAppVersion() string {
	ver := strings.TrimSpace(embeddedVersion)
	if ver != "" {
		return strings.TrimPrefix(ver, "v")
	}
	if data, err := os.ReadFile("internal/app/VERSION"); err == nil {
		return strings.TrimPrefix(strings.TrimSpace(string(data)), "v")
	}
	if data, err := os.ReadFile("VERSION"); err == nil {
		return strings.TrimPrefix(strings.TrimSpace(string(data)), "v")
	}
	return "1.0.0"
}

// CheckForUpdates checks the remote version file and returns if an update is available.
func (a *App) CheckForUpdates() map[string]interface{} {
	appVer := getAppVersion()

	// ponytail: simple HTTP GET to check raw text version, minimal overhead
	client := http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := client.Get("https://raw.githubusercontent.com/Vishnuj-n/studyloop/main/internal/app/VERSION")
	if err == nil && resp.StatusCode == http.StatusNotFound {
		_ = resp.Body.Close()
		resp, err = client.Get("https://raw.githubusercontent.com/Vishnuj-n/studyloop/main/VERSION")
	}
	if err != nil {
		return map[string]interface{}{
			"update_available": false,
			"current_version":  appVer,
			"error":            err.Error(),
		}
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return map[string]interface{}{
			"update_available": false,
			"current_version":  appVer,
			"error":            "unexpected status code: " + resp.Status,
		}
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return map[string]interface{}{
			"update_available": false,
			"current_version":  appVer,
			"error":            err.Error(),
		}
	}

	remoteVersion := strings.TrimSpace(string(bodyBytes))
	remoteVersionClean := strings.TrimPrefix(remoteVersion, "v")
	currentVersionClean := strings.TrimPrefix(appVer, "v")

	// ponytail: simple string comparison. Since we are doing sequential releases,
	// if the remote tag differs from current, we flag update available.
	if remoteVersionClean != "" && remoteVersionClean != currentVersionClean {
		return map[string]interface{}{
			"update_available": true,
			"latest_version":   remoteVersionClean,
			"current_version":  currentVersionClean,
			"url":              "https://github.com/Vishnuj-n/studyloop/releases",
		}
	}

	return map[string]interface{}{
		"update_available": false,
		"latest_version":   remoteVersionClean,
		"current_version":  currentVersionClean,
	}
}

// OpenRepoURL opens the GitHub repository releases page in the user's default system browser.
func (a *App) OpenRepoURL() {
	// ponytail: use native OS browser via Wails runtime wrapper
	wailsruntime.BrowserOpenURL(a.ctx, "https://github.com/Vishnuj-n/studyloop/releases")
}
