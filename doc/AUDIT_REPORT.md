# Codebase Audit Report

This report outlines technical debt, DRY principle violations, overly complex logic, and legacy deadweight code identified during the recent audit, in context with the shift to the "Context-Locked Session" model.

## 1. Deadweight & Legacy Fallbacks

**Regex/Markdown parsing**
*   **File:** `internal/notebook/upload.go` (approx. line 447)
*   **Issue:** `splitMarkdownSections` uses strict line-by-line `#` prefix matching to parse Markdown headers. This is a fragile artifact from before the LLM-drafted syllabus feature and struggles with complex document hierarchies.
*   **Recommendation:** Delete `splitMarkdownSections` and rely on the new deterministic LLM syllabus extraction.

*   **File:** `notebook_endpoints.go` (approx. line 293, 371, 900)
*   **Issue:** `extractChapterTitles` and `fallbackChapterTitles` use primitive string checking, and `slugify` uses `regexp.MustCompile(\`[^a-z0-9]+\`)`. These are legacy fallbacks that are brittle and no longer needed with the new architecture.
*   **Recommendation:** Remove `extractChapterTitles`, `fallbackChapterTitles`, and associated regex parsing, deferring entirely to the LLM-drafted syllabus flow.

**Swallowed Errors**
*   **File:** `app.go` (approx. line 1550)
*   **Issue:** `semanticSnippetByTokens` catches tokenization errors (`err != nil`) and silently swallows them, falling back to a rough character-based truncation limit instead of bubbling the error up to the caller to handle gracefully.
*   **Recommendation:** Refactor to return `(string, error)` and let the caller dictate fallback behavior or fail explicitly.

*   **File:** `internal/rag/pipeline.go` (approx. line 204)
*   **Issue:** `trimToTokenBudget` catches truncation and counting errors, but silently returns `"", nil` upon failure.
*   **Recommendation:** Bubble up the error instead of returning `nil` for the error parameter to ensure prompt assembly failures are visible.

## 2. Over-Engineering & Complexity

**SQL Transaction Vulnerability (Panic Leak)**
*   **File:** `internal/db/store.go`, `internal/db/quiz_repo.go`, `internal/db/flashcard_repo.go`, `internal/db/vector_repo.go` (multiple occurrences)
*   **Issue:** Transaction handling is duplicated extensively (`tx, err := conn.Begin()`), but more critically, the deferred rollback is conditional (`defer func() { if err != nil { _ = tx.Rollback() } }()`). Because `err` is not consistently a named return variable, if a panic occurs within the transaction, the deferred function evaluates a `nil` error and fails to execute the rollback, resulting in a database lock leak.
*   **Recommendation:** Refactor to use an unconditional `defer tx.Rollback()` or implement a unified transaction wrapper function that safely handles panics and rollbacks.

**High Cyclomatic Complexity**
*   **File:** `app.go` (approx. line 1550)
*   **Issue:** `semanticSnippetByTokens` has deeply nested `if/else` checks to recursively retry truncation using progressively conservative limits.
*   **Recommendation:** Simplify by extracting the token-budget verification loop into a separate helper function to flatten the control flow.

*   **File:** `internal/rag/pipeline.go` (approx. line 152)
*   **Issue:** `buildPrompt` has complex nested loops evaluating token budgets iteratively against candidate chunks, mixing prompt assembly logic with truncation validation.
*   **Recommendation:** Extract the token management into a dedicated `PromptBuilder` struct that abstracts the budget tracking.

**Frontend State Complexity**
*   **File:** `frontend/src/pages/Notebook.vue` (approx. line 492)
*   **Issue:** The component manually tracks `draftChapters` vs `originalDraftChapters` and deep-diffs them via `chaptersEqual()`. This creates excessively complex state tracking that doesn't leverage Vue's reactivity properly.
*   **Recommendation:** Replace manual diffing functions with a `computed` property that tracks changes dynamically against the original state.

## 3. DRY Principle Violations (Duplication)

**LLM Prompt Building Logic**
*   **File:** `app.go` (approx. line 1191, 1249, 1313)
*   **Issue:** `buildQuizPrompt`, `buildReaderCompletionQuizPrompt`, and `buildFlashcardPrompt` contain severely duplicated logic for looping over source sections, truncating content with `semanticSnippet`, and accumulating `totalContentLength`.
*   **Recommendation:** Extract the context-building loop into a single reusable helper function, `buildContextString(sections, maxContent, maxSections)`.

**Vue Frontend State Duplication**
*   **File:** `frontend/src/pages/Quiz.vue`, `frontend/src/pages/Flashcards.vue`, `frontend/src/pages/WrittenAssessment.vue`, `frontend/src/pages/Reader.vue`
*   **Issue:** Loading and tracking the `notebookTree`, `selectedNotebookID`, `selectedTopicID`, and resolving available topics via computed properties is completely duplicated across all assessment and reading views.
*   **Recommendation:** Extract the notebook selection and topic resolution logic into a reusable Vue composable (e.g., `useNotebookSelection.js`).

**SQL Query Duplication**
*   **File:** `internal/db/quiz_repo.go` (approx. line 13, 110)
*   **Issue:** `replaceQuestionsForTopicRepo` and `appendQuestionsForTopicRepo` contain identically duplicated `INSERT INTO questions ...` query construction inside their loops.
*   **Recommendation:** Extract the `INSERT INTO questions` loop into a shared unexported helper function that both transaction methods call.