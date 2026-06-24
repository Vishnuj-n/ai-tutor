---
name: sync-docs
description: Synchronize project documentation (doc/*.md, README.md) with current codebase state. Audit for staleness, update inaccurate sections, report changes.
---

# Sync Docs

Audit and update project documentation to match the actual codebase.

## When to use

- User says "update the docs", "sync docs with codebase", "docs are out of date", "audit docs"
- After significant code changes or feature additions
- User asks "is my README in sync with the codebase?"

## Procedure

### 1. Establish current state

- `git log --oneline -10` — recent commits for context
- `git diff --stat HEAD~N HEAD` — what changed recently (if user specifies N commits)
- List actual code structure: `internal/`, `frontend/src/`, key config files

### 2. Read all doc files

Read every doc that needs checking:
- `doc/ARCHITECTURE.md`
- `doc/APP_FLOW.md`
- `doc/SCHEMA.md`
- `doc/AGENT_MAP.md`
- `doc/DATA_API.md`
- `doc/PROJECT_STRUCTURE.md`
- `doc/PLATFORM_SUPPORT.md`
- `doc/SPRINT.md`
- `README.md`
- Any other `doc/*.md` files

### 3. Compare against codebase

For each doc, check:

| Section | What to verify |
|---------|---------------|
| Package listing | Do listed packages actually exist in `internal/`? |
| Route/page listing | Do listed Vue pages exist in `frontend/src/pages/`? |
| Sidebar items | Do sidebar nav items match `router/index.js`? |
| Task types | Do listed task types match `models.go` constants? |
| Go version | Does `go.mod` match the documented version? |
| Build instructions | Are build steps still accurate? |
| Feature claims | Do claimed features actually exist in code? |
| Component names | Do referenced components exist? |
| Table schemas | Do documented columns match `schema.go`? |

### 4. Flag discrepancies

For each doc file, report:
- **Accurate** — no changes needed
- **Stale** — list specific sections that are wrong
- **Severely outdated** — entire doc needs rewrite

### 5. Update (if user confirms)

For each stale doc:
- Read the current doc fully
- Edit specific sections to match codebase reality
- Preserve the doc's existing structure and style
- Do NOT add speculative content or future plans
- Do NOT remove TODO sections or future plans

### 6. Validate

- Ensure no broken internal links (e.g., doc references `doc/SCHEMA.md` which should exist)
- Check that code snippets in docs are syntactically correct
- Verify any tables or diagrams are still accurate

### 7. Report

```
| Doc File | Status | Changes |
|----------|--------|---------|
| README.md | ✅ Updated | Fixed Go version, added Notebooks/Onboarding pages |
| ARCHITECTURE.md | ✅ Updated | Added SOCRATIC_REMEDIAL task type, updated priority table |
| SCHEMA.md | ✅ Updated | Added external_help_required column |
| PROJECT_STRUCTURE.md | ⚠️ Severely outdated | Lists nonexistent packages — needs full rewrite |
| SPRINT.md | ✅ Accurate | No changes needed |
```

## Constraints

- User maintains `doc/SCHEMA.md` actively — treat it as authoritative, only add new columns/tables
- Never fabricate documentation for unimplemented features
- Preserve existing doc structure and tone
- If a full rewrite is needed (severely outdated), report that and ask before overwriting
