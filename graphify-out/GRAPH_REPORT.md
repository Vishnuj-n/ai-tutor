# Graph Report - .  (2026-05-09)

## Corpus Check
- 122 files · ~107,034 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 986 nodes · 1846 edges · 72 communities detected
- Extraction: 62% EXTRACTED · 38% INFERRED · 0% AMBIGUOUS · INFERRED: 698 edges (avg confidence: 0.8)
- Token cost: 12,000 input · 800 output

## Community Hubs (Navigation)
- [[_COMMUNITY_Core App Logic (Go Bridge)|Core App Logic (Go Bridge)]]
- [[_COMMUNITY_Flashcards & Quiz Service|Flashcards & Quiz Service]]
- [[_COMMUNITY_Database & Test Utilities|Database & Test Utilities]]
- [[_COMMUNITY_Notebook & Topic Management|Notebook & Topic Management]]
- [[_COMMUNITY_SRS & Review Logic (FSRS)|SRS & Review Logic (FSRS)]]
- [[_COMMUNITY_Wails Frontend Runtime|Wails Frontend Runtime]]
- [[_COMMUNITY_Text Processing & Ingestion|Text Processing & Ingestion]]
- [[_COMMUNITY_Application Setup & Environment|Application Setup & Environment]]
- [[_COMMUNITY_Core Data Models|Core Data Models]]
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
- [[_COMMUNITY_Community 22|Community 22]]
- [[_COMMUNITY_Community 23|Community 23]]
- [[_COMMUNITY_Community 24|Community 24]]
- [[_COMMUNITY_Community 25|Community 25]]
- [[_COMMUNITY_Community 27|Community 27]]
- [[_COMMUNITY_Community 28|Community 28]]
- [[_COMMUNITY_Community 29|Community 29]]
- [[_COMMUNITY_Community 30|Community 30]]
- [[_COMMUNITY_Community 33|Community 33]]
- [[_COMMUNITY_Community 34|Community 34]]
- [[_COMMUNITY_Community 35|Community 35]]
- [[_COMMUNITY_Community 36|Community 36]]
- [[_COMMUNITY_Community 37|Community 37]]
- [[_COMMUNITY_Community 54|Community 54]]
- [[_COMMUNITY_Community 55|Community 55]]
- [[_COMMUNITY_Community 56|Community 56]]
- [[_COMMUNITY_Community 57|Community 57]]
- [[_COMMUNITY_Community 58|Community 58]]
- [[_COMMUNITY_Community 59|Community 59]]
- [[_COMMUNITY_Community 60|Community 60]]
- [[_COMMUNITY_Community 61|Community 61]]
- [[_COMMUNITY_Community 62|Community 62]]
- [[_COMMUNITY_Community 63|Community 63]]
- [[_COMMUNITY_Community 64|Community 64]]
- [[_COMMUNITY_Community 65|Community 65]]
- [[_COMMUNITY_Community 66|Community 66]]
- [[_COMMUNITY_Community 67|Community 67]]
- [[_COMMUNITY_Community 68|Community 68]]
- [[_COMMUNITY_Community 69|Community 69]]
- [[_COMMUNITY_Community 70|Community 70]]
- [[_COMMUNITY_Community 71|Community 71]]
- [[_COMMUNITY_Community 72|Community 72]]
- [[_COMMUNITY_Community 73|Community 73]]
- [[_COMMUNITY_Community 74|Community 74]]
- [[_COMMUNITY_Community 75|Community 75]]
- [[_COMMUNITY_Community 76|Community 76]]
- [[_COMMUNITY_Community 77|Community 77]]
- [[_COMMUNITY_Community 78|Community 78]]
- [[_COMMUNITY_Community 79|Community 79]]
- [[_COMMUNITY_Community 80|Community 80]]
- [[_COMMUNITY_Community 81|Community 81]]
- [[_COMMUNITY_Community 82|Community 82]]
- [[_COMMUNITY_Community 83|Community 83]]
- [[_COMMUNITY_Community 84|Community 84]]
- [[_COMMUNITY_Community 85|Community 85]]
- [[_COMMUNITY_Community 86|Community 86]]
- [[_COMMUNITY_Community 87|Community 87]]
- [[_COMMUNITY_Community 88|Community 88]]
- [[_COMMUNITY_Community 89|Community 89]]
- [[_COMMUNITY_Community 90|Community 90]]
- [[_COMMUNITY_Community 91|Community 91]]

