---
name: verify-finding
description: Verify a code review or static analysis finding against current code. Fix only still-valid issues, skip stale ones, validate the result.
---

# Verify Finding

Verify whether a reported code finding is still valid in the current codebase, fix it if so, and validate.

## When to use

- User pastes a finding from a code review, linter, or static analysis tool
- User says "verify this finding", "check if this bug still exists", "is this still valid"
- Input format: file path + line range + description of the issue

## Procedure

### 1. Parse the finding

Extract from the user's input:
- **File path** (may need glob resolution)
- **Line range** (approximate)
- **Issue description** (what's wrong, what the expected behavior is)

### 2. Read the reported code

Read the file at the reported location with surrounding context (±50 lines). Understand:
- What the code currently does
- What the finding claims is wrong
- Whether the code has changed since the finding was reported

### 3. Investigate the claim

Use grep/glob to verify supporting evidence:
- If the finding says "X is never checked" — search for all callers
- If the finding says "error is silently ignored" — check error handling paths
- If the finding says "method Y bypasses the repository" — check if the code still uses direct SQL

### 4. Make a verdict per finding

For each finding, decide:

| Verdict | Action |
|---------|--------|
| **Still valid** | Fix it. Minimal, targeted edit. No refactoring beyond the fix. |
| **Stale — code changed** | Skip with one-line explanation of why it no longer applies. |
| **False positive** | Skip with brief reasoning. |

### 5. Apply fixes (if any)

- Edit only the specific lines related to the finding
- Do NOT refactor surrounding code
- Do NOT rename variables or reorganize
- Keep changes minimal and focused

### 6. Validate

Run the appropriate validation:
- `go build ./...` for Go changes
- `go test ./... -count=1` to ensure no regressions
- `go vet ./...` or `golangci-lint run` if available
- For frontend: check for syntax errors, run dev server if applicable

### 7. Report

Output a table:

```
| # | File | Finding | Verdict | Action |
|---|------|---------|---------|--------|
| 1 | app_study.go:375 | silent error suppression | ✅ Fixed | Added early return on ActivateTask error |
| 2 | Settings.vue:503 | RAG modal out of sync | ✅ Fixed | Added handleRagModalDismiss guard |
| 3 | service.go:57 | WithQueryDueReviewCards unreachable | ⏭ Skipped | Used in service_test.go — false positive |
```

## Constraints

- Never delete code unless explicitly asked
- Never refactor beyond the minimal fix
- Always validate with build + tests after changes
- If a fix would require architectural changes, report that instead of applying it
