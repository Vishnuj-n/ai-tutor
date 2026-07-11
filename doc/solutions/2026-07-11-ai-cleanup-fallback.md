# AI Cleanup Graceful Fallback

**Date:** 2026-07-11
**Files:** `notebook_endpoints.go`, `frontend/src/pages/Notebook.vue`

---

## Problem

When user clicks "AI Clean Up" and the LLM fails (provider nil, API error, etc.), the backend:

1. Set notebook status to `"failed"` in database
2. Returned error to frontend

This left the notebook in a broken state — original bookmark chapters were still valid, but the status was `"failed"`. User could still edit and confirm, but downstream code might reject a `"failed"` notebook.

## Root Cause

`notebook_endpoints.go` lines 257-264 treated LLM failure as a terminal error instead of a degradation scenario.

## Solution

Three-tier fallback in `DraftNotebookSyllabus` when `regenerate=true`:

1. Try LLM — if it works, use LLM chapters
2. If LLM fails/unavailable, try bookmarks (call `DraftSyllabusChapters` with `nil` provider)
3. If no bookmarks either, create single "General" chapter covering all pages

Status always ends up `"draft_ready"`. No error returned to frontend.

Frontend toast updated:
- LLM success: "AI cleaned up chapter list"
- Fallback used: "AI unavailable — using bookmark chapters"

## Before/After

| Scenario | Before | After |
|----------|--------|-------|
| LLM fails, bookmarks exist | Error toast, status `"failed"` | Fallback toast, status `"draft_ready"` |
| LLM fails, no bookmarks | Error toast, status `"failed"` | Single "General" chapter, status `"draft_ready"` |
| LLM provider nil | Error toast, status `"failed"` | Fallback toast, status `"draft_ready"` |

## Testing

- `go test ./internal/notebook/...` passes
- `go build ./...` compiles clean
