# frontend/ — Agent Instructions

## Purpose

Vue 3 + Vite frontend for the AI Tutor. Thin UI layer — all state lives in Go backend via Wails.

---

## Directory Reference

| Directory | Responsibility | Notes |
|-----------|----------------|-------|
| `src/pages/` | Vue page components | One per major view (Reader, Quiz, Dashboard, etc) |
| `src/components/` | Reusable UI components | Keep small and focused |
| `src/services/` | Wails API bridge | All backend calls go through here |
| `src/router/` | Vue Router config | Route → Page mapping |
| `wailsjs/` | Auto-generated Wails bindings | Don't edit manually |

---

## Rules

### ✅ DO

- Call backend via Wails bindings only
- Keep pages thin — render what backend sends
- Show loading states for synchronous operations (quiz generation)
- Handle explicit error states (no silent failures)
- Use Pinia for ephemeral UI state only

### ❌ DON'T

- Access SQLite directly
- Implement business logic in frontend
- Create autonomous flows
- Add hidden state machines
- Track engagement (timers, scroll depth, etc)

---

## Page Responsibilities

Each page receives `task_id` from queue controller, fetches context, renders:

```vue
<!-- Good: Page fetches data, renders -->
<script setup>
import { GetTaskContext } from '../services/appApi.js'

const props = defineProps(['taskId'])
const context = await GetTaskContext(props.taskId)
</script>
```

---

## API Bridge Pattern

```js
// src/services/appApi.js
export async function CompleteTask(taskID, result) {
  return await window.go.backend.App.CompleteTask(taskID, result)
}
```

All Wails calls centralized here — no direct `window.go.*` calls in components.

---

## UI Principles

1. **Explicit over implicit** — Show what's happening (generating, loading, error)
2. **No surveillance** — Don't track user behavior beyond completion
3. **Queue-aware** — UI reflects task lifecycle states

---

## Key Pages

| Page | Purpose |
|------|---------|
| `Dashboard.vue` | Queue state display, task launcher |
| `Reader.vue` | PDF viewing with page locking |
| `Quiz.vue` | Question display + submission |
| `Flashcards.vue` | FSRS review session |

---

## Reference Docs

- App Flow: `../doc/APP_FLOW.md`
- API Contracts: `../doc/DATA_API.md`
- Sprint: `../doc/SPRINT.md`

---

*Last updated: 2026-05-08*
