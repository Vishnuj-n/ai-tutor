# Sprint 2 Implementation - Session Progress

## Completed Tasks

### Phase 1: Data Models & Database ✅
- Created `db.go` with:
  - SQLite initialization and table creation
  - Topic, parent sections, chunks schema
  - Hardcoded seed data (OS Scheduling topic with 2 sections)
  - Query functions: GetTopicContent, GetChunksForTopic, GetParentSection

### Phase 2: Dependencies ✅
- Added mattn/go-sqlite3 to go.mod

### Phase 3: Embeddings & Retrieval ✅  
- Created `embeddings.go` with:
  - TF-IDF vector generation for chunks
  - Tokenization with stop words filter
  - Cosine similarity search
  - Parent-document expansion
  - RetrievalContext structure

### Phase 4: RAG Pipeline ✅
- Created `rag.go` with:
  - LLMProvider for OpenAI-compatible APIs
  - RAGPipeline orchestration
  - Prompt assembly with topic context
  - Error handling for missing content
  - RAGResponse structure with citations

### Phase 5: Backend Integration ✅
- Updated `app.go` with:
  - Database initialization
  - Embeddings loading for all topics
  - LLM provider setup
  - AskAI method exposed to frontend
  - GetTopicContent and GetAvailableTopics methods
  - Support for env var config (LLM_BASE_URL, LLM_API_KEY, LLM_MODEL)

### Phase 6: Frontend UI ✅
- Updated `Reader.vue` with:
  - Content loading and display
  - Ask AI textarea input
  - Response display with citations
  - Loading states
  - Error handling
  - Wails method calling pattern
  - Responsive layout

## Current State

The MVP implementation is complete. To test:

1. Build: `wails build`
2. Dev: `wails dev`
3. Open Reader page
4. Topic content loads from database
5. Type a question and click Ask
6. RAG pipeline retrieves relevant sections and calls LLM
7. Response displays with cited sections

## Known Limitations (Sprint 2)

- LLM must be online (env vars: LLM_BASE_URL, LLM_API_KEY, LLM_MODEL)
- Only 1 hardcoded topic (os-scheduling)
- Simple TF-IDF embedding (no neural network)
- No persistent settings storage yet
- No quiz generation yet
- No FSRS cards yet
- Database in temp folder (will reset on system restart)

## Next Steps (Future Sprints)

1. **Settings UI**: Allow user to configure LLM settings in app
2. **Persistent Config**: Store settings in app data folder
3. **Add More Topics**: Implement topic upload/parsing
4. **Quiz Generation**: Implement quiz generation from content
5. **FSRS Integration**: Add flashcard scheduling
6. **Neural Embeddings**: Replace TF-IDF with chromem-go or similar
7. **Sync**: Implement backend sync
