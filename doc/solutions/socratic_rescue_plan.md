# 🧭 Tutor (Socratic RAG) Integration & Entry Point Clean-up Plan

This document outlines the design and implementation plan to promote the **Socratic Tutor** to a top-level feature named **Tutor**, remove redundant tools, and integrate flexible RAG access configurations.

---

## 1. Objectives

1. **Promotion to "Tutor":** Expose `Socratic.vue` directly on the Sidebar as **Tutor** (using a dedicated icon/label). Remove the "Tools" tab and its redundant placeholders (Acronym Generator, Mindmap Generator).
2. **Access Control Toggles:** Introduce DB schema updates and Settings toggles to enable/disable RAG entry points:
   - **Sidebar -> Notebook -> Chapter -> RAG**: Access from specific chapters/topics.
   - **Sidebar -> All Notebooks -> RAG**: Global cross-notebook RAG chat.
   - **Queue-based Study (Reader/Quiz/etc.)**: Optional inline/collapsible Tutor panel.

---

## 2. Proposed Database Changes

We will add new boolean flags to the `user_settings` table in `internal/db/schema.go` and `internal/db/store.go` to persist user choices for these entry points.

### Schema Alterations (`internal/db/schema.go`)
We will add three new columns:
- `rag_notebook_chapter` (BOOLEAN DEFAULT 1)
- `rag_all_notebooks` (BOOLEAN DEFAULT 1)
- `rag_queue_study` (BOOLEAN DEFAULT 1)

These will be initialized automatically in the `InitSchema` migrations.

---

## 3. Frontend Architecture changes

### 3.1 Sidebar & Router Clean-up
- **Modify:** `frontend/src/components/Sidebar.vue` to remove the `/tools` item and insert the `/tutor` item (using the `Socratic.vue` component).
- **Modify:** `frontend/src/router/index.js` to change `/socratic` path to `/tutor` (redirecting `/socratic` to `/tutor` for safety) and delete `/tools` and its child placeholder routes.
- **Delete:** `frontend/src/pages/Tools.vue` and `frontend/src/pages/ToolPlaceholder.vue`.

### 3.2 Tutor (Socratic) View Updates (`frontend/src/pages/Socratic.vue`)
- Rename page heading/eyebrow to **Tutor** / **Socratic Assistant**.
- Dynamically honor the `rag_all_notebooks` flag:
  - If `rag_all_notebooks` is disabled, the "All notebooks" select option is disabled or hidden, forcing a notebook/chapter selection.
  - If enabled, allow global queries. We will update backend query logic if a global search across all topics is requested (topicID is empty).

### 3.3 Settings Page Integration (`frontend/src/pages/Settings.vue`)
- Render granular toggles under the main RAG switch (visible only when RAG is enabled):
  - `Allow Tutor from Notebook Chapters`
  - `Allow Tutor for All Notebooks`
  - `Allow Tutor inside Queue Study Sessions`
- Update bindings to save these states via the Wails API.

### 3.4 Notebook/Chapter Integration
- When viewing a notebook or its syllabus draft in `Notebook.vue`:
  - Show a small "Ask Tutor" icon next to each chapter/topic.
  - Clicking this navigates to `/tutor?topic_id=<id>&notebook_id=<id>`.
  - Hide this entry point if the `rag_notebook_chapter` setting is disabled.

### 3.5 Optional Queue-based Study Panel
- In `frontend/src/pages/Reader.vue` (and/or `Quiz.vue`):
  - Embed a drawer or side-by-side collapsible pane containing a simplified `Tutor` chat component.
  - The context of the chat is automatically locked to the active reading topic.
  - Hide this panel entirely if `rag_queue_study` is disabled.

---

## 4. Backend RAG Enhancements (`internal/study/socratic.go`)

- Enhance `AskSocratic` to support a global or empty `topicID`:
  - If `topicID` is empty (Global RAG), retrieve relevant chunks using semantic search across all indexed documents instead of being constrained to one topic.

---

## 5. Verification Plan

### Automated Tests
- Run backend unit tests `go test ./...` to verify schema migrations run cleanly.
- Verify `AskSocratic` behaves correctly with empty vs specific topic IDs.

### Manual Verification
- Verify that the Tools page is gone and the Sidebar lists **Tutor**.
- Toggle Settings flags and confirm entry points show/hide correctly.
- Verify page routing with `?topic_id=...` pre-selects the appropriate topic on the Tutor page.