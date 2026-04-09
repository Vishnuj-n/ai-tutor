# AI Tutor Project Structure

## Purpose

This structure keeps core logic explicit and easy to debug while the product grows into a unified scheduler-driven tutor.

## Backend (Go + Wails)

Top-level backend files are split by concern under package main:

- main.go
  - Wails app bootstrap and bindings only.
- app.go
  - Wails-facing methods and startup orchestration only.
- db.go
  - SQLite schema + low-level data access helpers.
- models.go
  - Shared domain models (TodayPlan, ScheduledTask, TopicSummary).
- scheduler.go
  - Daily planner logic across review, reading, quiz, and socratic tasks.
- embeddings.go
  - Retrieval embedding store and similarity search.
- rag.go
  - RAG pipeline orchestration.
- llm.go
  - Stateless OpenAI-compatible provider client + prompt assembly.

Debugging rule:
- If UI data is wrong, inspect app.go method first, then the matching service file (scheduler.go/rag.go), then db.go queries.

## Frontend (Vue)

- frontend/src/router
  - Route definitions and page mapping.
- frontend/src/components
  - Reusable shell components like Sidebar.
- frontend/src/pages
  - Page-level features (Dashboard, Reader, Quiz, Flashcards, Socratic, Settings).
- frontend/src/services
  - Backend bridge wrappers. Pages call services instead of raw window.go calls.

Debugging rule:
- Keep all backend calls in services files so network/bridge failures are isolated.

## Scheduler Contract (V1)

Dashboard consumes one payload from GetTodayPlan:

- date
- total_minutes
- review_minutes
- learning_minutes
- due_review_cards
- active_topics
- tasks[]

Task shape:

- id
- action_type (review, read, quiz, socratic, explore)
- title
- topic_id (optional)
- estimate_minutes
- priority
- meta

This contract is intentionally broad so future task types can be added without rewriting the dashboard page layout.
