# Branch Diff Report: main vs feature/quiz-loop

## Overview
**Total files changed:** 37 files  
**Total lines:** +34,058 insertions, -786 deletions  
**Review depth:** Deep (500+ lines, 10+ files, touches data mutation)

## File Changes Summary

### 1. **Core Application Files**
- `app.go` (+152/-?): Major changes including reading task persistence and new quiz/reader endpoints
- `app_contract_test.go` (+31/-?): Updated contract tests
- `AGENTS.md` (+9/-?): Updated agent instructions
- `.gitignore` (+4): Added new ignore patterns

### 2. **Documentation**
- `doc/APP_FLOW.md` (+4/-?): Updated application flow
- `doc/ARCHITECTURE.md` (+8/-?): Updated architecture documentation
- `god_files_report.md` (-17): Removed file
- `phase_1_plan.md` (-241): Removed planning document
- `graphify-out/GRAPH_REPORT.md` (+517): Added graph analysis report
- `graphify-out/graph.html` (+276): Added visualization HTML
- `graphify-out/graph.json` (+31,314): Added graph data

### 3. **Frontend Components**
- `frontend/src/composables/useChat.js` (+107): New chat composable
- `frontend/src/composables/useReaderBase.js` (+274): New reader base composable
- `frontend/src/pages/Dashboard.vue` (+3): Minor dashboard updates
- `frontend/src/pages/Flashcards.vue` (+2/-?): Flashcard updates
- `frontend/src/pages/Notebook.vue` (+78/-?): Notebook page updates
- `frontend/src/pages/Quiz.vue` (+457/-?): Major quiz component refactor
- `frontend/src/pages/Reader.vue` (+314/-?): Major reader component refactor
- `frontend/src/pages/WrittenAssessment.vue` (+5/-?): Assessment updates
- `frontend/src/services/appApi.js` (+24/-?): API service updates

### 4. **Database Layer**
- `internal/db/notebook_topic_tree_repo.go` (+14/-?): Notebook topic tree repository
- `internal/db/notebooks_repo.go` (+55): New notebook repository methods
- `internal/db/quiz_repo.go` (+8): Quiz repository updates
- `internal/db/reader_bundle_repo.go` (+11): Reader bundle repository
- `internal/db/schema.go` (+20): Schema updates
- `internal/db/store.go` (+19): Store updates
- `internal/db/study_queue_repo.go` (+192/-?): Major study queue repository updates
- `internal/db/study_queue_repo_test.go` (+15/-?): Test updates
- `internal/db/topics_repo.go` (+34/-?): Topics repository updates

### 5. **Business Logic**
- `internal/models/models.go` (+42): Model definitions
- `internal/notebook/upload.go` (+134): Notebook upload logic
- `internal/scheduler/service.go` (+1): Scheduler service update
- `internal/scheduler/service_test.go` (+3): Test updates
- `internal/study/flashcard.go` (+129/-?): Flashcard generation logic
- `internal/study/quiz_sync.go` (+286): New quiz synchronization logic
- `internal/study/reader.go` (+3/-?): Reader completion updates
- `notebook_endpoints.go` (+41/-?): Notebook endpoint updates

## Key Changes Analysis

### 1. **Reader Component Refactoring**
The Reader component has been significantly refactored:
- Extracted logic into composables (`useReaderBase`, `useChat`)
- Deprecated manual reading flow (redirects to dashboard)
- Consolidated task flow initialization
- Added reading task persistence to queue

### 2. **Quiz System Enhancement**
Major quiz system updates:
- New `quiz_sync.go` with comprehensive quiz generation
- Quiz attempt submission with scoring
- Automatic re-read task generation for failed quizzes
- Integration with study queue

### 3. **Database Schema Updates**
Multiple database changes:
- New fields for task tracking
- Enhanced study queue management
- Quiz attempt persistence
- Syllabus draft caching

### 4. **Architecture Improvements**
- Better separation of concerns with composables
- Consolidated backend initialization
- Improved error handling
- Enhanced data persistence

## Critical Issues Found

### 1. **Backend-Frontend Contract Issues**
- `useReaderBase` expects specific response structure from `InitializeReadingSession`
- No TypeScript interfaces for backend responses
- Missing validation of backend data

### 2. **Error Handling Gaps**
- Silent error discarding in `app.go` (`_ = db.InsertStudyTask`)
- Inconsistent error propagation
- Missing error boundaries

### 3. **Security Concerns**
- No task ownership validation
- Direct error message exposure
- Missing input sanitization

### 4. **State Management Issues**
- Fragmented state between components and composables
- Potential race conditions
- Missing synchronization mechanisms

### 5. **Testing Gaps**
- No integration tests for new quiz flow
- Missing frontend component tests
- Incomplete contract validation

## Architecture Violations Check

Based on `AGENTS.md` rules:

### ✅ **Compliant**
- Queue controls progression (tasks persisted to study queue)
- SQLite as source of truth (all state in database)
- Frontend doesn't own business logic (moved to composables/backend)
- Deterministic ordering maintained

### ⚠️ **Potential Issues**
- **Hidden orchestration state**: New quiz sync logic may create implicit flows
- **Background queue mutation**: Reading tasks auto-persisted in `GetTodayPlan`
- **FSRS creates tasks, not flow control**: Need to verify flashcard generation doesn't orchestrate

## Verification Status

### ✅ **Go Tests Pass**
All Go tests pass successfully.

### ⚠️ **Frontend Build**
Build failed due to file lock (`Access is denied`), not code compilation issues.

### ❓ **Integration Tests**
Not run - need to test:
- Full quiz flow
- Reader task flow
- Database migrations
- Error scenarios

## Recommendations

### Immediate Actions
1. **Verify Backend Contracts**
   - Validate `InitializeReadingSession` response structure
   - Add TypeScript interfaces
   - Implement runtime validation

