# Dashboard Status Banner Notification for Books Requiring Ingestion

**Date:** 2026-08-08  
**Module:** Frontend Dashboard & Notebook Ingestion (`StatusBanner.vue`, `Dashboard.vue`, `Notebook.vue`)

---

## Problem & Architectural Goals

1. **New Assignment / Draft Ingestion Alert**: When a new book or assignment is uploaded/assigned but needs chapter extraction & ingestion (status `uploaded` or `draft_ready` with `chunk_count === 0`), users need a clear, un-ignorable inline alert on the main Dashboard to take action.
2. **One-Click Ingestion Workflow**: Clicking the ingestion prompt on the Dashboard must immediately route the user to `/notebooks` and launch the syllabus chapter extraction modal for that specific book.
3. **Minimalist Architecture (Ponytail)**: Avoid adding redundant notification stores, background daemons, or complex pub/sub event buses. Reuse standard Vue 3 lifecycle hooks and the existing `StatusBanner.vue` component.

---

## Key Solutions Implemented

### 1. Action Button Support in `StatusBanner.vue`
- Added optional `actionLabel` prop and `@action` event emitter.
- Added `.banner-action-btn` primary button styling for direct inline actions.

### 2. Dashboard Notification Banner (`Dashboard.vue`)
- Added `pendingIngestionBook` reactive ref and `checkPendingIngestion()` querying helper.
- Renders a `StatusBanner` at the top of `Dashboard.vue` whenever a book needing ingestion is detected:
  - **Title**: `New Assignment — Ingestion Needed`
  - **Subtitle**: `${pendingIngestionBook.title} is ready for chapter extraction and ingestion.`
  - **Action**: `✨ Ingest Book` button routing to `/notebooks?ingest=<notebook_id>`.

### 3. Notebook Page Deep-Linking (`Notebook.vue`)
- Added detection for `route.query.ingest` on `onMounted`.
- Automatically launches `openSyllabusDraft(notebookId, notebookTitle)` when redirected from the Dashboard banner button.

---

## Design Tradeoff Rationale

### Inline Router Query Navigation vs. Global Event Bus / Pinia Store
- **Tradeoff Choice**: Direct router navigation with query parameters (`/notebooks?ingest=<id>`) over a global notification bus or state manager.
- **Rationale**: Minimal code diff, zero boilerplate, zero memory state to sync/persist, 100% deterministic behaviour (YAGNI principle).

---

## Verification

- **Frontend Unit Tests**: All 23 Vitest test suites passed cleanly (`npm test -- --run`).
