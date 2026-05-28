# Graph Report - ai-tutor  (2026-05-28)

## Corpus Check
- 95 files · ~134,770 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 971 nodes · 2062 edges · 21 communities detected
- Extraction: 58% EXTRACTED · 42% INFERRED · 0% AMBIGUOUS · INFERRED: 873 edges (avg confidence: 0.8)
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

## God Nodes (most connected - your core abstractions)
1. `Errorf()` - 180 edges
2. `initDBForTest()` - 51 edges
3. `EnsureTopic()` - 51 edges
4. `App` - 45 edges
5. `appBridge()` - 37 edges
6. `CreateNotebook()` - 36 edges
7. `Warnf()` - 31 edges
8. `newTestApp()` - 30 edges
9. `contains()` - 25 edges
10. `StudyService` - 21 edges

## Surprising Connections (you probably didn't know these)
- `CountFlashcardsForTopic()` --calls--> `countFlashcardsForTopicRepo()`  [INFERRED]
  internal\db\store.go → internal\db\flashcard_repo.go
- `TestIngestNotebookContentByTopicRollsBackOnMidTransactionFailure()` --calls--> `UpdateNotebookChunkCount()`  [INFERRED]
  internal\db\store_integration_test.go → internal\db\notebooks_repo.go
- `GetChunkTextByNotebookPageRange()` --calls--> `Errorf()`  [INFERRED]
  internal\db\notebooks_repo.go → internal\utils\logging.go
- `GetNotebookPageCount()` --calls--> `Errorf()`  [INFERRED]
  internal\db\notebooks_repo.go → internal\utils\logging.go
- `CreateReviewSession()` --calls--> `reviewSessionNow()`  [INFERRED]
  internal\db\store.go → internal\db\review_session_repo.go

## Communities

### Community 0 - "Community 0"
Cohesion: 0.03
Nodes (81): createWrittenQuestionRepo(), getAssessmentFSRSStateFromQuerier(), getAssessmentFSRSStateRepo(), getAssessmentFSRSStateRepoTx(), getWrittenQuestionByIDRepo(), upsertAssessmentFSRSReviewRepo(), upsertAssessmentFSRSReviewRepoTx(), loadExtension() (+73 more)

### Community 1 - "Community 1"
Cohesion: 0.03
Nodes (45): App, NotebookChunkInput, NotebookParentInput, DeleteNotebook(), deleteNotebookRepo(), doesTableExistTxRepo(), GetChunkTextByNotebookPageRange(), GetNotebookByID() (+37 more)

### Community 2 - "Community 2"
Cohesion: 0.06
Nodes (68): extractFirstChunkID(), extractRequestedCount(), flashcardJSON(), initTestDB(), initTestProvider(), mustInsertActiveQuizTask(), newTestApp(), questionJSON() (+60 more)

### Community 3 - "Community 3"
Cohesion: 0.08
Nodes (76): TestAskReaderAI_ScopedResponseShape(), TestExplainReaderSection_EmptyQuestion(), TestExplainReaderSection_Success(), TestGetNotebookTopicTreeReturnsNestedTopics(), TestGetReaderTopicBundle_Success(), ChunkEmbeddingBatchItem, ChunkVectorBatchItem, TestGetNextDueReviewNotebookUsesPriorityAndLegacyTopicLink() (+68 more)

### Community 4 - "Community 4"
Cohesion: 0.03
Nodes (8): GenerateFlashcardsForQuizTask(), GetTodayPlan(), UpdateDailyStudyMinutes(), loadAgenda(), generateManualQuiz(), handleContinue(), submitQuiz(), saveSettings()

### Community 5 - "Community 5"
Cohesion: 0.04
Nodes (52): Chunk, ChunkWithContext, CompletionResult, ExtractedSubtopic, Flashcard, FlashcardState, FSRSReviewLog, GeneratedFlashcard (+44 more)

### Community 6 - "Community 6"
Cohesion: 0.04
Nodes (3): EventsOn(), EventsOnce(), EventsOnMultiple()

### Community 7 - "Community 7"
Cohesion: 0.08
Nodes (35): initCleanTestDB(), TestGetNotebookTopicTreeEmptyReturnsArray(), loadExtension(), InitSchema(), Init(), SeedDemoDataForTests(), buildTokenArrays(), destroyValues() (+27 more)

### Community 8 - "Community 8"
Cohesion: 0.08
Nodes (26): EnsureNotebookTopic(), GetChunksWithContextByNotebookPageRange(), EnsureTopicsBatch(), Config, buildComprehensiveExamPrompt(), buildMarathonFlashcardPromptWithBudget(), flashcardLLMCard, flashcardLLMResponse (+18 more)

### Community 9 - "Community 9"
Cohesion: 0.1
Nodes (30): initTestPipeline(), TestAskAIInvalidTopicReturnsError(), TestAskAIResponseShape(), GetChunksForTopic(), NewProvider(), ApplyHeuristicScoring(), BuildContext(), NewEmbeddingStore() (+22 more)

### Community 10 - "Community 10"
Cohesion: 0.1
Nodes (37): activateTask(), appBridge(), askAI(), askReaderAI(), completeReading(), completeReviewSession(), confirmNotebookSyllabus(), deleteNotebook() (+29 more)

