# Sprint 2 Implementation Guide

## Architecture Overview

### Data Flow
```
User Input (Reader.vue)
  → Frontend calls AskAI backend method
  → App.AskAI validates topic and question
  → RAGPipeline.ProcessQuery orchestrates:
    1. Retrieve chunks from database
    2. Search for top-k similar chunks using TF-IDF
    3. Expand chunks to parent sections
    4. Assemble prompt with context
    5. Call OpenAI-compatible LLM
  → Return response with citations
  → Frontend displays result
```

### Database Schema

**Topics**
- id: topic identifier
- title: display name
- status: reading/learned/reviewing
- created_at, updated_at: timestamps

**Parents** (section-level)
- id: parent section identifier
- topic_id: reference to topic
- heading: section title
- order_index: display order
- content_text: full section content

**Chunks** (retrieval units)
- id: chunk identifier
- topic_id: reference to topic
- parent_id: reference to parent section
- chunk_text: chunk content (used for retrieval)
- token_count: for budget planning
- embedding_ref: (reserved for future neural embeddings)

## Key Components

### Backend (Go)

**db.go**
- Initializes SQLite database
- Defines schema and seed data
- Provides query functions for topics, parents, chunks
- One hardcoded topic: "os-scheduling" with example content

**embeddings.go**
- EmbeddingStore: manages chunk embeddings and retrieval
- TFVector: creates term-frequency vectors from text
- Tokenize: breaks text into meaningful tokens (with stop-word filtering)
- CosineSimilarity: computes similarity between vectors
- SearchTopK: finds top-k most similar chunks to a query
- BuildContext: expands chunk results to parent sections

**rag.go**
- LLMProvider: interface to OpenAI-compatible APIs
- RAGPipeline: orchestrates the full retrieval + generation flow
- ProcessQuery: main entry point
- assemblePrompt: builds the final prompt for the LLM
- RAGResponse: structured output with citations

**app.go**
- Wails integration point
- Initializes all components on startup
- Exposes backend methods to frontend:
  - GetTopicContent(topicID) → retrieves section content
  - GetAvailableTopics() → returns available topics
  - AskAI(topicID, question) → main RAG entry point

### Frontend (Vue)

**Reader.vue**
- Displays topic sections
- Ask AI panel with textarea input
- Response display with cited sections
- Loading and error states
- Calls wails methods via window.go.main.App

## Configuration

### Environment Variables
```bash
LLM_BASE_URL=http://localhost:8000  # Default: http://localhost:8000
LLM_API_KEY=sk-xxx                  # Required
LLM_MODEL=gpt-3.5-turbo             # Default: gpt-3.5-turbo
```

If not set, app will prompt for configuration (future sprint).

## Testing the Implementation

### 1. Start LLM Server
```bash
# If using local Ollama
ollama serve

# If using OpenAI
export LLM_BASE_URL=https://api.openai.com
export LLM_API_KEY=sk-xxx
export LLM_MODEL=gpt-3.5-turbo
```

### 2. Build and Run
```bash
wails dev
```

### 3. Try the Flow
1. Open Reader page
2. Topic content should load (Operating Systems: Scheduling)
3. Type a question like "What is Round Robin scheduling?"
4. Click "Ask"
5. Response appears with cited sections

## Extending for Future Sprints

### Adding More Topics
1. Add entries to seedData() in db.go
2. Create parent sections and chunks
3. Topics will be auto-embedded on startup

### Quiz Generation
1. Create QuizPipeline similar to RAGPipeline
2. Add GenerateQuiz LLM method
3. Store quiz_sets in database

### FSRS Cards
1. Create FSRS scheduler in separate file
2. Query fsrs_cards table
3. Update intervals based on user response

### Settings Page
1. Create SettingsProvider to manage config
2. Store in app data directory (not temp)
3. Allow user to change LLM settings

## Troubleshooting

### "Topic not found"
- Check that topic_id matches (should be "os-scheduling")
- Verify database was initialized

### "LLM API error"
- Verify LLM server is running
- Check LLM_BASE_URL is correct
- Verify LLM_API_KEY is valid

### "No content available"
- Verify chunks exist in database
- Check topic has parent sections

### Empty response
- Likely LLM connection issue
- Check network connectivity
- Verify LLM config in startup logs
