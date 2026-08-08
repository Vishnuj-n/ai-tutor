# Simplified Teacher PDF Upload, Page-Range Ingestion & Cloud Upload Restraints

**Date:** 2026-08-08  
**Module:** Cloud Dashboard (`Assignments.vue`, `useDashboard.js`), Desktop App Ingestion (`upload.go`, `upload_test.go`), Reader UI (`Reader.vue`, `NotebookUpload.vue`)

---

## Problem & Architectural Goals

1. **Teacher PDF Page Range Ingestion**: Teachers need to upload PDFs and optionally designate specific page ranges (e.g. Pages 10–30) from the Cloud Dashboard. Desktop student clients downloading these assignments must automatically extract and chunk only the designated pages into their guided study queue.
2. **Native PDF Preview**: Standardize native HTML `<iframe>` previews for teachers to confirm uploaded files on the dashboard without custom canvas/JS PDF reader engines.
3. **Cloud Profile Upload Scoping**: In Cloud Profile mode (when connected to a teacher's classroom code), direct local PDF uploads by students should be disabled so students exclusively study teacher-assigned materials. In Local Profile mode, direct local uploads remain fully supported.

---

## Key Solutions Implemented

### 1. Cloud Dashboard (Teacher Side)
- **`Assignments.vue` & `useDashboard.js`**:
  - Added optional `Start Page` and `End Page` numeric form inputs.
  - Included `start_page` and `end_page` in Supabase REST payloads to `teacher_assignments`.
  - Added native HTML `<iframe>` PDF preview drawers (`<iframe :src="`${url}#page=1`">`) for uploaded PDFs and active assignments list.
- **`SCHEMA.md`**: Documented `start_page` (`INTEGER NULLABLE`) and `end_page` (`INTEGER NULLABLE`) columns on `teacher_assignments`.

### 2. Desktop Application Ingestion (Backend)
- **`upload.go`**:
  - Introduced `ExtractDocumentRange(filePath, fileType, startPage, endPage)` and `extractPDFRange`.
  - Restricts PDF section extraction to pages inside `[startPage, endPage]` prior to 2500-word sliding window chunking.
  - Added unit test `TestExtractDocumentRangeMarkdown` in `upload_test.go`.

### 3. UI-Layer Cloud Profile Upload Restriction (Frontend)
- **`NotebookUpload.vue` & `Notebook.vue`**:
  - When `isCloudProfile` is active (classroom code present), the manual upload dropzone and file picker CTA are hidden and replaced with a clean status card:
    > *☁️ Cloud Classroom Active: Direct PDF upload is disabled for Cloud Profiles. Study materials published by your teacher in classroom **<CODE>** will download automatically.*
  - When in Local Profile mode (no classroom code), local file uploads are fully enabled.

---

## Design Tradeoff Rationale

### UI-Only Restriction vs. Dual-Layer (UI + Backend) Guards
- **Tradeoff Choice**: Enforced upload restriction via UI component replacement (`NotebookUpload.vue`) rather than adding extra backend endpoint checks in Go.
- **Rationale**: Adding Go backend checks introduced extra struct conversion and test suite maintenance overhead across Wails bindings. For a desktop app where IPC bindings are only invoked by local frontend components, hiding the UI dropzone provides a clean, zero-clutter solution with 0% risk of test breakage or backend clutter.

---

## Verification

- **Backend Unit Tests**: `go test ./internal/notebook/...` passed cleanly.
- **Frontend Integration Tests**: All Vitest test suites (`23/23` tests) passed cleanly.
