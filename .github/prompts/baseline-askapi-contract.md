# Phase 1: Baseline AskAI API Contract and Behavior

## Current API Signature 
**File**: `app.go` line 123+

```go
func (a *App) AskAI(topicID string, question string) map[string]interface{}
```

**Frontend bridge**: `frontend/src/services/appApi.js`  
```javascript
askAI(topicID, question) -> Promise<Response>
```

## Response Contract (Wails Return)
**File**: `app.go` AskAI method

**Success response** (map[string]interface{}):
```json
{
  "answer": "string",                      // the LLM-generated answer text
  "cited_sections": ["string", "string"],  // array of cited parent section headings
  "chunks_retrieved": 5,                   // number of chunks matched via lexical search
  "sections_used": 2,                      // number of unique parent sections expanded from chunks
  "error": null or absent                  // no error field on success
}
```

**Error response** (map[string]interface{}):
```json
{
  "error": "string"  // error message
}
```

## Current Implementation Flow (Lexical TF-IDF)
**File**: `internal/rag/pipeline.go` ProcessQuery() lines 35-108

1. **Validate topic**: `db.GetTopicContent(topicID)` 
   - Error: "topic not found: {msg}"
2. **Fetch all chunks**: `db.GetChunksForTopic(topicID)`
   - Error: "could not retrieve chunks: {msg}"
   - Empty result error: "no content available for this topic"
3. **Search top-k**: `embedStore.SearchTopK(question, chunks, 5)` (hardcoded k=5)
   - Uses in-memory TF-IDF vectors computed at startup by `EmbeddingStore.TFVector()`
   - Lexical tokenization: hardcoded English stopwords + regex split
   - No ONNX model, no Hugging Face tokenizer
   - Error: "no relevant content found for your question" (if results empty)
4. **Heuristic scoring**: `ApplyHeuristicScoring(results)` 
   - Currently a no-op pass-through
5. **Parent expansion**: `BuildContext(results, topicID)`
   - Calls `db.GetParentSection(parentID)` once per unique parent
   - Returns map of parent heading + full content text
   - Error: "could not build context: {msg}"
6. **Prompt assembly**: `buildPrompt(title, question, context)`
   - Concatenates title, sections, question into single prompt
7. **LLM call**: `llm.Provider.GenerateAnswer(prompt)`
   - OpenAI-compatible API via env vars: LLM_BASE_URL, LLM_API_KEY, LLM_MODEL
   - Error: "LLM error: {msg}"
8. **Response assembly**: Extract cited headings from context sections
   - Parse "**Heading**" markdown format
   - Return with chunk/section counts

## Embedding Store Current State (in-memory)
**File**: `internal/rag/embeddings.go`

- **Instance**: `EmbeddingStore.vectors map[string]VectorEntry`
- **Lifecycle**: Populated at startup in `app.startup()` by iterating chunks and calling `AddChunk()`
- **Vector type**: `map[string]float64` (sparse term-frequency vector)
- **Persistence**: None; lost on app restart
- **Dimension**: Variable (number of unique terms in vocabulary)
- **Reuse**: In-memory, fast lookup but no DB persistence

## Current Startup Hardcoding
**File**: `app.go` startup() lines 61-72

```go
topicIDs := []string{"os-scheduling"}  // hardcoded topic

for _, topicID := range topicIDs {
    chunks, err := db.GetChunksForTopic(topicID)
    ...
    for _, chunk := range chunks {
        embedStore.AddChunk(chunk)  // TF-IDF only
    }
}
```

- Only topic "os-scheduling" is indexed
- All chunks must fit in memory
- App restart loses all vectors

## Frontend Usage
**File**: `frontend/src/pages/Reader.vue`

- Calls `askAI(topicID, question)` via text input + button
- Displays response.answer in markdown
- Shows response.cited_sections as citations
- Shows response.chunks_retrieved and response.sections_used as metadata
- Handles response.error with explicit UI message

## Constraints to Preserve (Non-Negotiable)
1. **Request signature unchanged**: `AskAI(topicID, question)` stays identical
2. **Response shape unchanged**: Same JSON keys returned
3. **Topic scope**: Only retrieve from active topicID
4. **Parent expansion**: Child chunks → parent sections before prompting
5. **Deterministic top-k**: Same query + context should produce same retrieval order
6. **Topic-only retrieval**: Never cross topic boundaries without explicit scope toggle
7. **Stateless LLM call**: One request, no conversation memory
8. **Explicit error handling**: Fail clearly for missing topics, no retrieval, API unavailable

## Regression Tests to Add
1. `TestAskAIResponseShape()`: Verify response has all required keys (answer, cited_sections, chunks_retrieved, sections_used)
2. `TestAskAITopicScope()`: Verify only chunks from active topicID returned
3. `TestAskAIParentExpansion()`: Verify parent sections are returned in cited_sections
4. `TestAskAIErrorHandling()`: Test invalid topicID, no retrieval results, LLM error
5. `TestAskAIFrontendContract()`: Call via appApi.js bridge, verify promise resolves with correct shape
6. `TestAskAIDeterminism()`: Same query twice returns identical cited_sections and chunks_retrieved count

## Migration Boundary
- **Backend replacement**: Swap TF-IDF → ONNX, in-memory → sqlite-vec
- **API contract**: Unchanged (Wails signature + response shape identical)
- **Frontend code**: Zero changes required
- **Error messages**: May evolve (e.g., "sqlite-vec unavailable") but same error key structure
- **Top-k result count**: Should remain ~5 chunks (or be made configurable, reviewed in Phase 6)
