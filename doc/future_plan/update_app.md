# 🚀 System Infrastructure: YAGNI-Compliant App Updates

To avoid the massive architectural overhead of writing in-app binary patchers, managing file system process locks, or risking strict Windows Defender antivirus false-positives, the application utilizes a simple update notification and redirection pipeline.

This model shifts the heavy lifting of distribution out of the application codebase and handles it through progressive, lightweight stages.

---

## Current Architecture: The Notification & Redirection Pattern (v1.0.x Release Baseline)

This approach completely eliminates internal binary file manipulation, making it 100% safe and simple.

1. **The Remote Indicator:** A single, raw static text asset (`latest_version.txt`) containing the newest version tag string (e.g., `1.1.0`) is hosted publicly at:
   `https://raw.githubusercontent.com/Vishnuj-n/studyloop/main/latest_version.txt`
2. **The Local Comparison:** A compile-time constant in `app_update.go`:
   ```go
   const AppVersion = "1.0.0"
   ```
3. **The Boot Check:** On frontend mount, `App.vue` calls the backend `CheckForUpdates` method:
   - Evaluates version tags.
   - If a new version is found, displays a top-level warning dialog popup to prompt the user.
4. **The Settings Panel:** A dedicated update settings module in `Settings.vue` allows users to:
   - Trigger manual version checks.
   - See current vs latest release tags.
   - Click a redirect button to visit the repository.
5. **The Redirect:** Clicking the button calls:
   ```go
   wailsruntime.BrowserOpenURL(ctx, "https://github.com/Vishnuj-n/studyloop")
   ```
   to open the repository releases page in the user's default system browser.

---

## Future Stages

### Stage 2: App-Store Manifest Integration
Once the software achieves a stable baseline, we can transfer binary lifecycle management to the operating system level:
* Wrap and publish the binary to **Windows Package Manager (WinGet)** manifests and the Microsoft Store.
* Windows handles background delivery, digital signature verification, and localized file replacement silently when the app is idle.
* This achieves updates with **exactly zero lines of complex patching code** inside our application.

### Stage 3: Silent Background Hot-Swap (Enterprise/Scale Option Only)
This architectural model is deferred indefinitely:
* Download new compiled targets into a temporary directory, wait until SQLite connection pools close gracefully, kill the PID, and let a detached background process hot-swap the primary executable.
* Marked as low-priority/YAGNI due to high complexity and security permissions overhead.