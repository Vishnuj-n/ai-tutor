# AI Tutor RAG Architecture

## 1. Purpose

### What

The retrieval-augmented generation layer powers contextual AI for the current topic only.

### Why

- Keep answers grounded in the material the learner is actively studying
- Prevent cross-topic drift and conversational behavior
- Preserve predictable latency, cost, and token usage

### How

- Retrieve only from the active `topic_id`
- Expand matched child chunks to their parent sections before prompting
- Build a single-turn prompt and send one stateless LLM request

## 2. Scope and Boundaries

### What

RAG is used for contextual explanation and topic-scoped assistance, primarily in Reader and Flashcards Explain flows.

### Why

The app is a guided tutor, not a chatbot. Retrieval must support the learning flow instead of replacing it.

### How

- Allowed: ask for clarification on the active topic, explain a flashcard, summarize a section, answer content-specific questions
- Not allowed: free-form general chat, long-lived memory, cross-topic search by default, autonomous multi-step research

## 3. Retrieval Inputs

### What

The pipeline consumes:

- Active topic context
- User question or explain request
- Topic content stored as parent sections and child chunks
- Token budget and output constraints

### Why

RAG must be deterministic about what it can see and how much it can send to the model.

### How

- The UI sends the active topic identifier with the request
- Backend validates that the topic exists and is eligible for retrieval
- Retrieval queries only the embeddings associated with that topic
- Each chunk uses one canonical chunk_id shared across stores:
  - SQLite stores relational metadata (for example `importance_score`, `weakness_score`)
  - Use the Go client for the Chroma vector database to persist and query vector embeddings; record the Chroma vector under the same `chunk_id` so stores remain synchronized

## 4. Content Structure

### What

Source material is stored in a parent-child retrieval layout:

- Parent section: heading-level or section-level content block
- Child chunk: smaller embedded unit used for similarity search

### Why

- Parent sections preserve readable context
- Child chunks improve recall and semantic matching
- Parent expansion prevents awkward fragment-only answers

### How

- Parse content into heading-aware parent sections first
- Split oversized sections with token-aware fallback chunking
- Assign a stable, unique chunk_id to every child chunk at ingest time
- Store embeddings on child chunks
- Keep parent and topic references so retrieval can return original section text and enforce scope

## 5. Retrieval Pipeline

### What

The pipeline is a single pass from query to response.

### Why

Simple control flow is easier to debug and keeps AI behavior predictable.

### How

```text
User question
  -> validate active topic
  -> embed query
  -> search topic-scoped child chunks
  -> ApplyHeuristicScoring (V1: no-op/basic boost, V2: weak-area boosting)
  -> select top-k matches
  -> expand matches to parent sections
  -> assemble prompt within token budget
  -> call OpenAI-compatible model once
  -> return answer with section labels/citations
```

Heuristic scoring contract:

- `ApplyHeuristicScoring` must be a named pipeline step, even if minimal in V1
- V1 behavior can be pass-through or simple deterministic boosts
- V2 plugs in learner-state-aware ranking (for example weakness-based boost)

## 6. Prompt Assembly

### What

Prompt assembly combines the user request with the minimum supporting context needed to answer well.

### Why

The model should see enough material to stay grounded, but never exceed the token budget or receive unnecessary context.

### How

Prompt payload should include:

- User question
- Active topic metadata
- Retrieved parent sections or section excerpts
- Output instructions

Embedding metadata requirements (ingestion-time):

- When using Chroma, create vector records via the Go client and include `topic_id` and `parent_id` metadata.
- Ensure the Chroma record id matches the SQLite `chunk_id` so records can be cross-referenced.
- Keep metadata minimal but sufficient for fast filter-first retrieval

Prompt rules:

- Keep only the most relevant sections
- Remove duplicate context where child hits map to the same parent
- Enforce a strict max token budget before the API call
- Prefer concise answers unless the UI explicitly requests a longer explanation

## 7. Token Budgeting

### What

Token budgeting limits how much context is assembled for the model.

### Why

This prevents truncated prompts, wasted spend, and unstable responses.

### How

- Reserve tokens for the model response first
- Allocate the remainder to retrieved context
- Drop lower-ranked chunks when the budget is exceeded
- Prefer fewer high-signal parent sections over many shallow fragments

Practical rule:

- If a section cannot fit in the remaining budget, do not partially force it in unless the parser can trim it cleanly

## 8. Answer Behavior

### What

RAG responses are grounded explanations, not open-ended chat history.

### Why

The learner should get a direct answer tied to the current topic and the source material.

### How

- Cite or label the section used for the answer when possible
- Keep the answer focused on the user question
- Ask the user to return to the Reader if the topic context is insufficient
- Avoid inventing knowledge not present in retrieved context unless the product explicitly allows brief synthesis

## 9. Failure Modes

### What

RAG can fail because of missing topic context, retrieval problems, or model/API unavailability.

### Why

Users need clear feedback instead of hidden fallback behavior.

### How

- If no active topic exists, stop and show a clear guidance message
- If retrieval returns nothing useful, state that the topic content is insufficient for the request
- If the AI API is unavailable, show an explicit online-required error
- Never fabricate an answer or silently switch to a different topic

## 10. What RAG Does Not Do

### What

These are deliberate exclusions.

### Why

The app stays simpler, more predictable, and easier to maintain.

### How

- No global knowledge search across all topics
- No chat memory between requests
- No agent planning or multi-step tool use
- No background autonomous retriever that rewrites study flow

## 11. Related Data

### What

The retrieval layer depends on the topic, parent, and chunk records stored locally.

### Why

RAG should be traceable back to the source material and the current study state.

### How

- Topic records identify the active scope
- Parent records store human-readable section text
- Chunk records store identifiers, retrieval metadata, and scoring hooks
- The UI uses the returned section labels to show where the answer came from

SQLite schema hooks (required now to avoid later migrations):

- Keep scoring columns on chunk-level records, including:
  - `importance_score`
  - `weakness_score`
- Keep `topic_id` and `parent_id` persisted with each chunk row
- Treat these fields as forward-compatible hooks in V1 (they may be default/unused initially)