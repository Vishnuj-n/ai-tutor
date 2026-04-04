# AI Tutor App Flow

## 1. UX Intent

### What

A guided tutor experience that always proposes the next best learning action.

### Why

Users progress faster when decision fatigue is removed.

### How

- Dashboard prioritizes actionable tasks
- Every task click routes directly to the exact working screen
- AI appears only as contextual assistance within learning steps

## 2. Navigation and Layout Behavior

### What

Left sidebar navigation with persistent sections:

1. Dashboard
2. Reader
3. Quiz
4. Flashcards
5. Socratic Tutor
6. Settings (bottom)
7. Sync button (bottom)

### Why

Creates a stable mental model and low-friction task switching.

### How

- Active section is highlighted
- Bottom actions remain visible across pages
- Dashboard is default landing page on app launch

## 3. Daily Use Flow

### What

Primary daily sequence:

1. Open Dashboard
2. Complete due reviews
3. Read new topic
4. Mark topic as learned
5. Generate quiz and start review loop

### Why

Aligns behavior with spaced repetition and gradual knowledge expansion.

### How

- Dashboard computes task order from scheduler priorities
- Each completed task updates counters and next recommendations
- Progress summary refreshes immediately after key actions

## 4. Dashboard Flow

### What

Dashboard is the command center for today tasks.

### Why

Centralized action list prevents scattered study behavior.

### How

Task groups:

- Due reviews
- New topics to read
- Optional exploration

Task interaction contract:

- Clicking a task opens the destination page with topic scope
- If data is ready, page renders immediately
- If data requires preparation, show loading and transition automatically

Example:

- Today task says Quiz for Topic 1
- Click task -> Quiz page opens for Topic 1
- Quiz is preloaded, or page shows preparing quiz state until ready

## 5. Reading Flow

### What

Structured concept learning in Reader page.

### Why

Reading should focus on curated sections, not raw source files.

### How

1. User opens a topic from Dashboard or Reader list.
2. Reader shows sectioned content with headings.
3. User uses Ask AI panel for contextual clarifications.
4. User clicks Mark as Learned when confident.
5. System updates topic status and unlocks review lifecycle.

## 6. Ask AI Flow

### What

Single-turn contextual help for the active topic.

### Why

Maintains precision and avoids chatbot drift.

### How

1. User submits question from Reader or Flashcards Explain.
2. Backend runs topic-scoped retrieval.
3. Prompt is assembled from user query plus selected context.
4. LLM returns one response.
5. UI displays answer tied to current topic context.

Rules:

- No cross-topic retrieval by default
- No chat memory between requests
- No fallback output when network/API is unavailable

## 7. Quiz Flow

### What

Topic-based quiz sessions generated after learning milestones.

### Why

Checks understanding before long-term spaced reinforcement.

### How

1. Topic is marked learned.
2. User triggers quiz generation (or receives scheduled prompt).
3. Backend generates and stores quiz_set JSON.
4. Quiz page loads latest set for selected topic.
5. Results can inform follow-up review content.

Offline behavior:

- Existing quizzes can be attempted offline
- New quiz generation requires internet and returns clear error if unavailable

## 8. Flashcards Review Flow

### What

FSRS-driven retention loop for learned material.

### Why

Transforms short-term understanding into durable memory.

### How

1. User starts due review session from Dashboard.
2. Flashcard is shown with recall prompt.
3. User grades recall: Again, Hard, Good, Easy.
4. FSRS updates due date and card state.
5. User may click Explain for contextual AI clarification.

## 9. Socratic Tutor Flow

### What

Optional guided questioning mode within topic scope.

### Why

Promotes deeper reasoning without becoming a general chatbot.

### How

- Select active topic
- Present guided question sequence
- Evaluate user response heuristically
- Offer next question or recommend returning to Reader/Flashcards

## 10. Settings and Sync Flow

### What

Configuration and manual sync trigger.

### Why

Keeps provider switching simple and future sync explicit.

### How

Settings:

- Base URL
- API key
- model
- Phase 2 cloud endpoint placeholder
- App preferences

Sync button:

- Manual trigger only
- Version token format: timestamp + hash
- No distributed conflict resolution in phase 1

## 11. Error and State Feedback Rules

### What

Consistent status signaling for loading, success, and failure.

### Why

A guided app must communicate state clearly at every step.

### How

- Show loading states for any asynchronous page preload
- Show empty-state guidance when no tasks are available
- Show explicit AI-unavailable errors for online-only features
- Never present fabricated AI responses
