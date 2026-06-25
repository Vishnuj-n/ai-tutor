# Walkthrough — Flashcard Review Forecast Graph on Dashboard

We have successfully implemented and refined the Flashcard Review Forecast timeline graph on the Dashboard page!

---

## Changes Implemented

### 1. Database Query Range Support
- **[internal/db/store.go](file:///c:/Users/vishn/PROJECT/ai-tutor/internal/db/store.go)**: Added `QueryDueReviewCardsForRange(start int64, end int64)` to count active, profile-scoped flashcards due within a specific time interval.
- **[internal/db/flashcard_repo_test.go](file:///c:/Users/vishn/PROJECT/ai-tutor/internal/db/flashcard_repo_test.go)**: Added the `TestQueryDueReviewCardsForRange` unit test to verify target range boundaries, suspended card filtering, and profile scoping.

### 2. Backend Wails Bindings
- **[app_study.go](file:///c:/Users/vishn/PROJECT/ai-tutor/app_study.go)**: Exposed a new `GetFlashcardDueTimeline()` API endpoint returning a 7-day timeline forecast of due counts relative to local midnight.

### 3. Layout Restructuring (Two-Column Sidebar)
- **[frontend/src/pages/Dashboard.vue](file:///c:/Users/vishn/PROJECT/ai-tutor/frontend/src/pages/Dashboard.vue)**:
  - Restructured the dashboard content into a two-column layout (`.dashboard-grid`).
  - **Left column (`.dashboard-main`)**: Holds today's study tasks, victory cards, and onboarding states.
  - **Right column (`.dashboard-sidebar`)**: Contains the forecast timeline widget, keeping it square-shaped (`aspect-ratio: 1 / 1.05`) and compact on desktop instead of stretching horizontally. On mobile/stacked views, it degrades gracefully with tasks positioned first ("above").
  - Changed the SVG coordinates and viewBox from `500x200` to a balanced `400x300` resolution.
  - Plotted clean `400x300` grid lines and coordinates for lines, gradients, and dots.
  - Added percent-based Y positioning (`percentY`) for hover tooltips so that coordinates scale uniformly.

### 4. Interactive Axis scale & Ticks
- **[frontend/src/pages/Dashboard.vue](file:///c:/Users/vishn/PROJECT/ai-tutor/frontend/src/pages/Dashboard.vue)**:
  - Added a dynamic `yTicks` computed property that partitions the graph's vertical range into clean, integer-divisible increments (`0%`, `25%`, `50%`, `75%`, `100%`) representing actual card counts.
  - Plotted grid lines and corresponding Y-axis text labels dynamically next to the chart axes.
  - Plotted solid border axes (`.axis-line`) on the left and bottom of the chart.
  - Aligned the horizontal threshold line to sit cleanly inside the Y-axis margins.

### 5. API Bridge
- **[frontend/src/services/appApi.js](file:///c:/Users/vishn/PROJECT/ai-tutor/frontend/src/services/appApi.js)**: Added Wails bridge integration for `getFlashcardDueTimeline`.

---

## Verification Results

### Go Test Suite Passes
All Go unit and integration tests passed successfully:
```powershell
go test ./...
# ok  	ai-tutor                   16.260s
# ok  	ai-tutor/internal/db       19.610s
# ok  	ai-tutor/internal/scheduler 3.580s
```

### Vite Frontend Build Compiles
The Vite production assets compile without warnings:
```bash
vite v6.4.3 building for production...
✓ built in 15.11s
dist/index.html                                 0.38 kB
dist/assets/WrittenAssessment-dZHoCFjG.css      5.60 kB
dist/assets/index-D7_H37f5.css                112.59 kB
dist/assets/WrittenAssessment-BJYplLtc.js       4.90 kB
dist/assets/index-B-Qh925T.js               2,877.85 kB
```
