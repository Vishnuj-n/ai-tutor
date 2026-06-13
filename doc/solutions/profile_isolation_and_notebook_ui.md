# Walkthrough — Profile Isolation & Notebook UI Simplification

All changes have been successfully implemented across the codebase in a single cohesive phase. Below is a detailed summary of the architectural and UI enhancements.

## Changes Made

### 1. Backend Repository Layer (`internal/db/notebooks_repo.go`)

- Added optional `profileID` parameter to `GetNotebooks(topicID, profileID string)` function
- When `profileID` is set, filters results to include only notebooks belonging to that profile OR unassigned notebooks (NULL/empty `profile_id`)
- When `profileID` is empty, returns all notebooks (backward compatible)
- Profile isolation logic: `(profile_id = ? OR profile_id IS NULL OR profile_id = '')`

### 2. Backend Endpoint Layer (`notebook_endpoints.go`)

- Updated `GetNotebooks(topicID, profileID string)` to accept and forward the new `profileID` parameter to the repository layer
- Mirrors the Chrome-style profile isolation pattern: notebooks belong to specific profiles

### 3. Internal Service Consumers (`internal/retrieval/indexer.go`, `internal/study/sync.go`, `app.go`)

- Updated all internal callers of `db.GetNotebooks()` to pass empty string for `profileID` (no filter) where appropriate
- Indexer and sync services intentionally bypass profile filtering to process all notebooks

### 4. Frontend API Bridge (`frontend/src/services/appApi.js`)

- Updated `getNotebooks(topicID, profileID)` to accept and forward the new `profileID` parameter
- Default empty string maintains backward compatibility with existing callers

### 5. Notebook Page UI (`frontend/src/pages/Notebook.vue`)

- **Removed** the redundant separate "Smart Shelf" section with dual-column Active Lane + Dormant Warehouse layout
- **Restructured** into unified view with active notebooks prioritized at top in a dedicated "Active Lane" section
- Added inline action buttons directly on notebook cards:
  - **Activate** button (purple gradient) on dormant books — disabled when 4 active limit reached
  - **Sleep** button (amber gradient) on active books
- Active notebooks display with purple border glow styling
- Changed heading from "Your Notebooks" to "Dormant Books" when active books exist
- Updated `loadNotebooks()` to pass `activeProfileID.value` for profile-aware filtering

### 6. CSS Styles (`frontend/src/pages/Notebook.vue`)

- Added `.active-lane-section` and `.section-hint` styles for the prioritized active section
- Added `.active-notebook-card` with purple border and glow effect
- Added `.active-icon` for highlighted file icons
- Added `.btn-activate` and `.btn-sleep` action button styles with hover effects

---

## Verification Plan

### Automated Tests
- Run Go unit tests: `go test ./...`
- Go backend compiles successfully (`go build ./...`)
- Frontend builds successfully (`npm run build` in `frontend/`)

### Manual Smoke Testing
1. Switch profiles via dashboard dropdown — verify Notebooks page shows only that profile's books
2. Upload a new notebook while a profile is active — verify it appears in the correct profile
3. Activate a dormant book — verify it moves to the Active Lane at top
4. Sleep an active book — verify it moves to Dormant Books section
5. Try to activate 5th book — verify the Activate button is disabled