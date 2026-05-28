# Graph Report - ai-tutor  (2026-05-27)

## Corpus Check
- 95 files · ~135,896 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1046 nodes · 2206 edges · 41 communities detected
- Extraction: 58% EXTRACTED · 42% INFERRED · 0% AMBIGUOUS · INFERRED: 926 edges (avg confidence: 0.8)
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
- [[_COMMUNITY_Community 22|Community 22]]
- [[_COMMUNITY_Community 24|Community 24]]
- [[_COMMUNITY_Community 25|Community 25]]
- [[_COMMUNITY_Community 26|Community 26]]
- [[_COMMUNITY_Community 31|Community 31]]
- [[_COMMUNITY_Community 32|Community 32]]
- [[_COMMUNITY_Community 33|Community 33]]
- [[_COMMUNITY_Community 34|Community 34]]
- [[_COMMUNITY_Community 48|Community 48]]
- [[_COMMUNITY_Community 49|Community 49]]
- [[_COMMUNITY_Community 50|Community 50]]
- [[_COMMUNITY_Community 51|Community 51]]
- [[_COMMUNITY_Community 52|Community 52]]
- [[_COMMUNITY_Community 53|Community 53]]
- [[_COMMUNITY_Community 54|Community 54]]
- [[_COMMUNITY_Community 55|Community 55]]
- [[_COMMUNITY_Community 56|Community 56]]
- [[_COMMUNITY_Community 57|Community 57]]
- [[_COMMUNITY_Community 58|Community 58]]

## God Nodes (most connected - your core abstractions)
1. `Errorf()` - 183 edges
2. `EnsureTopic()` - 60 edges
3. `App` - 51 edges
4. `initDBForTest()` - 51 edges
5. `newTestApp()` - 44 edges
6. `appBridge()` - 42 edges
7. `CreateNotebook()` - 38 edges
8. `Warnf()` - 33 edges
9. `contains()` - 25 edges
10. `StudyService` - 23 edges

## Surprising Connections (you probably didn't know these)
- `UpdateNotebookChunkCount()` --calls--> `TestIngestNotebookContentByTopicRollsBackOnMidTransactionFailure()`  [INFERRED]
  internal\db\notebooks_repo.go → internal\db\store_integration_test.go
- `GetChunkTextByNotebookPageRange()` --calls--> `Errorf()`  [INFERRED]
  internal\db\notebooks_repo.go → internal\utils\logging.go
- `GetNotebookPageCount()` --calls--> `Errorf()`  [INFERRED]
  internal\db\notebooks_repo.go → internal\utils\logging.go
- `GetTokensPerPageMap()` --calls--> `Errorf()`  [INFERRED]
  internal\db\reader_repo.go → internal\utils\logging.go
- `TestSchemaIncludesRereadAttemptsTable()` --calls--> `initDBForTest()`  [INFERRED]
  internal\db\study_queue_repo_test.go → internal\db\test_helpers_test.go

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
Nodes (93): TestGenerateShortAnswerPrompt_Success(), TestScoreShortAnswerLoadsPersistedPromptAndUpdatesFSRS(), createWrittenQuestionRepo(), getAssessmentFSRSStateFromQuerier(), getAssessmentFSRSStateRepo(), getAssessmentFSRSStateRepoTx(), getWrittenQuestionByIDRepo(), upsertAssessmentFSRSReviewRepo() (+85 more)

### Community 1 - "Community 1"
Cohesion: 0.06
Nodes (83): extractFirstChunkID(), extractRequestedCount(), flashcardJSON(), initTestDB(), initTestPipeline(), initTestProvider(), mustInsertActiveQuizTask(), newTestApp() (+75 more)

### Community 2 - "Community 2"
Cohesion: 0.03
Nodes (37): App, NotebookChunkInput, NotebookParentInput, DeleteNotebook(), deleteNotebookRepo(), doesTableExistTxRepo(), GetChunkTextByNotebookPageRange(), GetNotebookByID() (+29 more)

### Community 3 - "Community 3"
Cohesion: 0.03
Nodes (8): GenerateFlashcardsForQuizTask(), GetTodayPlan(), UpdateDailyStudyMinutes(), loadAgenda(), generateManualQuiz(), handleContinue(), submitQuiz(), saveSettings()

### Community 4 - "Community 4"
Cohesion: 0.09
Nodes (63): TestAskReaderAI_ScopedResponseShape(), TestExplainReaderSection_EmptyQuestion(), TestExplainReaderSection_Success(), TestGetNotebookTopicTreeReturnsNestedTopics(), TestGetReaderTopicBundle_Success(), TestGetNextDueReviewNotebookUsesPriorityAndLegacyTopicLink(), TestQueryDueReviewCardsIgnoresOrphanedCards(), TestQueryDueReviewCardsIgnoresSuspendedCards() (+55 more)

