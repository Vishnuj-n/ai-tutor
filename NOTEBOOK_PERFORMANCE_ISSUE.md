# Notebook Performance Issue Analysis

## Problem
Confirming notebook chapter or name edits is slow because `ConfirmNotebookSyllabus` always does full re-ingestion, even for metadata-only updates.

## Root Cause
1. `ConfirmNotebookSyllabus` extracts the entire document (slow for PDFs)
2. Rebuilds all chunks from document text
3. Re-ingests chunks into database
4. Updates embeddings

## Current Partial Fix (UI)
- Modified `confirmSyllabusDraft` in `Notebook.vue` to skip `ConfirmNotebookSyllabus` if chapters didn't change
- This fixes the case where user only changes notebook title

## Remaining Issues
1. Chapter title changes (without page boundary changes) still trigger full re-ingestion
2. No optimization for already-ingested ("chunked") notebooks

## Proposed Backend Solution

### Option 1: Modify `ConfirmNotebookSyllabus`
Add logic to:
1. Check if notebook status is "chunked"
2. Get existing topics and their page bounds (need new DB function)
3. Compare with new chapter boundaries
4. If boundaries unchanged:
   - Skip document extraction (use stored page count)
   - Skip `BuildTopicGroupsFromChapters` and `IngestNotebookContentByTopic`
   - Update topic titles via `EnsureTopicsBatch` (topic IDs may change)
   - Update chunk topic IDs if needed (database update, not re-ingestion)
5. If boundaries changed:
   - Do full re-ingestion as before

### Option 2: New endpoint for metadata updates
Create `UpdateNotebookMetadata` endpoint that:
1. Updates notebook title
2. Updates topic titles
3. Doesn't re-ingest content
4. Returns error if chapter boundaries changed (requires re-ingestion)

### Option 3: Hybrid approach
Modify `ConfirmNotebookSyllabus` to:
1. Always validate using stored page count (skip document extraction)
2. Check if re-ingestion is needed (boundaries changed)
3. If not needed, just update metadata
4. If needed, extract document and re-ingest

## Technical Challenges
1. Topic IDs are generated from chapter titles: `nb-{notebookID}-ch-{index}-{sanitizedTitle}`
   - If chapter title changes, topic ID changes
   - Need to create new topics and update chunk references
2. Need function to get existing topics for a notebook
3. Need function to update chunk topic IDs in bulk

## Recommended Approach
Implement Option 1 with these steps:

1. Add `GetNotebookTopics(notebookID string)` function in `topics_repo.go`
2. Modify `ConfirmNotebookSyllabus` to:
   - Use `nb.PageCount` for validation (skip document extraction initially)
   - Check if notebook is "chunked"
   - If yes, get existing topics and compare boundaries
   - If boundaries unchanged, update metadata only
   - If boundaries changed, extract document and re-ingest
3. Add helper to update chunk topic IDs when topic titles change

## UI Improvements
1. Show warning when re-ingestion will happen
2. Disable "Confirm and Ingest" button with tooltip if only metadata changed (offer "Update Metadata" instead)
3. Show notebook status in syllabus modal

## Testing
1. Test metadata-only updates (fast)
2. Test boundary changes (slow, as before)
3. Test mixed changes (some boundaries changed, some didn't)