2. **Fix Error Handling**
   - Remove silent error discarding
   - Add proper error logging
   - Implement error boundaries

3. **Add Security Validation**
   - Validate task ownership
   - Sanitize error messages
   - Add input validation

### Medium-term Improvements
1. **Add Comprehensive Testing**
   - Integration tests for all flows
   - Frontend component tests
   - Contract validation tests

2. **Improve State Management**
   - Consolidate state management
   - Add synchronization mechanisms
   - Implement proper error recovery

3. **Document Contracts**
   - Backend API documentation
   - Composable interfaces
   - Data flow diagrams

## Risk Assessment

### High Risk
- Backend-frontend contract mismatches
- Silent error discarding
- Missing security validation

### Medium Risk
- State management fragmentation
- Missing integration tests
- Incomplete error handling

### Low Risk
- Code organization issues
- Documentation gaps
- Minor UI inconsistencies

## Conclusion

The `feature/quiz-loop` branch contains significant improvements to the quiz system and reader component refactoring. However, several critical issues need attention before merging:

1. **Backend-frontend contract validation** is essential to prevent runtime errors
2. **Error handling improvements** are needed to avoid silent failures
3. **Security validation** must be added to prevent unauthorized access
4. **Comprehensive testing** is required to ensure system stability

The changes align with the project's architecture principles but introduce some potential violations that need verification.


## Critical Bug Found

### **Parameter Name Mismatch in useReaderBase.js**

**Location:** `frontend/src/composables/useReaderBase.js:117-127`

**Issue:** The `initializeSession` function in `useReaderBase.js` is calling `initializeReadingSession` with parameter names that don't match the query object structure.

**Problem Code:**
```javascript
const result = await initializeReadingSession(
  taskID.value,
  query.notebookId || query.notebook_id,    // OK: checks both camelCase and snake_case
  query.topicId || query.topic_id,          // OK: checks both camelCase and snake_case
  query.startPage || query.start_page,      // BUG: query object has startPage (camelCase)
  query.endPage || query.end_page           // BUG: query object has endPage (camelCase)
)
```

**Root Cause:** In `Reader.vue`, the query object is created with:
```javascript
const query = {
  notebookId: route.query.notebookId || route.query.notebook_id,
  topicId: route.query.topicId || route.query.topic_id,
  startPage: parseInt(route.query.startPage || route.query.start_page) || 0,  // camelCase
  endPage: parseInt(route.query.endPage || route.query.end_page) || 0         // camelCase
}
```

But `useReaderBase.js` is checking for `query.start_page` and `query.end_page` (snake_case) which don't exist in the query object.

**Impact:** The `startPage` and `endPage` parameters will always be `0` because `query.start_page` and `query.end_page` are undefined, making the fallback to `0`.

**Fix Required:** Change the parameter access in `useReaderBase.js` to use only camelCase:
```javascript
const result = await initializeReadingSession(
  taskID.value,
  query.notebookId,
  query.topicId,
  query.startPage,  // Remove || query.start_page
  query.endPage     // Remove || query.end_page
)
```

Or ensure the query object includes both naming conventions.


## Additional Issues Found

### 1. **Backend Response Structure Validation**
**Issue:** The frontend assumes the backend always returns the full response structure when there's no error, but doesn't validate the structure.

**Risk:** If the backend returns `OK: true` but missing some fields (e.g., `bundle` is nil), the frontend will crash when accessing `bundle.topic_title`.

**Fix:** Add defensive checks for all expected properties:
```javascript
if (!result?.task || !result?.page_bounds) {
  globalError.value = 'Invalid response from server'
  return false
}
```

### 2. **Error Handling Inconsistency**
**Issue:** The backend returns a structured `ReadingSessionResponse` with `OK` boolean and `Error` string, but the frontend only checks `result?.error`.

**Risk:** If backend returns `OK: false` with empty `Error` string (edge case), frontend won't detect the error.

**Fix:** Check both `result.ok` and `result.error`:
```javascript
if (result?.ok === false || result?.error) {
  globalError.value = result.error || 'Unknown error'
  return false
}
```

### 3. **Missing TypeScript/Validation**
**Issue:** No TypeScript interfaces or runtime validation for backend responses.

**Risk:** Type mismatches cause runtime errors.

**Fix:** Add TypeScript interfaces or runtime validation:
```javascript
// TypeScript interface
interface ReadingSessionResponse {
  ok: boolean
  error?: string
  task?: { notebook_id: string; topic_id: string; /* ... */ }
  page_bounds?: { start_page: number; end_page: number; current_page: number }
  navigation?: { can_go_prev: boolean; can_go_next: boolean; can_complete: boolean }
  bundle?: { topic_title: string; notebook_url: string; file_type: string; page_count: number; sections: any[] }
}
```

## Summary of Critical Issues

1. **Parameter name mismatch** in `useReaderBase.js` (camelCase vs snake_case)
2. **Missing response structure validation** in frontend
3. **Incomplete error checking** (only checks `error`, not `ok`)
4. **No TypeScript/runtime validation** for backend contracts
5. **Silent error discarding** in `app.go` (`_ = db.InsertStudyTask`)
6. **Missing task ownership validation** (security risk)
7. **Fragmented state management** between component and composables

## Priority Fixes

### **P0 (Blocking)**
1. Fix parameter name mismatch in `useReaderBase.js`
2. Add response structure validation in frontend
3. Fix silent error discarding in `app.go`

### **P1 (High Priority)**
1. Add task ownership validation
2. Improve error checking (check both `ok` and `error`)
3. Add basic TypeScript interfaces

### **P2 (Medium Priority)**
1. Consolidate state management
2. Add integration tests
3. Document backend-frontend contracts