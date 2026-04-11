Creating a dedicated roadmap file is a smart way to document your "Windows-First" decision. It keeps your current tasks focused while letting anyone else (or your future self) know that you haven't forgotten about other platforms.

I recommend creating this as `doc/PLATFORM_SUPPORT.md`. 

### Proposed Content for `doc/PLATFORM_SUPPORT.md`

---

## 🗺️ Platform Support Roadmap

### **Current Status: Windows (Primary Target)**
The AI Tutor is currently optimized and tested exclusively for **Windows 10/11 (x64)**. This decision was made to stabilize the core RAG (Retrieval-Augmented Generation) pipeline and "Auto-Syllabus" logic without the overhead of managing cross-platform native binaries during the MVP phase.

**Current Windows-Specific Dependencies:**
* **ONNX Runtime**: Utilizes `onnxruntime.dll` for local embedding inference.
* **Vector Storage**: Utilizes `vec0.dll` for SQLite vector search capabilities.
* **Build System**: Scripts and environment variables are currently tailored for CMD/PowerShell and MSVC/MinGW toolchains.

---

### **Planned Support: macOS & Linux**
While not currently supported, the architecture has been designed with future portability in mind. The following steps are planned for future releases:

* **macOS (Intel/Apple Silicon)**: 
    * Integration of `libonnxruntime.dylib`.
    * Compilation of `vec0.dylib` for Darwin.
    * Handling of macOS-specific app-data directories in `app.go`.
* **Linux (x64/ARM64)**: 
    * Integration of `libonnxruntime.so`.
    * Compilation of `vec0.so`.
    * Validation of local CGO build requirements for various distributions.

---

### **Why the Delay?**
Local-first AI requires tightly coupled native libraries. Focusing on a single OS allowed for:
1.  **Faster Feature Rollout**: Completing the automated textbook parsing and indexing logic.
2.  **Deterministic Testing**: Ensuring the ONNX-to-SQLite pipeline is mathematically stable before dealing with OS-specific memory or driver issues.
3.  **Simplified Asset Management**: Using a single `sync-deps.sh` flow for Windows-only binaries.

---

### Implementation Note
By adding this file, you can now safely remove the half-finished `switch runtime.GOOS` blocks in `internal/embeddings/onnx.go` and the generic library name guessing in `internal/runtime/assets.go`. It turns a "missing feature" into a "documented plan."