## God Nodes (most connected - your core abstractions)
1. `Errorf()` - 135 edges
2. `EnsureTopic()` - 49 edges
3. `App` - 44 edges
4. `initDBForTest()` - 41 edges
5. `newTestApp()` - 38 edges
6. `appBridge()` - 36 edges
7. `CreateNotebook()` - 28 edges
8. `contains()` - 22 edges
9. `initTestDB()` - 17 edges
10. `ReplaceQuestionsForTopic()` - 17 edges

## Surprising Connections (you probably didn't know these)
- `TestIngestNotebookContentByTopicRollsBackOnMidTransactionFailure()` --calls--> `UpdateNotebookChunkCount()`  [INFERRED]
  internal\db\store_integration_test.go → internal\db\notebooks_repo.go
- `TestStudyQueueLifecycleAndState()` --calls--> `UpdateNotebookPriority()`  [INFERRED]
  internal\db\study_queue_repo_test.go → internal\db\notebooks_repo.go
- `Go Backend` --semantically_similar_to--> `Backend (Go + Wails)`  [INFERRED] [semantically similar]
  internal/AGENTS.md → doc/PROJECT_STRUCTURE.md
- `Vue 3 + Vite Frontend` --semantically_similar_to--> `Frontend (Vue)`  [INFERRED] [semantically similar]
  frontend/AGENTS.md → doc/PROJECT_STRUCTURE.md
- `initTestDB()` --calls--> `Init()`  [INFERRED]
  app_contract_test.go → internal\db\store.go

## Hyperedges (group relationships)
- **Wails Bridge Interface** — app_askai, app_scoreanswer, notebook_endpoints_uploadnotebookfrompath, notebook_endpoints_draftnotebooksyllabus [EXTRACTED 0.90]
- **Notebook Ingestion Workflow** — notebook_endpoints_uploadnotebookfrompath, notebook_endpoints_draftnotebooksyllabus, notebook_endpoints_confirmnotebooksyllabus [EXTRACTED 1.00]
- **Graphify Extraction Tools** — check_cache_script, detect_files_script, extract_ast_script [EXTRACTED 1.00]
- **Frontend Pages and Routing** — dashboard_page, reader_page, notebook_page, quiz_page, flashcards_page, written_assessment, socratic_page, tools_page, settings_page, vue_router [EXTRACTED 1.00]
- **Wails Integration Layer** — app_api, wails_app_bridge, wails_models, wails_runtime [INFERRED 0.95]
- **Database Repository Layer** — assessment_repo_createwrittenquestionrepo, flashcard_repo_createflashcardsrepo, notebooks_repo_createnotebook, notebooks_repo_getnotebooks [INFERRED 0.95]
- **Persistent Study Queue Architecture** — doc_plan_scope_corequeue, doc_requirements_studyqueue, doc_schema_studyqueue, doc_data_api_queuerouter [EXTRACTED 0.95]

## Communities

### Community 0 - "Core App Logic (Go Bridge)"
Cohesion: 0.03
Nodes (37): App, GetNotebooks(), UpdateNotebookStatus(), GetReaderTopicBundle(), GetDailyStudyMinutes(), CompleteReading(), CompleteReadingWithGeneratedQuiz(), CompleteTask() (+29 more)

### Community 1 - "Flashcards & Quiz Service"
Cohesion: 0.05
Nodes (71): TestGenerateShortAnswerPrompt_Success(), normalizeQuizAnswer(), createWrittenQuestionRepo(), getAssessmentFSRSStateFromQuerier(), getAssessmentFSRSStateRepo(), getAssessmentFSRSStateRepoTx(), getWrittenQuestionByIDRepo(), upsertAssessmentFSRSReviewRepo() (+63 more)