### Community 5 - "Community 5"
Cohesion: 0.06
Nodes (39): TestGenerateFlashcardsCreatesAndReturnsCards(), TestGenerateFlashcardsReturnsExistingCardsWithoutDuplication(), countFlashcardsForTopicRepo(), EnsureNotebookTopic(), GetChunksWithContextByNotebookPageRange(), GetTotalChunkTokens(), GetTotalChunkTokensForPageRange(), CountFlashcardsForTopic() (+31 more)

### Community 6 - "Community 6"
Cohesion: 0.04
Nodes (3): EventsOn(), EventsOnce(), EventsOnMultiple()

### Community 7 - "Community 7"
Cohesion: 0.04
Nodes (46): Chunk, ChunkWithContext, CompletionResult, ExtractedSubtopic, Flashcard, FlashcardState, FSRSReviewLog, GeneratedFlashcard (+38 more)

### Community 8 - "Community 8"
Cohesion: 0.07
Nodes (32): NormalizeWhitespace(), ExtractedDocument, ExtractedSection, FileMetadata, BuildTopicGroupsFromChapters(), chapterIndexForPage(), markdownSection, Option (+24 more)

### Community 9 - "Community 9"
Cohesion: 0.09
Nodes (42): activateTask(), appBridge(), askAI(), askReaderAI(), completeReading(), completeReadingSession(), completeReviewSession(), confirmNotebookSyllabus() (+34 more)

### Community 10 - "Community 10"
Cohesion: 0.1
Nodes (29): completeReviewSessionRepo(), createReviewSessionRepo(), fetchExistingReviewTask(), getDueReviewCardsForNotebookRepo(), getExistingReviewTaskForNotebookRepo(), getExistingReviewTaskForNotebookTxRepo(), getTaskByIDTxRepo(), remainingReviewTaskCardsTxRepo() (+21 more)

### Community 11 - "Community 11"
Cohesion: 0.1
Nodes (30): initCleanTestDB(), TestGetNotebookTopicTreeEmptyReturnsArray(), buildTokenArrays(), destroyValues(), extractEmbedding(), extractIONames(), inferMaxSeqLen(), meanPool2D() (+22 more)

### Community 12 - "Community 12"
Cohesion: 0.08
Nodes (24): GetChunksForTopic(), GetChunksForTopicPageRange(), GetChunksForTopics(), GetChunkTextsForTopicPageRange(), GetFirstNotebookIDByTopicID(), GetParentPassagesForTopicPageRange(), GetParentSection(), GetTokensPerPageMap() (+16 more)

### Community 13 - "Community 13"
Cohesion: 0.11
Nodes (26): SeedDemoDataForTests(), ApplyHeuristicScoring(), BuildContext(), NewEmbeddingStore(), EmbeddingStore, Pipeline, buildPrompt(), countPromptTokens() (+18 more)

### Community 14 - "Community 14"
Cohesion: 0.08
Nodes (19): TestNotebookAssetURLRejectsTraversalNames(), TestNotebookAssetURLUsesBasename(), mapReviewRating(), NewApp(), normalizeQuizAnswer(), notebookAssetURL(), queueTaskToScheduledTask(), resolveAppDir() (+11 more)

### Community 15 - "Community 15"
Cohesion: 0.11
Nodes (19): Config, ModelLimits, openAIMessage, openAIRequest, openAIResponse, Provider, firstEnv(), firstEnvInt() (+11 more)

### Community 16 - "Community 16"
Cohesion: 0.2
Nodes (18): Option, queryDailyStudyMinutesFn, queryDueReviewCardsFn, queryNextDueReviewNotebookFn, queryNextReadingTopicFn, queryTokensPerPageMapFn, service, New() (+10 more)

### Community 17 - "Community 17"
Cohesion: 0.1
Nodes (6): CompletionResult, NotebookTopicTreeNode, NotebookTopicTreeTopic, QuizAnswer, StudyQueueTask, SyllabusChapterDraft

### Community 18 - "Community 18"
Cohesion: 0.17
Nodes (16): TestParsePDFCPUBookmarkDraftFromJSON_EmptyPayload(), TestParsePDFCPUBookmarkDraftFromJSON_NestedPayload(), LLMProvider, extractPDFCPUBookmarkDraft(), findPDFCPUExecutable(), firstInt(), firstString(), ParsePDFCPUBookmarkDraftFromJSON() (+8 more)

### Community 19 - "Community 19"
Cohesion: 0.15
Nodes (11): ingestionProgressPayload, emitIngestionProgress(), UpdateNotebookIndexingStatus(), UpdateChunkEmbedding(), computeTextHash(), doesHashMatch(), NewVectorIndexer(), IndexerConfig (+3 more)

### Community 20 - "Community 20"
Cohesion: 0.27
Nodes (13): clampFloat(), easyStabilityFactor(), estimateRetrievability(), firstReviewState(), goodStabilityFactor(), hardStabilityFactor(), maxInt(), NextFSRSState() (+5 more)

### Community 21 - "Community 21"
Cohesion: 0.33
Nodes (6): Sliding Window Chunking, onnxruntime_go, RAG Architecture, sqlite-vec, blocks table, block_vectors table

