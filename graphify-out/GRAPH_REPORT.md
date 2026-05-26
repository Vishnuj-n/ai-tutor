# Graph Report - ai-tutor  (2026-05-10)

## Corpus Check
- 99 files · ~130,103 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1039 nodes · 2153 edges · 40 communities detected
- Extraction: 59% EXTRACTED · 41% INFERRED · 0% AMBIGUOUS · INFERRED: 882 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)
- [[_COMMUNITY_Community 0|Community 0]]
- [[_COMMUNITY_Community 1|Community 1]]
- [[_COMMUNITY_Community 2|Community 2]]
- [[_COMMUNITY_Community 3|Community 3]]
- [[_COMMUNITY_Community 4|Community 4]]
- [[_COMMUNITY_Community 5|Community 5]]
- [[_COMMUNITY_Community 6|Community 6]]
- [[_COMMUNITY_Community 7|Community 7]]
- [[_COMMUNITY_Community 8|Community 8]]
- [[_COMMUNITY_Community 9|Community 9]]
- [[_COMMUNITY_Community 10|Community 10]]
- [[_COMMUNITY_Community 11|Community 11]]
- [[_COMMUNITY_Community 12|Community 12]]
- [[_COMMUNITY_Community 13|Community 13]]
- [[_COMMUNITY_Community 14|Community 14]]
- [[_COMMUNITY_Community 15|Community 15]]
- [[_COMMUNITY_Community 16|Community 16]]
- [[_COMMUNITY_Community 17|Community 17]]
- [[_COMMUNITY_Community 18|Community 18]]
- [[_COMMUNITY_Community 19|Community 19]]
- [[_COMMUNITY_Community 20|Community 20]]
- [[_COMMUNITY_Community 21|Community 21]]
- [[_COMMUNITY_Community 23|Community 23]]
- [[_COMMUNITY_Community 24|Community 24]]
- [[_COMMUNITY_Community 25|Community 25]]
- [[_COMMUNITY_Community 30|Community 30]]
- [[_COMMUNITY_Community 31|Community 31]]
- [[_COMMUNITY_Community 32|Community 32]]
- [[_COMMUNITY_Community 33|Community 33]]
- [[_COMMUNITY_Community 44|Community 44]]
- [[_COMMUNITY_Community 45|Community 45]]
- [[_COMMUNITY_Community 46|Community 46]]
- [[_COMMUNITY_Community 47|Community 47]]
- [[_COMMUNITY_Community 48|Community 48]]
- [[_COMMUNITY_Community 49|Community 49]]
- [[_COMMUNITY_Community 50|Community 50]]
- [[_COMMUNITY_Community 51|Community 51]]
- [[_COMMUNITY_Community 52|Community 52]]
- [[_COMMUNITY_Community 53|Community 53]]
- [[_COMMUNITY_Community 54|Community 54]]

## God Nodes (most connected - your core abstractions)
1. `Errorf()` - 174 edges
2. `EnsureTopic()` - 58 edges
3. `App` - 51 edges
4. `initDBForTest()` - 49 edges
5. `newTestApp()` - 44 edges
6. `appBridge()` - 43 edges
7. `CreateNotebook()` - 36 edges
8. `contains()` - 23 edges
9. `StudyService` - 22 edges
10. `InsertStudyTask()` - 21 edges

## Surprising Connections (you probably didn't know these)
- `countFlashcardsForTopicRepo()` --calls--> `CountFlashcardsForTopic()`  [INFERRED]
  internal\db\flashcard_repo.go → internal\db\store.go
- `UpdateNotebookChunkCount()` --calls--> `TestIngestNotebookContentByTopicRollsBackOnMidTransactionFailure()`  [INFERRED]
  internal\db\notebooks_repo.go → internal\db\store_integration_test.go
- `TestSchemaIncludesRereadAttemptsTable()` --calls--> `initDBForTest()`  [INFERRED]
  internal\db\study_queue_repo_test.go → internal\db\test_helpers_test.go
- `TestSchemaIncludesReviewTaskCardsTableAndIndex()` --calls--> `initDBForTest()`  [INFERRED]
  internal\db\study_queue_repo_test.go → internal\db\test_helpers_test.go
