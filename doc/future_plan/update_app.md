Here is the standalone `future_solutions.md` sub-module dedicated entirely to your YAGNI-compliant app update strategy. You can drop this directly into your documentation structure.

---

# 🚀 Future Solutions — System Infrastructure: YAGNI-Compliant App Updates

To avoid the massive architectural overhead of writing in-app binary patchers, managing file system process locks, or risking strict Windows Defender antivirus false-positives, the application utilizes an evolutionary update deployment pipeline.

This model shifts the heavy lifting of distribution out of the application codebase and handles it through progressive, lightweight stages.

---

## The 3-Phase Deployment Strategy

### Phase 1: The Lazy Notification Pattern (v1.0.x Release Baseline)

This approach completely eliminates internal binary file manipulation, making it 100% safe for early testing.

1. **The Remote Indicator:** A single, raw static text asset (`latest_version.txt`) containing the newest version tag string (e.g., `1.1.0`) is hosted publicly on GitHub Pages or a public release bucket.
2. **The Asynchronous Check:** On application boot, the Go backend spins up a light, isolated goroutine that executes a simple `http.Get` request against the text asset.
3. **The Local Comparison:** The string is read and matched against a local compile-time constant:
```go
const AppVersion = "1.0.0"

```


4. **The Safe Hand-off:** If the remote version string is greater than the local constant, the backend emits a `wails.EventsEmit` notification to the Vue frontend. The UI presents an elegant top-level dashboard banner:
> 🚀 **Version 1.1.0 is now available!** Click here to open the download page in your browser. `[ Get Update ]`


5. **The Execution:** Clicking the button calls Wails' internal shell runner to safely open the GitHub Releases URL in the user's default system browser, letting them download the new zipped executable directly.
```go
runtime.BrowserOpenURL(ctx, "https://github.com/yourusername/ai-tutor/releases")

```



---

### Phase 2: App-Store Manifest Integration (v2.0.0 Release Migration)

Once the software achieves a stable baseline, we entirely deprecate internal notification code and transfer binary lifecycle management to the operating system level.

* **Target Delivery:** The application binary is wrapped and published directly to **Windows Package Manager (WinGet)** manifests and the Microsoft Store.
* **Mechanism:** When a new release tag is finalized, an automated GitHub Action updates the WinGet repository manifest.
* **System Execution:** The native Windows Update system agents detect the updated manifest hash. Windows handles background delivery, digital signature verification, and localized file replacement silently when the app is idle.
* **Engineering Win:** This achieves silent background auto-updates for our desktop environment with **exactly zero lines of complex patching code** maintained inside our custom Go service layers.

---

### Phase 3: Silent Background Hot-Swap (Enterprise/Scale Option Only)

This architectural model is deferred indefinitely unless enterprise or multi-tenant desktop distribution cycles genuinely demand it.

```
[Background Polling Thread] ──► (Finds New Version)
            │
            ▼
[Download to OS %TEMP% Folder] ──► (Verify SHA256 Signature)
            │
            ▼
[Wails Graceful Shutdown] ──► (Call db.Close() / Release SQLite Locks)
            │
            ▼
[Spawn Detached Windows Process] ──► (Executes Silent Installer & Relaunches Main Executable)

```

* **The Operation:** The running app downloads the new compiled `.msi` target into a temporary directory, waits until the student closes their active study session to gracefully close SQLite connection pools, kills its own master PID, and lets a detached background process hot-swap the primary executable.
* **The Complexity Warning:** This phase is marked as low-priority/YAGNI because handling Windows administration permissions, locked execution rings, and network interruptions during raw binary file-swapping adds unnecessary instability to the core study metrics loop.