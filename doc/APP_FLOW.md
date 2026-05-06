# AI Tutor App Flow

## Daily Study Flow

```
App Launch
    │
    ▼
Dashboard loads daily agenda (orchestrator.GetDailyAgenda)
    │
    ├─► Review tasks (Priority 1)
    │    └─► Due flashcards, quiz questions, written assessments
    │
    └─► Reading tasks (Priority 2)
         └─► Active notebooks with page ranges
    │
    ▼
User selects a task
    │
    ▼
─────────────────────────────────────────────────────
REVIEW TASK FLOW
─────────────────────────────────────────────────────
    │
    ├─► Flashcard review
    │    └─► Show card → User rates (Again/Hard/Good/Easy)
    │         └─► Update FSRS state in SQLite
    │
    ├─► Quiz review
    │    └─► Show question → User answers
    │         └─► Score answer → Update FSRS state
    │
    └─► Written assessment
         └─► Show prompt → User writes answer
              └─► LLM scores answer → Update FSRS state
    │
    ▼
─────────────────────────────────────────────────────
READING TASK FLOW
─────────────────────────────────────────────────────
    │
    ▼
Reader displays notebook content (page range)
    │
    ▼
User reads content
    │
    ├─► Ask AI (optional)
    │    └─► RAG pipeline retrieves relevant chunks
    │         └─► LLM generates contextual answer
    │
    ▼
User completes reading session
    │
    ▼
─────────────────────────────────────────────────────
MARATHON MODE (Optional)
─────────────────────────────────────────────────────
    │
    ▼
Generate quiz for page range (study.GenerateMarathonQuiz)
    │
    ▼
Generate flashcards for page range (study.GenerateMarathonFlashcards)
    │
    ▼
Generate comprehensive exam (study.GenerateComprehensiveExam)
    │
    ▼
User completes assessments
    │
    ┼─► Quiz answers scored → FSRS updated
    ┼─► Flashcard reviews → FSRS updated
    └─► Written answers scored → FSRS updated
    │
    ▼
Back to Dashboard
```

## Notebook Upload Flow

```
User uploads PDF
    │
    ▼
notebook.Service processes file
    │
    ├─► Extract text from PDF
    ├─► Parse into topics and sections
    ├─► Create chunks with page boundaries
    └─► Store in SQLite
    │
    ▼
Generate embeddings (if AI runtime available)
    │
    ├─► ONNX embedder processes chunks
    └─► Store vectors in sqlite-vec
    │
    ▼
Notebook available for reading and assessment
```

## Context Window Strategy

LLM calls via `internal/llm` provider include:

| Layer | Content | Token Budget |
|---|---|---|
| System prompt | Task-specific instructions | Fixed |
| Context chunks | Retrieved chunks (RAG) or page-bounded content | ~8000 tokens max |
| Task instruction | Generate quiz/flashcards/score answer | Fixed |

## Error States

| Trigger | Behaviour |
|---|---|
| AI runtime not ready | Show error in Ask AI, disable AI features |
| No chunks found for page range | Return empty result for marathon mode |
| LLM generation fails | Return error message to user |
| Database error | Return error message to user |