### Community 2 - "Database & Test Utilities"
Cohesion: 0.07
Nodes (66): extractFirstChunkID(), extractRequestedCount(), flashcardJSON(), initCleanTestDB(), initTestDB(), initTestPipeline(), initTestProvider(), newTestApp() (+58 more)

### Community 3 - "Notebook & Topic Management"
Cohesion: 0.05
Nodes (41): TestGenerateFlashcardsCreatesAndReturnsCards(), TestGenerateFlashcardsReturnsExistingCardsWithoutDuplication(), countFlashcardsForTopicRepo(), GetChunksForTopics(), GetChunkTextsForTopicPageRange(), GetParentPassagesForTopicPageRange(), GetParentSection(), GetTopicContent() (+33 more)

### Community 4 - "SRS & Review Logic (FSRS)"
Cohesion: 0.11
Nodes (51): TestExplainReaderSection_EmptyQuestion(), TestExplainReaderSection_Success(), insertFSRSReviewLogRepo(), GetNotebookTopicTree(), CreateChunk(), CreateParentSection(), insertChunkRow(), AppendQuestionsForTopic() (+43 more)

### Community 5 - "Wails Frontend Runtime"
Cohesion: 0.04
Nodes (3): EventsOn(), EventsOnce(), EventsOnMultiple()

### Community 6 - "Text Processing & Ingestion"
Cohesion: 0.07
Nodes (29): NormalizeWhitespace(), ExtractedDocument, ExtractedSection, FileMetadata, markdownSection, Option, Service, absInt() (+21 more)

### Community 7 - "Application Setup & Environment"
Cohesion: 0.07
Nodes (30): mapReviewRating(), NewApp(), resolveAppDir(), resolveDBPath(), resolveNotebookDir(), llmProviderInterface, main(), notebookHandler() (+22 more)

### Community 8 - "Core Data Models"
Cohesion: 0.05
Nodes (39): Chunk, ChunkWithContext, CompletionResult, ExtractedSubtopic, Flashcard, FlashcardState, FSRSReviewLog, GeneratedFlashcard (+31 more)

### Community 10 - "Community 10"
Cohesion: 0.11
Nodes (36): activateTask(), appBridge(), askAI(), completeReading(), completeReadingSession(), confirmNotebookSyllabus(), deleteNotebook(), draftNotebookSyllabus() (+28 more)

### Community 11 - "Community 11"
Cohesion: 0.11
Nodes (27): Ask AI Endpoint, SeedDemoDataForTests(), ApplyHeuristicScoring(), BuildContext(), NewEmbeddingStore(), EmbeddingStore, Pipeline, buildPrompt() (+19 more)

### Community 12 - "Community 12"
Cohesion: 0.11
Nodes (28): buildTokenArrays(), destroyValues(), extractEmbedding(), extractIONames(), inferMaxSeqLen(), meanPool2D(), meanPool2DFloat64(), meanPool3D() (+20 more)

### Community 13 - "Community 13"
Cohesion: 0.12
Nodes (18): copyFile(), fileSHA256(), NewAssetValidator(), AssetValidator, Option, queryDailyStudyMinutesFn, queryDueReviewCardsFn, queryNextReadingTopicFn (+10 more)

### Community 14 - "Community 14"
Cohesion: 0.09
Nodes (15): ingestionProgressPayload, emitIngestionProgress(), UpdateNotebookTopic(), TopicBatchItem, TopicPageBoundsBatchItem, deleteAssessmentDataOutsideBoundsTx(), DeleteTopic(), GetAllTopics() (+7 more)

### Community 15 - "Community 15"
Cohesion: 0.12
Nodes (17): deleteNotebookRepo(), doesTableExistTxRepo(), ingestNotebookContentByTopicRepo(), insertChunkRowRepo(), insertParentRowRepo(), linkNotebookChunkRowRepo(), NotebookChunkInput, NotebookParentInput (+9 more)

### Community 16 - "Community 16"
Cohesion: 0.12
Nodes (17): Config, openAIMessage, openAIRequest, openAIResponse, Provider, firstEnv(), firstEnvInt(), LoadConfigFromEnv() (+9 more)

