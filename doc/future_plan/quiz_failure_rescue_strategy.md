# Quiz Failure Rescue Strategy — User-Configurable Remediation

## Overview

Let users choose what happens after a quiz failure. Instead of hardcoding `maxAutomaticRereadAttempts = 1`, the backend reads a user preference and routes accordingly.

Two tracks:

| Track | Behavior | Best For |
|-------|----------|----------|
| **Fast Track** | Quiz fail → SOCRATIC_REMEDIAL directly | Deep encoding, conceptual topics |
| **Classic Track** | Quiz fail → REREAD → quiz fail → SOCRATIC_REMEDIAL | Dense text, sequential learners |

Default: **Classic Track** (current behavior, backward compatible).

---

## Flow Comparison

```
CLASSIC (current):
  Quiz Fail → REREAD → Quiz Fail → SOCRATIC_REMEDIAL → Re-quiz

FAST:
  Quiz Fail → SOCRATIC_REMEDIAL → Re-quiz
```

---

## Database Change

Add one column to `user_settings`:

```sql
ALTER TABLE user_settings ADD COLUMN default_remedial_strategy TEXT DEFAULT 'CLASSIC';
```

Values: `'CLASSIC'` | `'FAST'`

No new table. Single-row table (`id = 1`), already has the pattern.

---

## Backend Changes

### 1. `internal/db/schema.go`

Add to `alterStatements`:

```go
{"user_settings", "default_remedial_strategy", "ALTER TABLE user_settings ADD COLUMN default_remedial_strategy TEXT DEFAULT 'CLASSIC'"},
```

### 2. `internal/db/store.go`

Add getter/setter:

```go
func (r *Store) GetRemedialStrategy() (string, error) {
    var strategy string
    err := r.db.QueryRow(
        `SELECT COALESCE(default_remedial_strategy, 'CLASSIC') FROM user_settings WHERE id = 1`,
    ).Scan(&strategy)
    return strategy, err
}

func (r *Store) SetRemedialStrategy(strategy string) error {
    _, err := r.db.Exec(
        `UPDATE user_settings SET default_remedial_strategy = ? WHERE id = 1`, strategy,
    )
    return err
}
```

### 3. `internal/study/quiz_sync.go`

Replace the hardcoded constant with a runtime check:

```go
// Remove this:
// const maxAutomaticRereadAttempts = 1

// In SubmitQuizAttempt, before the failure branch:
strategy, _ := s.repo.GetRemedialStrategy()

if strategy == "FAST" {
    // Skip reread, go straight to SOCRATIC_REMEDIAL
    // (reuse existing strike-3 logic from line 385)
} else {
    // Classic: current reread-then-socratic flow
    // (existing logic unchanged)
}
```

The `maxAutomaticRereadAttempts` const stays for Classic track. FAST track bypasses it entirely.

### 4. `app.go` — Wails bindings

```go
func (a *App) GetRemedialStrategy() string {
    strategy, _ := a.store.GetRemedialStrategy()
    return strategy
}

func (a *App) SetRemedialStrategy(strategy string) error {
    return a.store.SetRemedialStrategy(strategy)
}
```

---

## Frontend Changes

### Settings UI — Remediation Strategy Section

Add to Settings.vue (or split into a sub-component `RemediationSettings.vue` if the page keeps growing):

```vue
<template>
  <div class="settings-section">
    <h3>Quiz Failure Rescue</h3>
    <p class="settings-description">
      Choose what happens when you fail a quiz.
    </p>

    <div class="strategy-options">
      <label class="strategy-option" :class="{ active: strategy === 'CLASSIC' }">
        <input type="radio" value="CLASSIC" v-model="strategy" />
        <div class="option-content">
          <span class="option-title">Classic Track</span>
          <span class="option-desc">Reread first, then Socratic tutor if you fail again</span>
        </div>
      </label>

      <label class="strategy-option" :class="{ active: strategy === 'FAST' }">
        <input type="radio" value="FAST" v-model="strategy" />
        <div class="option-content">
          <span class="option-title">Fast Track</span>
          <span class="option-desc">Go directly to Socratic AI tutor (deeper encoding)</span>
        </div>
      </label>
    </div>
  </div>
</template>
```

Wire to `SetRemedialStrategy` on change. Read on mount via `GetRemedialStrategy`.

---

## Settings Page Split (If Needed)

If Settings.vue is already large, extract a new component:

```
frontend/src/components/settings/
├── RemediationSettings.vue    ← new
├── StudyTimeSettings.vue      ← existing/future
├── LLMSettings.vue            ← existing/future
└── SyncSettings.vue           ← existing/future
```

Settings.vue becomes a thin shell that imports these section components. Each section handles its own save/load.

---

## Testing

### Unit Tests

1. **TestFastTrackSkipsReread**
   - Set strategy = "FAST"
   - Fail quiz
   - Assert: SOCRATIC_REMEDIAL inserted, no REREAD

2. **TestClassicTrackInsertsReread** (existing behavior)
   - Set strategy = "CLASSIC" (or default)
   - Fail quiz
   - Assert: REREAD inserted

3. **TestDefaultIsClassic**
   - Fresh DB, no settings change
   - Fail quiz
   - Assert: REREAD inserted (backward compatible)

### Integration Test

4. **TestFullFlowFastTrack**
   - Set FAST → fail quiz → SOCRATIC_REMEDIAL → complete → re-quiz pass → flashcards

5. **TestFullFlowClassicTrack**
   - Set CLASSIC → fail quiz → REREAD → fail quiz → SOCRATIC_REMEDIAL → re-quiz pass → flashcards

---

## Implementation Order

1. Schema migration — add column (5 min)
2. Store getter/setter (10 min)
3. `quiz_sync.go` — branch on strategy (20 min)
4. Wails bindings (5 min)
5. Settings UI component (30 min)
6. Unit tests (20 min)
7. Integration test (15 min)

**Estimated: ~1.5 hours**

---

## Success Criteria

- [ ] `user_settings.default_remedial_strategy` column exists
- [ ] Default value is `CLASSIC` (no behavior change for existing users)
- [ ] FAST track: quiz fail → SOCRATIC_REMEDIAL (no reread)
- [ ] CLASSIC track: quiz fail → REREAD → quiz fail → SOCRATIC_REMEDIAL
- [ ] Settings page shows two radio options with descriptions
- [ ] Setting persists across app restarts
- [ ] `go test ./...` passes
- [ ] `wails dev` loads without errors
