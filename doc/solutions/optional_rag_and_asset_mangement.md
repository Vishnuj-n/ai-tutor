Walkthrough — Optional RAG, Asset Manager, and Background Vectorization
All requirements have been successfully implemented. The application now supports a production-ready AssetManager that isolated ONNX assets inside the %LOCALAPPDATA% path, provides full SHA256 integrity verification, and supports dynamic RAG toggling with simulated copy-streaming download workflows. Below is a detailed summary of the files modified and functionality verified.

Changes Implemented
1. Production-Ready Asset Management
internal/runtime/asset_manager.go [NEW]: Declared the AssetManifest schema. Resolved the target assets directory dynamically to %LOCALAPPDATA%/ai-tutor/assets/ on Windows. Implemented CheckAssets(), AcquireAssets(), and StageDLLs(). It simulates downloading by copy-streaming local ./asset files in blocks with progress callbacks and validates files against SHA256 hashes.
internal/runtime/assets.go [DELETED]: Removed the basic validation code.
2. Database Schema & Migration
internal/db/schema.go [MODIFY]: Added rag_enabled BOOLEAN DEFAULT 0 to the user_settings table. Included an alter statement in alterStatements for active migrations.
internal/db/store.go [MODIFY]: Added GetRAGEnabled() (bool, error). Modified GetUserSettings() and UpdateUserSettings() to read/write rag_enabled setting.
internal/models/models.go [MODIFY]: Added RAGEnabled field to UserSettings struct.
3. Startup Boot & Dynamic Loading
internal/runtime/boot.go [MODIFY]: Modified Bootstrap to first init the DB without vec0, fetch rag_enabled, and load ONNX embedding assets/DLLs only if RAG is enabled. Re-initializes DB with vector support dynamically when RAG is active.
app.go [MODIFY]: Exposed RAG settings dynamically. Created InitializeRAG() to run simulated acquisition in the background, emitting events to the UI. Created reloadRetrievalEngine() helper to dynamically rebuild/unload the embedder and socratic retrieval engine at runtime.
notebook_endpoints.go [MODIFY]: Added a check in ConfirmNotebookSyllabus to trigger the VectorIndexer.IndexAllTopics() run in a background goroutine when RAG is active.
4. Frontend Integration
frontend/src/services/appApi.js [MODIFY]: Integrated ragEnabled argument into settings APIs and exported the InitializeRAG() backend endpoint.
frontend/src/pages/Onboarding.vue [MODIFY]: Added Step 3: Local AI Retrieval (RAG). Displays radio selectors to Enable/Skip RAG, and animates a simulated download progress bar.
frontend/src/pages/Reader.vue [MODIFY]: Fetches RAG status. If disabled, greys out the AI Chat sidebar and overlays a friendly call-to-action lock to enable it.
frontend/src/pages/Settings.vue [MODIFY]: Added a local RAG checkbox. Shows the setup modal with progress bars if toggled on, and unloads RAG instantly from the backend if toggled off.
Verification Results
Automated Tests
Ran go test ./... in the backend root: Passed successfully. All database queries, FSRS, and lexical fallback checks passed.
Ran npm run build in the frontend root: Passed successfully. Compiled Vite bundles without warnings or bundle errors.