### Community 17 - "Community 17"
Cohesion: 0.1
Nodes (6): CompletionResult, NotebookTopicTreeNode, NotebookTopicTreeTopic, QuizAnswer, StudyQueueTask, SyllabusChapterDraft

### Community 18 - "Community 18"
Cohesion: 0.17
Nodes (16): TestParsePDFCPUBookmarkDraftFromJSON_EmptyPayload(), TestParsePDFCPUBookmarkDraftFromJSON_NestedPayload(), LLMProvider, extractPDFCPUBookmarkDraft(), findPDFCPUExecutable(), firstInt(), firstString(), ParsePDFCPUBookmarkDraftFromJSON() (+8 more)

### Community 19 - "Community 19"
Cohesion: 0.19
Nodes (16): App API Bridge, Base Button Component, Dashboard Page, Flashcards Page, Markdown Service, Notebooks Management, Quiz Page, Document Reader (+8 more)

### Community 20 - "Community 20"
Cohesion: 0.27
Nodes (13): clampFloat(), easyStabilityFactor(), estimateRetrievability(), firstReviewState(), goodStabilityFactor(), hardStabilityFactor(), maxInt(), NextFSRSState() (+5 more)

### Community 21 - "Community 21"
Cohesion: 0.39
Nodes (8): chunkVectorBatchItemRepo, UpsertChunkVectorsBatch(), isVectorUnavailableError(), lookupChunkRowIDRepo(), searchVectorsForTopicRepo(), upsertChunkVectorRepo(), upsertChunkVectorsBatchRepo(), vectorToJSONRepo()

### Community 22 - "Community 22"
Cohesion: 0.33
Nodes (1): AssessmentFSRSRecord

### Community 23 - "Community 23"
Cohesion: 0.33
Nodes (6): Sliding Window Chunking, onnxruntime_go, RAG Architecture, sqlite-vec, blocks table, block_vectors table

### Community 24 - "Community 24"
Cohesion: 0.5
Nodes (4): createWrittenQuestionRepo, createFlashcardsRepo, CreateNotebook, conn

### Community 25 - "Community 25"
Cohesion: 0.5
Nodes (4): Queue Router API, Core Queue System, Persistent Guided Study Queue, study_queue table

### Community 27 - "Community 27"
Cohesion: 0.67
Nodes (3): App Startup, Main Entry Point, Notebook Asset Handler

### Community 28 - "Community 28"
Cohesion: 0.67
Nodes (3): Engine, LLMProvider, StudyService

### Community 29 - "Community 29"
Cohesion: 0.67
Nodes (3): Persistent Queue Model, AGENTS.md, ARCHITECTURE.md

### Community 30 - "Community 30"
Cohesion: 0.67
Nodes (3): AGENT_MAP.md, buildPageBoundedContext, study

### Community 33 - "Community 33"
Cohesion: 1.0
Nodes (2): InitSchema, Init

### Community 34 - "Community 34"
Cohesion: 1.0
Nodes (2): Service, service

### Community 35 - "Community 35"
Cohesion: 1.0
Nodes (2): FSRS, NextFSRSState

### Community 36 - "Community 36"
Cohesion: 1.0
Nodes (2): Frontend (Vue), Vue 3 + Vite Frontend

### Community 37 - "Community 37"
Cohesion: 1.0
Nodes (2): Backend (Go + Wails), Go Backend

### Community 54 - "Community 54"
Cohesion: 1.0
Nodes (1): App Wails Bridge

### Community 55 - "Community 55"
Cohesion: 1.0
Nodes (1): Score Quiz Answer

### Community 56 - "Community 56"
Cohesion: 1.0
Nodes (1): Test App Factory

### Community 57 - "Community 57"
Cohesion: 1.0
Nodes (1): Local Path Upload

### Community 58 - "Community 58"
Cohesion: 1.0
Nodes (1): Draft Syllabus

### Community 59 - "Community 59"
Cohesion: 1.0
Nodes (1): Semantic Cache Checker

### Community 60 - "Community 60"
Cohesion: 1.0
Nodes (1): File Detector

### Community 61 - "Community 61"
Cohesion: 1.0
Nodes (1): AST Extractor