### Community 22 - "Community 22"
Cohesion: 0.5
Nodes (4): Queue Router API, Core Queue System, Persistent Guided Study Queue, study_queue table

### Community 24 - "Community 24"
Cohesion: 0.67
Nodes (3): Engine, LLMProvider, StudyService

### Community 25 - "Community 25"
Cohesion: 0.67
Nodes (3): Persistent Queue Model, AGENTS.md, ARCHITECTURE.md

### Community 26 - "Community 26"
Cohesion: 0.67
Nodes (3): AGENT_MAP.md, buildPageBoundedContext, study

### Community 31 - "Community 31"
Cohesion: 1.0
Nodes (2): Service, service

### Community 32 - "Community 32"
Cohesion: 1.0
Nodes (2): FSRS, NextFSRSState

### Community 33 - "Community 33"
Cohesion: 1.0
Nodes (2): Frontend (Vue), Vue 3 + Vite Frontend

### Community 34 - "Community 34"
Cohesion: 1.0
Nodes (2): Backend (Go + Wails), Go Backend

### Community 48 - "Community 48"
Cohesion: 1.0
Nodes (1): retrieval

### Community 49 - "Community 49"
Cohesion: 1.0
Nodes (1): SearchResult

### Community 50 - "Community 50"
Cohesion: 1.0
Nodes (1): runtime

### Community 51 - "Community 51"
Cohesion: 1.0
Nodes (1): AssetValidator

### Community 52 - "Community 52"
Cohesion: 1.0
Nodes (1): scheduler

### Community 53 - "Community 53"
Cohesion: 1.0
Nodes (1): scaledQuizQuestionCount

### Community 54 - "Community 54"
Cohesion: 1.0
Nodes (1): utils

### Community 55 - "Community 55"
Cohesion: 1.0
Nodes (1): APP_FLOW.md

### Community 56 - "Community 56"
Cohesion: 1.0
Nodes (1): The Academic Curator

### Community 57 - "Community 57"
Cohesion: 1.0
Nodes (1): Windows-First Target

### Community 58 - "Community 58"
Cohesion: 1.0
Nodes (1): fsrs_cards table

## Knowledge Gaps
- **110 isolated node(s):** `llmProviderInterface`, `ragPipelineInterface`, `ingestionProgressPayload`, `OpenAIRequest`, `OpenAIMessage` (+105 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **Thin community `Community 31`** (2 nodes): `Service`, `service`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 32`** (2 nodes): `FSRS`, `NextFSRSState`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 33`** (2 nodes): `Frontend (Vue)`, `Vue 3 + Vite Frontend`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 34`** (2 nodes): `Backend (Go + Wails)`, `Go Backend`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 48`** (1 nodes): `retrieval`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 49`** (1 nodes): `SearchResult`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 50`** (1 nodes): `runtime`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 51`** (1 nodes): `AssetValidator`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 52`** (1 nodes): `scheduler`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 53`** (1 nodes): `scaledQuizQuestionCount`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 54`** (1 nodes): `utils`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 55`** (1 nodes): `APP_FLOW.md`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 56`** (1 nodes): `The Academic Curator`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 57`** (1 nodes): `Windows-First Target`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 58`** (1 nodes): `fsrs_cards table`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Errorf()` connect `Community 0` to `Community 1`, `Community 2`, `Community 4`, `Community 5`, `Community 7`, `Community 8`, `Community 10`, `Community 11`, `Community 12`, `Community 13`, `Community 14`, `Community 15`, `Community 16`, `Community 18`, `Community 19`?**
  _High betweenness centrality (0.458) - this node is a cross-community bridge._
- **Why does `EventsEmit()` connect `Community 19` to `Community 6`?**
  _High betweenness centrality (0.106) - this node is a cross-community bridge._
- **Why does `loadAgenda()` connect `Community 3` to `Community 2`?**
  _High betweenness centrality (0.089) - this node is a cross-community bridge._
- **Are the 181 inferred relationships involving `Errorf()` (e.g. with `.startup()` and `resolveAppDir()`) actually correct?**
  _`Errorf()` has 181 INFERRED edges - model-reasoned connections that need verification._
- **Are the 59 inferred relationships involving `EnsureTopic()` (e.g. with `mustInsertActiveQuizTask()` and `TestGetNotebookTopicTreeReturnsNestedTopics()`) actually correct?**
  _`EnsureTopic()` has 59 INFERRED edges - model-reasoned connections that need verification._
- **Are the 49 inferred relationships involving `initDBForTest()` (e.g. with `TestUpdateFlashcardReviewTransactionalSave()` and `TestUpdateFlashcardReviewRollsBackCardOnLogInsertFailure()`) actually correct?**
  _`initDBForTest()` has 49 INFERRED edges - model-reasoned connections that need verification._
- **Are the 7 inferred relationships involving `newTestApp()` (e.g. with `New()` and `NewService()`) actually correct?**
  _`newTestApp()` has 7 INFERRED edges - model-reasoned connections that need verification._