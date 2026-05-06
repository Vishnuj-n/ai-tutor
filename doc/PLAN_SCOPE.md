# AI-Tutor V4 — Plan & Scope

## Project Summary

AI-Tutor V4 is a desktop application that transforms PDFs into structured, spaced-repetition learning sessions. It combines a Go backend (via Wails v2), a React/TypeScript frontend, an embedded SQLite database, and an LLM integration to automatically generate quizzes and flashcards from uploaded reading material.

---

## Goals

- Replace passive reading with active, tested learning sessions driven by FSRS v4 spaced repetition.
- Automatically generate quizzes and flashcards from any text-layer PDF using an LLM.
- Enforce daily reading quotas calculated from user-defined session blocks and deadlines.
- Surface remediation and memory-collapse recovery flows so weak material is re-studied before it is forgotten.
- Run entirely as a native desktop app with no cloud dependency beyond the user-configured LLM endpoint.

---

## In Scope

### Core Session Engine
- Global FSRS review phase at session start: due cards first, new cards last, timeboxed countdown.
- Remediation phase: detect blocks with stability below threshold, insert re-read tasks, require a pass at Reader Level threshold + 10% before continuing.
- Velocity queue: score notebooks by `(Stars × 10) + VelocityRequirement`, tie-break by highest velocity then alphabetical filename.
- Session block enforcement: close session gracefully if remaining block time is less than the next mission duration.

### PDF Ingestion Pipeline
- Extract entire PDF to raw text using `pdftotext -layout` (poppler-utils).
- Clean text: join hyphenated words, remove headers/footers, normalize whitespace to produce clean paragraphs.
- Build paragraph-aware blocks (~2500 words each) while tracking page ranges.
- Parse PDF bookmark tree to detect chapter boundaries and tag blocks.
- OCR detection on the first 3 pages: flag `ocr_required` and block ingestion if no text layer is found.
- Greedy ingestion: parse and generate quizzes for upcoming blocks in the background while the user is reading the current one.

### LLM Integration
- Single HTTP client compatible with any OpenAI-format endpoint (OpenAI, Anthropic, local proxy).
- Phase 1 (during reading): generate quiz questions per mission. Retry once if fewer than 2 questions are returned; unlock reader boundary and log a warning if the retry also fails.
- Phase 2 (during break): generate flashcards as new FSRS cards; optionally generate an essay question for Expert-level readers where card stability exceeds 80%.
- Phase 2 timeout: 5 minutes; cards are queued for the next day if generation does not complete in time.
- Context window: fixed system prompt (reader level) + 500 words of previous lesson or chapter title + full current mission text + task instruction.

### FSRS Engine
- Cards enter as `New` state, progress through standard FSRS v4 states.
- Quiz score percentage maps to FSRS rating (proportional: partial failures hit stability less hard than total failures).
- Timebox enforcement: stops serving cards when the countdown expires; reschedules remaining due cards to tomorrow.
- Memory collapse: 3 consecutive flashcard failures lock the current mission, force a re-read of the source block, and require a pass at Reader Level threshold + 10% before resuming.

### Scheduler & Quota
- Daily quota = `RemainingWords / Days`; recalculated when the last session gap exceeds 24 hours.
- Three alert types: Schedule Alert (pace is tight), Critical Alert (deadline is mathematically impossible), Expansion Alert (new PDF upload changes pace).
- Session block parser tracks remaining time in the current block and detects overflow.

### Data Layer
- SQLite via `modernc/sqlite` (zero-dependency, embedded).
- All SQL contained in `internal/db/queries.go`; no raw queries elsewhere.
- Sequential forward-only migrations via a version table.
- Mission and quiz rows carry a `status` column (`pending` / `ready` / `failed`) so the orchestrator only serves missions with quizzes ready.

### Frontend
- Wails v2 bridge: all `window.go.*` calls are isolated to `lib/wailsBindings.ts` to keep the Go API mockable.
- Barrel exports for all component folders; no deep import paths.
- State managed in stores (no fetch logic inside stores); business logic lives in hooks (`useSession`, `useQuiz`, `useFSRS`, `useTimer`).
- Shared components (`Button`, `Modal`, `Timer`, `Alert`) are pure presentational with no store dependencies.

### First-Run Setup
- Setup Wizard collects Brain, Body, and Engine settings on first launch and persists them to SQLite.

---

## Out of Scope

- **`internal/rag_stub`**: fully inactive in V4; must not be called from any other package.
- OCR processing: the app detects OCR-required PDFs and blocks them with a warning; it does not perform OCR itself.
- Essay question results do not feed into FSRS scoring.
- Cloud sync, multi-device support, or any remote database.
- Any LLM provider integration beyond a standard OpenAI-compatible HTTP endpoint.

---

## Key Constraints

| Constraint | Detail |
|---|---|
| LLM retry budget | Phase 1: one retry only. Failure after retry unlocks boundary without crashing the session. |
| Phase 2 timeout | 5 minutes hard cap. Overflow cards queue to tomorrow; session continues uninterrupted. |
| Quiz minimum | Fewer than 2 questions = retry; still failing = log + unlock. Never block the session. |
| FSRS ordering | Due cards always before new cards. New cards never jump the queue. |
| Session block overflow | Graceful close with full state saved. Never truncate mid-mission. |
| Chapter boundary | Context overlap resets at chapter boundaries detected by the syllabus parser. |
| RAG stub | Dead code in V4. Zero calls permitted from live packages. |

---

## Concurrency Boundaries

Three goroutines run in parallel during a session:

- **Main goroutine** — drives the session flow: reader display, quiz gate, FSRS updates, break timer.
- **Background ingestion goroutine** — parses the next mission PDF and runs Phase 1 quiz generation while the user reads the current mission.
- **Background Phase 2 goroutine** — generates flashcards and optional essay questions during the break; subject to the 5-minute timeout.

Background goroutines write to separate SQLite rows and never block the main session goroutine.

---

## Milestone Checklist

- [ ] SQLite schema, migrations, and typed query layer (`internal/db`)
- [ ] PDF ingestion pipeline: extractor, cleaner, chunker, syllabus parser, OCR detection (`internal/parser`)
- [ ] FSRS engine and scoring map (`internal/fsrs`)
- [ ] Scheduler: session blocks, quota calculation, alert emission (`internal/scheduler`)
- [ ] Orchestrator: velocity scoring, mission selection, remediation, memory collapse (`internal/orchestrator`)
- [ ] LLM client, prompt library, pipeline phases, retry logic (`internal/tutor`)
- [ ] Wails bridge (`app.go`) wiring all packages to the frontend
- [ ] React component tree: Reader, QuizGate, Flashcard, Dashboard, Settings, shared primitives
- [ ] Hook layer: `useSession`, `useQuiz`, `useFSRS`, `useTimer`
- [ ] Store layer: `sessionStore`, `fsrsStore`, `notebookStore`
- [ ] First-run Setup Wizard
- [ ] Concurrency integration test: greedy ingestion + Phase 2 do not block main goroutine
- [ ] End-to-end session flow test: happy path from PDF upload through FSRS update
- [ ] Edge case coverage: OCR block, LLM retry failure, Phase 2 timeout, memory collapse loop, impossible deadline alert