### Community 62 - "Community 62"
Cohesion: 1.0
Nodes (1): Vite Configuration

### Community 63 - "Community 63"
Cohesion: 1.0
Nodes (1): Main App Component

### Community 64 - "Community 64"
Cohesion: 1.0
Nodes (1): Windows Sync Script

### Community 65 - "Community 65"
Cohesion: 1.0
Nodes (1): Error Message Component

### Community 66 - "Community 66"
Cohesion: 1.0
Nodes (1): Loading Spinner Component

### Community 67 - "Community 67"
Cohesion: 1.0
Nodes (1): Tool Placeholder

### Community 68 - "Community 68"
Cohesion: 1.0
Nodes (1): Formatting Utilities

### Community 69 - "Community 69"
Cohesion: 1.0
Nodes (1): AssessmentFSRSRecord

### Community 70 - "Community 70"
Cohesion: 1.0
Nodes (1): getWrittenQuestionByIDRepo

### Community 71 - "Community 71"
Cohesion: 1.0
Nodes (1): getAssessmentFSRSStateFromQuerier

### Community 72 - "Community 72"
Cohesion: 1.0
Nodes (1): getAssessmentFSRSStateRepo

### Community 73 - "Community 73"
Cohesion: 1.0
Nodes (1): upsertAssessmentFSRSReviewRepo

### Community 74 - "Community 74"
Cohesion: 1.0
Nodes (1): loadExtension

### Community 75 - "Community 75"
Cohesion: 1.0
Nodes (1): getFlashcardsForTopicRepo

### Community 76 - "Community 76"
Cohesion: 1.0
Nodes (1): getFlashcardByIDRepo

### Community 77 - "Community 77"
Cohesion: 1.0
Nodes (1): updateFlashcardReviewRepo

### Community 78 - "Community 78"
Cohesion: 1.0
Nodes (1): IngestNotebookContent

### Community 79 - "Community 79"
Cohesion: 1.0
Nodes (1): GetNotebooks

### Community 80 - "Community 80"
Cohesion: 1.0
Nodes (1): createVectorTable

### Community 81 - "Community 81"
Cohesion: 1.0
Nodes (1): retrieval

### Community 82 - "Community 82"
Cohesion: 1.0
Nodes (1): SearchResult

### Community 83 - "Community 83"
Cohesion: 1.0
Nodes (1): runtime

### Community 84 - "Community 84"
Cohesion: 1.0
Nodes (1): AssetValidator

### Community 85 - "Community 85"
Cohesion: 1.0
Nodes (1): scheduler

### Community 86 - "Community 86"
Cohesion: 1.0
Nodes (1): scaledQuizQuestionCount

### Community 87 - "Community 87"
Cohesion: 1.0
Nodes (1): utils

### Community 88 - "Community 88"
Cohesion: 1.0
Nodes (1): APP_FLOW.md

### Community 89 - "Community 89"
Cohesion: 1.0
Nodes (1): The Academic Curator

### Community 90 - "Community 90"
Cohesion: 1.0
Nodes (1): Windows-First Target

### Community 91 - "Community 91"
Cohesion: 1.0
Nodes (1): fsrs_cards table

