This document serves as a "parking lot" for RAG V2 features. **Strict Rule: These ideas are currently out-of-scope for Sprint 2.** They should only be considered for implementation starting in Sprint 4 or 5, once the basic end-to-end pipeline (embed-search-answer) is fully stable.

# RAG Architecture Evolution (V2 Ideas)

## 1. Query Intelligence Layer
The goal is to move from "blind retrieval" to "intent-aware retrieval" by adding a lightweight classification step before the vector search.

* **Query Classifier:** A simple function to categorize incoming questions into:
    * `EXPLAIN`: General conceptual questions.
    * `SUMMARIZE`: High-level overviews.
    * `COMPARE`: Contrast between two concepts.
    * `TEST`: Requesting a quiz or self-assessment.
* **Dynamic Token Strategy:** Adjust the `top-k` value based on the category (e.g., `SUMMARIZE` needs more chunks; `EXPLAIN` needs fewer but more relevant chunks).

## 2. Retrieval Evolution
Moving beyond simple cosine similarity to improve the signal-to-noise ratio.

* **Heuristic Re-ranking:** After fetching the top 10 chunks from `sqlite-vec`, apply a secondary scoring layer in Go:
    * Boost chunks that match the `topic_id` exactly.
    * Boost chunks with high `importance_score`.
    * Prioritize "Parent" sections over "Child" fragments if similarity is nearly equal.
* **Expanded Retrieval Mode:** A toggle in the UI to allow the system to look outside the currently active topic if the query cannot be answered using only local chunks.

## 3. Learning-Aware Retrieval (Adaptive RAG)
Integrate the user's performance data into the retrieval logic to prioritize areas where they are struggling.

* **Weak-Area Boosting:** If a user consistently fails a quiz on a specific `chunk_id`, increase the weight of that chunk during RAG retrieval until their "weakness score" decreases.
* **Difficulty Matching:** Match the `difficulty_level` of retrieved chunks to the user's current progress.

## 4. Proposed Database Schema Additions
These tables will support the features above without breaking the existing SQLite structure.

```sql
-- For cross-topic expansion and prerequisites
CREATE TABLE topic_relations (
  id INTEGER PRIMARY KEY,
  source_topic_id TEXT,
  target_topic_id TEXT,
  relation_type TEXT, -- "prerequisite", "related", "advanced"
  weight REAL DEFAULT 1.0
);

-- For adaptive retrieval based on performance
CREATE TABLE user_weak_areas (
  id INTEGER PRIMARY KEY,
  topic_id TEXT,
  chunk_id TEXT,
  weakness_score REAL,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- For offline tuning and debugging
CREATE TABLE rag_feedback (
  id INTEGER PRIMARY KEY,
  query TEXT,
  topic_id TEXT,
  helpful BOOLEAN,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

## 5. Implementation Roadmap
* **Sprints 1–3:** **Core Stability.** (Focus on Reader, simple RAG, and UI).
* **Sprint 4:** **Query Understanding.** Add rule-based classification and prompt improvements.
* **Sprint 5:** **Ranking Quality.** Implement basic re-ranking and chunk importance scores.
* **Sprint 6:** **Adaptivity.** Integrate FSRS and quiz feedback into the retrieval loop.

## 6. Maintenance of Core Philosophy
Even in V2, the system must remain:
1.  **Deterministic:** The same query and context should generally yield the same result.
2.  **Debuggable:** Every response must clearly show which chunks were used.
3.  **Local-First:** Keep the heavy lifting (SQLite, vector search) on the local machine.
4.  **Anti-Agentic:** Avoid autonomous "loops." Keep the AI as a high-precision tool inside the learning workflow.