- `QueryNextReadingTopic()` --calls--> `Warnf()`  [INFERRED]
  internal\db\topics_repo.go → internal\utils\logging.go

## Hyperedges (group relationships)
- **Wails Bridge Interface** — app_askai, app_scoreanswer, notebook_endpoints_uploadnotebookfrompath, notebook_endpoints_draftnotebooksyllabus [EXTRACTED 0.90]
- **Notebook Ingestion Workflow** — notebook_endpoints_uploadnotebookfrompath, notebook_endpoints_draftnotebooksyllabus, notebook_endpoints_confirmnotebooksyllabus [EXTRACTED 1.00]
- **Graphify Extraction Tools** — check_cache_script, detect_files_script, extract_ast_script [EXTRACTED 1.00]
- **Frontend Pages and Routing** — dashboard_page, reader_page, notebook_page, quiz_page, flashcards_page, written_assessment, socratic_page, tools_page, settings_page, vue_router [EXTRACTED 1.00]
- **Wails Integration Layer** — app_api, wails_app_bridge, wails_models, wails_runtime [INFERRED 0.95]
- **Database Repository Layer** — assessment_repo_createwrittenquestionrepo, flashcard_repo_createflashcardsrepo, notebooks_repo_createnotebook, notebooks_repo_getnotebooks [INFERRED 0.95]
- **Persistent Study Queue Architecture** — doc_plan_scope_corequeue, doc_requirements_studyqueue, doc_schema_studyqueue, doc_data_api_queuerouter [EXTRACTED 0.95]

## Communities

### Community 0 - "Community 0"
Cohesion: 0.03
Nodes (94): createWrittenQuestionRepo(), getAssessmentFSRSStateFromQuerier(), getAssessmentFSRSStateRepo(), getAssessmentFSRSStateRepoTx(), getWrittenQuestionByIDRepo(), upsertAssessmentFSRSReviewRepo(), upsertAssessmentFSRSReviewRepoTx(), AssessmentFSRSRecord (+86 more)

### Community 1 - "Community 1"
Cohesion: 0.05
Nodes (95): extractFirstChunkID(), extractRequestedCount(), flashcardJSON(), initTestDB(), initTestPipeline(), initTestProvider(), mustInsertActiveQuizTask(), newTestApp() (+87 more)

### Community 2 - "Community 2"
Cohesion: 0.03
Nodes (38): App, deleteNotebookRepo(), doesTableExistTxRepo(), ingestNotebookContentByTopicRepo(), insertChunkRowRepo(), insertParentRowRepo(), linkNotebookChunkRowRepo(), NotebookChunkInput (+30 more)

### Community 3 - "Community 3"
Cohesion: 0.03
Nodes (9): RecordFlashcardReview(), UpdateDailyStudyMinutes(), loadAgenda(), generate(), loadQueueSession(), rate(), generateManualQuiz(), submitQuiz() (+1 more)

### Community 4 - "Community 4"
Cohesion: 0.06
Nodes (48): normalizeQuizAnswer(), ChunkEmbeddingBatchItem, ChunkVectorBatchItem, countFlashcardsForTopicRepo(), createFlashcardsRepo(), getFlashcardByIDQuerier(), getFlashcardByIDRepo(), getFlashcardByIDRepoTx() (+40 more)

### Community 5 - "Community 5"
Cohesion: 0.06
Nodes (42): mapReviewRating(), NewApp(), queueTaskToScheduledTask(), resolveAppDir(), resolveDBPath(), resolveNotebookDir(), ingestionProgressPayload, llmProviderInterface (+34 more)

### Community 6 - "Community 6"
Cohesion: 0.04
Nodes (3): EventsOn(), EventsOnce(), EventsOnMultiple()

### Community 7 - "Community 7"
Cohesion: 0.12
Nodes (46): GetNotebookTopicTree(), CreateParentSection(), AppendQuestionsForTopic(), GetQuestionsForTopic(), InsertFSRSReviewLog(), distanceFunctionAvailable(), p95Duration(), TestAppendQuestionsForTopicPreservesExistingQuestions() (+38 more)

### Community 8 - "Community 8"
Cohesion: 0.06
Nodes (32): NormalizeWhitespace(), ExtractedDocument, ExtractedSection, FileMetadata, BuildTopicGroupsFromChapters(), chapterIndexForPage(), markdownSection, Option (+24 more)

