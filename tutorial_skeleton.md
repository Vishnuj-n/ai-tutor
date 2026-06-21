# AI Tutor: New Developer Tutorial Skeleton

## 1. The App Layer (`app.go`)

### Core Purpose
The `App` struct is the **Wails binding layer** — it's the only code the frontend can call directly. While the file comment claims "no business logic," it actually contains significant **orchestration logic** (~544 lines) including RAG initialization, asset management coordination, and state gating. This is the reality of an MVP-scale app.

### Annotated Code Snippets
```go
// app.go - Actual state and initialization
type App struct {
    ctx               context.Context
    repo              *db.Repository
    embedder          *embeddings.OnnxEmbedder
    retrievalEngine   *retrieval.Engine
    fastLLMProvider   llmProviderInterface
    heavyLLMProvider  llmProviderInterface
    scheduler         scheduler.Service
    studyService      *study.StudyService
    aiReady           bool      // Gated behind asset checks
    aiInitError       string    // Surface initialization failures
    indexQueue        *retrieval.VectorIndexQueue
}

// startup - Called by Wails on app launch
func (a *App) startup(ctx context.Context) {
    // 1. runtime.Bootstrap() - loads DB, embedder, services
    // 2. Copies boot results into App fields
    // 3. Starts VectorIndexQueue for background indexing
}

// InitializeRAG - Async RAG setup with progress events
func (a *App) InitializeRAG() map[string]interface{} {
    // Launches goroutine that:
    // 1. Downloads/acquires ONNX assets
    // 2. Stages vec0.dll extension
    // 3. Re-initializes DB with vector support
    // 4. Indexes all existing topics
    // 5. Emits progress events to frontend
}
```

### Step-by-Step Execution Flow
1. **App Launch**: `main.go` calls `NewApp()` then Wails calls `app.startup()`.
2. **Bootstrap**: `runtime.Bootstrap()` initializes DB, embedder, scheduler, and study services.
3. **Frontend Ready**: Vue app mounts and can now call Wails-bound methods.
4. **RAG Request**: If user enables RAG, frontend calls `InitializeRAG()`.
5. **Async Setup**: `InitializeRAG` spawns a goroutine, emits progress events, and sets `aiReady = true` on success.
6. **Gated Access**: Subsequent AI calls (e.g., `AskReaderAI`) check `aiReady` before proceeding.

---

## 2. The Study Queue (`study_queue`)

### Core Purpose
The `study_queue` is the central state machine of the application. It replaces complex orchestration engines with a simple, SQL-queryable table that determines the user's next action.

### Annotated Code Snippets
```sql
-- doc/SCHEMA.md
CREATE TABLE study_queue (
    id TEXT PRIMARY KEY,
    notebook_id TEXT NOT NULL,
    task_type TEXT NOT NULL,  -- READING, QUIZ, REREAD, FLASHCARD_REVIEW
    status TEXT NOT NULL,     -- PENDING, ACTIVE, COMPLETED, FAILED
    priority INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### Step-by-Step Execution Flow
1. **Task Insertion**: When a notebook is uploaded or a quiz is failed, a new row is inserted into `study_queue`.
2. **Ordering**: The scheduler queries this table, ordering by task priority (e.g., `FLASHCARD_REVIEW` > `READING`).
3. **Activation**: When the user clicks "Start", the status changes from `PENDING` to `ACTIVE`.
4. **Completion**: Upon finishing the task, the status becomes `COMPLETED`, and follow-up tasks are inserted.

---

## 3. RAG & Embeddings (`internal/embeddings/`)

### Core Purpose
Provides local, offline-first AI retrieval. It converts user queries and notebook content into vectors using ONNX models, allowing the tutor to answer questions based strictly on the provided material.

### Annotated Code Snippets
```go
// internal/embeddings/onnx_embedder.go
type OnnxEmbedder struct {
    session *ort.Session // ONNX Runtime session
    // ...
}

// Converts text to a vector
func (e *OnnxEmbedder) Embed(text string) ([]float32, error) {
    // Tokenize -> Run ONNX session -> Return float vector
}
```

### Step-by-Step Execution Flow
1. **Ingestion**: Text is chunked (sliding window) and passed to `OnnxEmbedder.Embed`.
2. **Storage**: The resulting vectors are stored in SQLite using the `vec0` extension.
3. **Retrieval**: When a user asks a question, the query is embedded, and a KNN search (`k_nearest_neighbors`) finds relevant chunks.
4. **Context Injection**: These chunks are injected into an LLM prompt for the final response.

---

## 4. Scheduler Service (`internal/scheduler/`)

### Core Purpose
Implements the deterministic ordering rules for the study queue. It ensures that time-sensitive tasks (Flashcards) appear before new content (Reading).

### Annotated Code Snippets
```go
// internal/scheduler/scheduler.go
func (s *Service) GetNextTask() (*models.StudyTask, error) {
    // Executes the deterministic SQL ordering query
    // 1. FLASHCARD_REVIEW (due)
    // 2. REREAD
    // 3. QUIZ
    // 4. READING
    // 5. EXAMINER
}
```

### Step-by-Step Execution Flow
1. **Query**: The Dashboard calls `GetNextTask` on startup or after task completion.
2. **Filter**: The service filters for `status = 'PENDING'`.
3. **Sort**: It applies the fixed priority hierarchy and notebook weights.
4. **Return**: The highest-priority task object is returned to the frontend for rendering.

---

## 5. Runtime Bootstrap (`internal/runtime/`)

### Core Purpose
Ensures all dependencies (SQLite DB, ONNX models, vec0.dll) are correctly located and initialized before the application becomes interactive.

### Annotated Code Snippets
```go
// internal/runtime/boot.go
func Bootstrap(ctx context.Context) (*BootResult, error) {
    // 1. Resolve app directories (%LOCALAPPDATA%)
    // 2. Load SQLite + vec0 extension
    // 3. Initialize ONNX embedder from asset/ folder
    // 4. Return initialized services
}
```

### Step-by-Step Execution Flow
1. **Asset Check**: Checks if `model_int8.onnx` and `tokenizer.json` exist.
2. **Extension Load**: Loads `vec0.dll` into the SQLite driver.
3. **DB Init**: Creates the repository and runs any pending migrations.
4. **Service Wiring**: Connects the LLM, RAG, and Scheduler services together.
5. **Ready State**: Sets `aiReady = true` in the `App` struct, enabling frontend AI features.
