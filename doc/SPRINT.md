# SPRINT.md — AI Tutor
 

## The Immutable Architecture Rules (Apply to all Sprints)
1. **Fresh Schema:** No migration scripts. Delete `ai-tutor.db` and let `store.go` rebuild it. (manually already done at location AppData\Roaming\ai-tutor)
2. **One Page, One Chunk:** Text chunks strictly map to a single `page_num`. 
3. **Question Lineage:** Every generated question stores `source_page_start`, `source_page_end`, `llm_model`, and `prompt_version`.
4. **Hard Deletion:** If a user shrinks a chapter boundary, execute an immediate SQL `DELETE` for questions orphaned by the new boundaries.
5. **Two-Step Fast Retrieval:** Vector search must pre-filter `rowid` by `topic_id` and `page_num` *before* executing the distance calculation to avoid virtual table join penalties.

---

## Sprint 12: Schema Rebuild & Dynamic Pacing
**Goal:** Establish the foundation and the time-budget math engine.

* **Database Reset:** Implement the fresh schema. Add `page_num` to `chunks`. Add `current_page_cursor` to `topics`. Create the `user_settings` table to store a single `daily_study_minutes` integer.
* **The Scheduler Math (`service.go`):** * Calculate FSRS review time: `(DueCards * 0.5 mins) = ReviewBudget`.
  * Calculate reading capacity: `(daily_study_minutes - ReviewBudget) = ReadingBudget`.
  * Calculate page target: `ReadingBudget / 2.5 mins = PagesToRead`.
  * Calculate end cursor: `TargetPage = current_page_cursor + PagesToRead`.
* **The Clamp Edge Case:** If `TargetPage` lands within 4 pages of the topic's `end_page`, force `TargetPage = end_page`.
* **The Settings UI:** Build the input in the Vue frontend to update the global `daily_study_minutes` limit.
* **Output:** Return a daily task formatted as: `"Read: [Topic] (Pages X to Y)"`.

## Sprint 13: Context-Locked Reader & The Great Purge
**Goal:** Build the execution environment, generate assessments safely, and delete legacy guesswork code.

* **The Purge:** Delete regex TOC parsers, blind 180-word splitters, and vector-based ingestion routing.
* **Semantic Chunker:** Write the new chunker that splits text at the nearest period or newline around the 150-word mark, strictly bounded by page endings.
* **The Reader UI:** Mount the PDF viewer. Accept the start and target page numbers from the dashboard. Add a "Complete Session" button.
* **Cursor Advancement:** Only advance the database `current_page_cursor` when the user explicitly clicks "Complete Session". 
* **Mid-Sentence Buffer Fetch:** When "Complete" is clicked, fetch text using: `SELECT text FROM chunks WHERE topic_id = ? AND page_num BETWEEN ? AND ?+1 ORDER BY id ASC`. Send this text to the FAST_LLM.
* **Incremental Assessment Generation:** Generate 5 questions. Save them with exact page lineage and prompt version metadata.
* **Acceptance Gates:** Context-locked vector retrieval must test at p95 < 50ms. Macro-quiz assembly from stored questions must test at p95 < 100ms.

## Sprint 14: FSRS Integration & Smart Scaling
**Goal:** Tie generated assessments to memory algorithms and automate background generation.

* **FSRS Hookup:** Connect the FSRS scoring algorithm to the quiz and Socratic examiner outputs. Track success/failure on individual generated questions.
* **Density Scaling:** Replace hardcoded assessment counts. Pass the total chunk length to the FAST_LLM and instruct it to scale the number of flashcards and quiz questions to match the material density.
* **Background Queue:** Implement a Go routine worker. Identify the next two reading tasks in the schedule. Pre-build the quizzes and flashcards for these upcoming sessions while the user reads the current text.

## Sprint 15: Task Management & Dashboard Routing
**Goal:** Finalize the user dashboard experience.

* **Persistent Checklist:** Build a task checklist in the left sidebar. Allow users to tick off items to log completed work.
* **State Routing:** Wire the dashboard buttons to control application state. Clicking a reading task mounts `Reader.vue`, loads the topic, and physically locks the context to the assigned pages.
* **Completion State:** Clear the dashboard state when the user completes the daily queue.

## Sprint 16: Concurrency & Tools Sidebar
**Goal:** Optimize speed and add specific learning utilities.

* **Concurrent Ingestion:** Rewrite the PDF indexing pipeline to use Go routines. Process chapter chunking and ONNX embedding concurrently.
* **Acronym Generator:** Add an acronym tool to the sidebar. Pass the locked active page context to the FAST_LLM and request mnemonic devices. 
* **Mindmap Generator:** Add a mindmap tool that reads the locked page context and outputs structured JSON for a frontend rendering library.
* **Documentation Rewrite:** Update `/doc` files to document the dual-LLM routing, the context-locked schema, and the two-step vector retrieval.