### Community 9 - "Community 9"
Cohesion: 0.04
Nodes (46): Chunk, ChunkWithContext, CompletionResult, ExtractedSubtopic, Flashcard, FlashcardState, FSRSReviewLog, GeneratedFlashcard (+38 more)

### Community 10 - "Community 10"
Cohesion: 0.08
Nodes (32): initCleanTestDB(), TestGetNotebookTopicTreeEmptyReturnsArray(), loadExtension(), GetTopicContent(), InitSchema(), Init(), SeedDemoDataForTests(), ApplyHeuristicScoring() (+24 more)

### Community 11 - "Community 11"
Cohesion: 0.09
Nodes (43): activateTask(), appBridge(), askAI(), askReaderAI(), completeReading(), completeReadingSession(), completeReviewSession(), confirmNotebookSyllabus() (+35 more)

### Community 12 - "Community 12"
Cohesion: 0.11
Nodes (28): buildTokenArrays(), destroyValues(), extractEmbedding(), extractIONames(), inferMaxSeqLen(), meanPool2D(), meanPool2DFloat64(), meanPool3D() (+20 more)

### Community 13 - "Community 13"
Cohesion: 0.12
Nodes (22): Engine, cosineSimilarity(), NewEngine(), tokenize(), Scope, SearchResult, Option, queryDailyStudyMinutesFn (+14 more)

### Community 14 - "Community 14"
Cohesion: 0.12
Nodes (17): Config, openAIMessage, openAIRequest, openAIResponse, Provider, firstEnv(), firstEnvInt(), LoadConfigFromEnv() (+9 more)

### Community 15 - "Community 15"
Cohesion: 0.1
Nodes (6): CompletionResult, NotebookTopicTreeNode, NotebookTopicTreeTopic, QuizAnswer, StudyQueueTask, SyllabusChapterDraft

### Community 16 - "Community 16"
Cohesion: 0.17
Nodes (16): TestParsePDFCPUBookmarkDraftFromJSON_EmptyPayload(), TestParsePDFCPUBookmarkDraftFromJSON_NestedPayload(), LLMProvider, extractPDFCPUBookmarkDraft(), findPDFCPUExecutable(), firstInt(), firstString(), ParsePDFCPUBookmarkDraftFromJSON() (+8 more)

### Community 17 - "Community 17"
Cohesion: 0.27
Nodes (13): clampFloat(), easyStabilityFactor(), estimateRetrievability(), firstReviewState(), goodStabilityFactor(), hardStabilityFactor(), maxInt(), NextFSRSState() (+5 more)

### Community 18 - "Community 18"
Cohesion: 0.27
Nodes (4): copyFile(), fileSHA256(), NewAssetValidator(), AssetValidator

### Community 19 - "Community 19"
Cohesion: 0.38
Nodes (9): chunkVectorBatchItemRepo, UpsertChunkVectorsBatch(), isVectorUnavailableError(), lookupChunkRowIDRepo(), searchVectorsForNotebookRepo(), searchVectorsForTopicRepo(), upsertChunkVectorRepo(), upsertChunkVectorsBatchRepo() (+1 more)

### Community 20 - "Community 20"
Cohesion: 0.33
Nodes (6): Sliding Window Chunking, onnxruntime_go, RAG Architecture, sqlite-vec, blocks table, block_vectors table

### Community 21 - "Community 21"
Cohesion: 0.5
Nodes (4): Queue Router API, Core Queue System, Persistent Guided Study Queue, study_queue table

### Community 23 - "Community 23"
Cohesion: 0.67
Nodes (3): Engine, LLMProvider, StudyService

### Community 24 - "Community 24"
Cohesion: 0.67
Nodes (3): AGENT_MAP.md, buildPageBoundedContext, study

### Community 25 - "Community 25"
Cohesion: 0.67
Nodes (3): Persistent Queue Model, AGENTS.md, ARCHITECTURE.md

### Community 30 - "Community 30"
Cohesion: 1.0
Nodes (2): Service, service

### Community 31 - "Community 31"
Cohesion: 1.0
Nodes (2): FSRS, NextFSRSState

