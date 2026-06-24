---
name: dead-code-audit
description: Audit a list of functions flagged as unreachable by static analysis. Search codebase for references, report verdict per function, delete only confirmed dead code when asked.
---

# Dead Code Audit

Given a list of functions/types flagged as unreachable by `go vet`, `golangci-lint`, or similar tools, verify each one and produce a verdict.

## When to use

- User pastes output from `go vet -unreachable`, `golangci-lint`, or similar
- User says "check if these are really dead code", "are these unreachable", "audit dead code"
- Input format: list of file paths + function/type names flagged as unreachable

## Procedure

### 1. Parse the flagged items

Extract from the user's input:
- **File path** for each flagged function/type
- **Function/type name** (exact identifier)

### 2. Search for references

For each flagged function, search the entire codebase:
- `rg -n "<FunctionName>" --type go` — find all references
- Check: definition site, call sites, test files, generated code
- Check if it's exported (may be called from other packages)
- Check if it implements an interface (satisfies a contract even if never directly called)

### 3. Make a verdict

| Verdict | Meaning | Action |
|---------|---------|--------|
| ✅ **Dead** | Only defined, zero references anywhere | Report for deletion |
| ❌ **Not dead** | Referenced in production code | Skip, note where it's used |
| ⚠️ **Test-only** | Only referenced in `_test.go` files | Flag — may be intentional test helper |
| 🔒 **Interface impl** | Implements an interface even if not directly called | Skip — needed for type satisfaction |

### 4. Report

Output a table:

```
| Function | File | Verdict | Evidence |
|----------|------|---------|----------|
| InitMockKeyringForTests | keyring.go:50 | ✅ Dead | Only defined, never called |
| WithExtractPDFFunc | upload.go:42 | ❌ Not dead | Used in upload_test.go:146 |
| WithQueryDueReviewCards | service.go:57 | ❌ Not dead | Used in service_test.go:13,68,112,152 |
```

### 5. Delete (only if user explicitly asks)

When the user says "delete the dead ones":
- Remove only functions with ✅ Dead verdict
- Remove the function definition and any associated comments
- Do NOT remove the file if other functions remain in it
- Run `go build ./...` after each deletion
- If deletion breaks compilation, restore and report the issue

## Constraints

- Never delete without explicit user permission
- Always report before deleting — the user may have a reason to keep something
- Check test files — "dead in prod, alive in tests" is common and intentional
- Check interface satisfaction before flagging as dead