## Knowledge Gaps
- **140 isolated node(s):** `llmProviderInterface`, `ragPipelineInterface`, `ingestionProgressPayload`, `OpenAIRequest`, `OpenAIMessage` (+135 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **Thin community `Community 22`** (6 nodes): `AssessmentFSRSRecord`, `.GetDueAt()`, `.GetLastReviewedAt()`, `.GetSourceChunkID()`, `.GetState()`, `.GetTopicID()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 33`** (2 nodes): `InitSchema`, `Init`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 34`** (2 nodes): `Service`, `service`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 35`** (2 nodes): `FSRS`, `NextFSRSState`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 36`** (2 nodes): `Frontend (Vue)`, `Vue 3 + Vite Frontend`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 37`** (2 nodes): `Backend (Go + Wails)`, `Go Backend`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 54`** (1 nodes): `App Wails Bridge`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 55`** (1 nodes): `Score Quiz Answer`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 56`** (1 nodes): `Test App Factory`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 57`** (1 nodes): `Local Path Upload`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 58`** (1 nodes): `Draft Syllabus`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 59`** (1 nodes): `Semantic Cache Checker`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 60`** (1 nodes): `File Detector`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 61`** (1 nodes): `AST Extractor`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 62`** (1 nodes): `Vite Configuration`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 63`** (1 nodes): `Main App Component`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 64`** (1 nodes): `Windows Sync Script`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 65`** (1 nodes): `Error Message Component`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 66`** (1 nodes): `Loading Spinner Component`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 67`** (1 nodes): `Tool Placeholder`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 68`** (1 nodes): `Formatting Utilities`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 69`** (1 nodes): `AssessmentFSRSRecord`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 70`** (1 nodes): `getWrittenQuestionByIDRepo`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 71`** (1 nodes): `getAssessmentFSRSStateFromQuerier`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 72`** (1 nodes): `getAssessmentFSRSStateRepo`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 73`** (1 nodes): `upsertAssessmentFSRSReviewRepo`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 74`** (1 nodes): `loadExtension`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 75`** (1 nodes): `getFlashcardsForTopicRepo`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 76`** (1 nodes): `getFlashcardByIDRepo`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 77`** (1 nodes): `updateFlashcardReviewRepo`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 78`** (1 nodes): `IngestNotebookContent`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 79`** (1 nodes): `GetNotebooks`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 80`** (1 nodes): `createVectorTable`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 81`** (1 nodes): `retrieval`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 82`** (1 nodes): `SearchResult`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 83`** (1 nodes): `runtime`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 84`** (1 nodes): `AssetValidator`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 85`** (1 nodes): `scheduler`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 86`** (1 nodes): `scaledQuizQuestionCount`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 87`** (1 nodes): `utils`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 88`** (1 nodes): `APP_FLOW.md`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 89`** (1 nodes): `The Academic Curator`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 90`** (1 nodes): `Windows-First Target`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 91`** (1 nodes): `fsrs_cards table`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Errorf()` connect `Flashcards & Quiz Service` to `Core App Logic (Go Bridge)`, `Database & Test Utilities`, `Notebook & Topic Management`, `SRS & Review Logic (FSRS)`, `Text Processing & Ingestion`, `Application Setup & Environment`, `Community 11`, `Community 12`, `Community 13`, `Community 14`, `Community 15`, `Community 16`, `Community 18`, `Community 21`?**
  _High betweenness centrality (0.355) - this node is a cross-community bridge._
- **Why does `EventsEmit()` connect `Community 14` to `Wails Frontend Runtime`, `Application Setup & Environment`?**
  _High betweenness centrality (0.083) - this node is a cross-community bridge._
- **Why does `App` connect `Core App Logic (Go Bridge)` to `Flashcards & Quiz Service`, `Notebook & Topic Management`, `SRS & Review Logic (FSRS)`, `Application Setup & Environment`, `Community 14`, `Community 15`?**
  _High betweenness centrality (0.069) - this node is a cross-community bridge._
- **Are the 133 inferred relationships involving `Errorf()` (e.g. with `.startup()` and `createWrittenQuestionRepo()`) actually correct?**
  _`Errorf()` has 133 INFERRED edges - model-reasoned connections that need verification._
- **Are the 48 inferred relationships involving `EnsureTopic()` (e.g. with `TestGetNotebookTopicTreeReturnsNestedTopics()` and `TestScoreAnswerCorrectAnswerFullText()`) actually correct?**
  _`EnsureTopic()` has 48 INFERRED edges - model-reasoned connections that need verification._
- **Are the 39 inferred relationships involving `initDBForTest()` (e.g. with `TestUpdateFlashcardReviewTransactionalSave()` and `TestUpdateFlashcardReviewRollsBackCardOnLogInsertFailure()`) actually correct?**
  _`initDBForTest()` has 39 INFERRED edges - model-reasoned connections that need verification._
- **Are the 7 inferred relationships involving `newTestApp()` (e.g. with `New()` and `NewService()`) actually correct?**
  _`newTestApp()` has 7 INFERRED edges - model-reasoned connections that need verification._