### Community 32 - "Community 32"
Cohesion: 1.0
Nodes (2): Frontend (Vue), Vue 3 + Vite Frontend

### Community 33 - "Community 33"
Cohesion: 1.0
Nodes (2): Backend (Go + Wails), Go Backend

### Community 44 - "Community 44"
Cohesion: 1.0
Nodes (1): retrieval

### Community 45 - "Community 45"
Cohesion: 1.0
Nodes (1): SearchResult

### Community 46 - "Community 46"
Cohesion: 1.0
Nodes (1): runtime

### Community 47 - "Community 47"
Cohesion: 1.0
Nodes (1): AssetValidator

### Community 48 - "Community 48"
Cohesion: 1.0
Nodes (1): scheduler

### Community 49 - "Community 49"
Cohesion: 1.0
Nodes (1): scaledQuizQuestionCount

### Community 50 - "Community 50"
Cohesion: 1.0
Nodes (1): utils

### Community 51 - "Community 51"
Cohesion: 1.0
Nodes (1): APP_FLOW.md

### Community 52 - "Community 52"
Cohesion: 1.0
Nodes (1): The Academic Curator

### Community 53 - "Community 53"
Cohesion: 1.0
Nodes (1): Windows-First Target

### Community 54 - "Community 54"
Cohesion: 1.0
Nodes (1): fsrs_cards table

## Knowledge Gaps
- **107 isolated node(s):** `llmProviderInterface`, `ragPipelineInterface`, `ingestionProgressPayload`, `OpenAIRequest`, `OpenAIMessage` (+102 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **Thin community `Community 30`** (2 nodes): `Service`, `service`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 31`** (2 nodes): `FSRS`, `NextFSRSState`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 32`** (2 nodes): `Frontend (Vue)`, `Vue 3 + Vite Frontend`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 33`** (2 nodes): `Backend (Go + Wails)`, `Go Backend`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 44`** (1 nodes): `retrieval`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 45`** (1 nodes): `SearchResult`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 46`** (1 nodes): `runtime`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 47`** (1 nodes): `AssetValidator`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 48`** (1 nodes): `scheduler`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 49`** (1 nodes): `scaledQuizQuestionCount`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 50`** (1 nodes): `utils`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 51`** (1 nodes): `APP_FLOW.md`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 52`** (1 nodes): `The Academic Curator`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 53`** (1 nodes): `Windows-First Target`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 54`** (1 nodes): `fsrs_cards table`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Errorf()` connect `Community 0` to `Community 1`, `Community 2`, `Community 3`, `Community 4`, `Community 5`, `Community 7`, `Community 8`, `Community 9`, `Community 10`, `Community 12`, `Community 13`, `Community 14`, `Community 16`, `Community 18`, `Community 19`?**
  _High betweenness centrality (0.550) - this node is a cross-community bridge._
- **Why does `rate()` connect `Community 3` to `Community 0`?**
  _High betweenness centrality (0.105) - this node is a cross-community bridge._
- **Why does `loadAgenda()` connect `Community 3` to `Community 2`, `Community 11`?**
  _High betweenness centrality (0.102) - this node is a cross-community bridge._
- **Are the 172 inferred relationships involving `Errorf()` (e.g. with `.startup()` and `resolveAppDir()`) actually correct?**
  _`Errorf()` has 172 INFERRED edges - model-reasoned connections that need verification._
- **Are the 57 inferred relationships involving `EnsureTopic()` (e.g. with `mustInsertActiveQuizTask()` and `TestGetNotebookTopicTreeReturnsNestedTopics()`) actually correct?**
  _`EnsureTopic()` has 57 INFERRED edges - model-reasoned connections that need verification._
- **Are the 47 inferred relationships involving `initDBForTest()` (e.g. with `TestUpdateFlashcardReviewTransactionalSave()` and `TestUpdateFlashcardReviewRollsBackCardOnLogInsertFailure()`) actually correct?**
  _`initDBForTest()` has 47 INFERRED edges - model-reasoned connections that need verification._
- **Are the 7 inferred relationships involving `newTestApp()` (e.g. with `New()` and `NewService()`) actually correct?**
  _`newTestApp()` has 7 INFERRED edges - model-reasoned connections that need verification._