### Community 11 - "Community 11"
Cohesion: 0.09
Nodes (28): ExtractedDocument, ExtractedSection, FileMetadata, BuildTopicGroupsFromChapters(), chapterIndexForPage(), markdownSection, Option, topicGroupBuilder (+20 more)

### Community 12 - "Community 12"
Cohesion: 0.09
Nodes (24): ingestionProgressPayload, emitIngestionProgress(), chunkVectorBatchItemRepo, UpdateNotebookIndexingStatus(), Close(), GetChunkEmbeddingRefsForTopic(), UpdateChunkEmbedding(), UpdateChunkEmbeddingsBatch() (+16 more)

### Community 13 - "Community 13"
Cohesion: 0.1
Nodes (17): TestNotebookAssetURLRejectsTraversalNames(), TestNotebookAssetURLUsesBasename(), NewApp(), notebookAssetURL(), queueTaskToScheduledTask(), resolveAppDir(), resolveDBPath(), resolveNotebookDir() (+9 more)

### Community 14 - "Community 14"
Cohesion: 0.12
Nodes (26): completeReviewSessionRepo(), createReviewSessionRepo(), fetchExistingReviewTask(), getDueReviewCardCountsByNotebookRepo(), getDueReviewCardsForNotebookRepo(), getExistingReviewTaskForNotebookRepo(), getExistingReviewTaskForNotebookTxRepo(), getTaskByIDTxRepo() (+18 more)

### Community 15 - "Community 15"
Cohesion: 0.11
Nodes (19): Config, ModelLimits, openAIMessage, openAIRequest, openAIResponse, Provider, firstEnv(), firstEnvInt() (+11 more)

### Community 16 - "Community 16"
Cohesion: 0.12
Nodes (13): GetParentSection(), SearchVectorsForNotebook(), Engine, cosineSimilarity(), NewEngine(), tokenize(), Scope, SearchResult (+5 more)

### Community 17 - "Community 17"
Cohesion: 0.2
Nodes (18): Option, queryDailyStudyMinutesFn, queryDueReviewCardsFn, queryNextDueReviewNotebookFn, queryNextReadingTopicFn, queryTokensPerPageMapFn, service, New() (+10 more)

### Community 18 - "Community 18"
Cohesion: 0.1
Nodes (6): CompletionResult, NotebookTopicTreeNode, NotebookTopicTreeTopic, QuizAnswer, StudyQueueTask, SyllabusChapterDraft

### Community 19 - "Community 19"
Cohesion: 0.17
Nodes (16): TestParsePDFCPUBookmarkDraftFromJSON_EmptyPayload(), TestParsePDFCPUBookmarkDraftFromJSON_NestedPayload(), LLMProvider, extractPDFCPUBookmarkDraft(), findPDFCPUExecutable(), firstInt(), firstString(), ParsePDFCPUBookmarkDraftFromJSON() (+8 more)

### Community 20 - "Community 20"
Cohesion: 0.33
Nodes (1): AssessmentFSRSRecord

## Knowledge Gaps
- **97 isolated node(s):** `llmProviderInterface`, `ragPipelineInterface`, `ingestionProgressPayload`, `OpenAIRequest`, `OpenAIMessage` (+92 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **Thin community `Community 20`** (6 nodes): `AssessmentFSRSRecord`, `.GetDueAt()`, `.GetLastReviewedAt()`, `.GetSourceChunkID()`, `.GetState()`, `.GetTopicID()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Errorf()` connect `Community 0` to `Community 1`, `Community 2`, `Community 3`, `Community 5`, `Community 7`, `Community 8`, `Community 9`, `Community 11`, `Community 12`, `Community 13`, `Community 14`, `Community 15`, `Community 16`, `Community 17`, `Community 19`?**
  _High betweenness centrality (0.519) - this node is a cross-community bridge._
- **Why does `EventsEmit()` connect `Community 12` to `Community 6`?**
  _High betweenness centrality (0.094) - this node is a cross-community bridge._
- **Why does `GetNotebooks()` connect `Community 1` to `Community 4`, `Community 12`?**
  _High betweenness centrality (0.090) - this node is a cross-community bridge._
- **Are the 178 inferred relationships involving `Errorf()` (e.g. with `.startup()` and `resolveAppDir()`) actually correct?**
  _`Errorf()` has 178 INFERRED edges - model-reasoned connections that need verification._
- **Are the 49 inferred relationships involving `initDBForTest()` (e.g. with `TestUpdateFlashcardReviewTransactionalSave()` and `TestUpdateFlashcardReviewRollsBackCardOnLogInsertFailure()`) actually correct?**
  _`initDBForTest()` has 49 INFERRED edges - model-reasoned connections that need verification._
- **Are the 50 inferred relationships involving `EnsureTopic()` (e.g. with `mustInsertActiveQuizTask()` and `TestGetNotebookTopicTreeReturnsNestedTopics()`) actually correct?**
  _`EnsureTopic()` has 50 INFERRED edges - model-reasoned connections that need verification._
- **What connects `llmProviderInterface`, `ragPipelineInterface`, `ingestionProgressPayload` to the rest of the system?**
  _97 weakly-connected nodes found - possible documentation gaps or missing edges._