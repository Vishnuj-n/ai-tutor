# Agent Map

Each directory owns one responsibility.

## Internal

- db → all persistence
- scheduler → FSRS scheduling and daily planning
- orchestrator → task orchestration and agenda building
- llm → LLM provider abstraction
- study → study session management (quiz, flashcard, reading)
- notebook → notebook ingestion and management
- embeddings → ONNX-based embedding generation
- rag → RAG pipeline for Ask AI
- retrieval → retrieval engine for Socratic mode
- runtime → asset validation and runtime preparation
- models → shared data models
- subtopic → subtopic processing
- utils → shared utilities

## Rules

- No cross imports between internal modules
- DB is the only place with SQL
- Frontend never calls Go directly
- No business logic in UI

## Data Flow

UI → hook → bindings → app.go → service layer → db
