# Graph Report - ai-tutor  (2026-06-05)

## Corpus Check
- 93 files · ~130,872 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 947 nodes · 2046 edges · 21 communities detected
- Extraction: 57% EXTRACTED · 43% INFERRED · 0% AMBIGUOUS · INFERRED: 884 edges (avg confidence: 0.8)
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
1. `Errorf()` - 173 edges
2. `initDBForTest()` - 51 edges
3. `EnsureTopic()` - 48 edges
4. `App` - 38 edges
5. `CreateNotebook()` - 36 edges
6. `appBridge()` - 30 edges
7. `Warnf()` - 30 edges
8. `contains()` - 27 edges
9. `newTestApp()` - 24 edges
10. `withTx()` - 24 edges

## Surprising Connections (you probably didn't know these)
- `TestIngestNotebookContentByTopicRollsBackOnMidTransactionFailure()` --calls--> `UpdateNotebookChunkCount()`  [INFERRED]
  internal\db\store_integration_test.go → internal\db\notebooks_repo.go
- `GetChunkTextByNotebookPageRange()` --calls--> `Errorf()`  [INFERRED]
  internal\db\notebooks_repo.go → internal\utils\logging.go
- `GetNotebookPageCount()` --calls--> `Errorf()`  [INFERRED]
  internal\db\notebooks_repo.go → internal\utils\logging.go
- `GetTopicCurrentPageCursor()` --calls--> `Errorf()`  [INFERRED]
  internal\db\topics_repo.go → internal\utils\logging.go
- `QueryNextReadingTopic()` --calls--> `Warnf()`  [INFERRED]
  internal\db\topics_repo.go → internal\utils\logging.go

## Communities

### Community 0 - "Community 0"
Cohesion: 0.03
Nodes (106): getAssessmentFSRSStateFromQuerier(), getAssessmentFSRSStateRepo(), getAssessmentFSRSStateRepoTx(), getWrittenQuestionByIDRepo(), upsertAssessmentFSRSReviewRepo(), upsertAssessmentFSRSReviewRepoTx(), ChunkEmbeddingBatchItem, ChunkVectorBatchItem (+98 more)

### Community 1 - "Community 1"
Cohesion: 0.08
Nodes (79): TestAskReaderAI_ScopedResponseShape(), TestGetNotebookTopicTreeReturnsNestedTopics(), TestGetReaderTopicBundle_Success(), TestReviewSessionEndpointsSupportGenerationRecoveryAndCompletion(), TestGetNextDueReviewNotebookUsesPriorityAndLegacyTopicLink(), TestQueryDueReviewCardsIgnoresOrphanedCards(), TestQueryDueReviewCardsIgnoresSuspendedCards(), TestUpdateFlashcardReviewRollsBackCardOnLogInsertFailure() (+71 more)

### Community 2 - "Community 2"
Cohesion: 0.05
Nodes (34): App, queueTaskToScheduledTask(), GetNotebooks(), UpdateNotebookPriority(), UpdateNotebookStatus(), UpdateNotebookTitle(), GetReaderTopicBundle(), GetDailyStudyMinutes() (+26 more)

### Community 3 - "Community 3"
Cohesion: 0.05
Nodes (40): createWrittenQuestionRepo(), EnsureNotebookTopic(), GetChunksWithContextByNotebookPageRange(), GetParentSection(), CreateWrittenQuestion(), EnsureTopicsBatch(), generate(), loadQueueSession() (+32 more)

### Community 4 - "Community 4"
Cohesion: 0.03
Nodes (4): GenerateFlashcardsForQuizTask(), generateManualQuiz(), handleContinue(), submitQuiz()

### Community 5 - "Community 5"
Cohesion: 0.04
Nodes (52): Chunk, ChunkWithContext, CompletionResult, ExtractedSubtopic, Flashcard, FlashcardState, FSRSReviewLog, GeneratedFlashcard (+44 more)

### Community 6 - "Community 6"
Cohesion: 0.08
Nodes (49): extractFirstChunkID(), extractRequestedCount(), flashcardJSON(), initCleanTestDB(), initTestDB(), initTestPipeline(), initTestProvider(), mustInsertActiveQuizTask() (+41 more)

### Community 7 - "Community 7"
Cohesion: 0.04
Nodes (3): EventsOn(), EventsOnce(), EventsOnMultiple()

### Community 8 - "Community 8"
Cohesion: 0.07
Nodes (32): NormalizeWhitespace(), ExtractedDocument, ExtractedSection, FileMetadata, BuildTopicGroupsFromChapters(), chapterIndexForPage(), markdownSection, Option (+24 more)

### Community 9 - "Community 9"
Cohesion: 0.06
Nodes (35): insertFSRSReviewLogRepo(), NotebookChunkInput, NotebookParentInput, DeleteNotebook(), deleteNotebookRepo(), doesTableExistTxRepo(), GetChunkTextByNotebookPageRange(), GetNotebookByID() (+27 more)

### Community 10 - "Community 10"
Cohesion: 0.11
Nodes (27): GetTopicContent(), SeedDemoDataForTests(), ApplyHeuristicScoring(), BuildContext(), NewEmbeddingStore(), EmbeddingStore, Pipeline, buildPrompt() (+19 more)

### Community 11 - "Community 11"
Cohesion: 0.11
Nodes (28): buildTokenArrays(), destroyValues(), extractEmbedding(), extractIONames(), inferMaxSeqLen(), meanPool2D(), meanPool2DFloat64(), meanPool3D() (+20 more)

### Community 12 - "Community 12"
Cohesion: 0.12
Nodes (24): Engine, cosineSimilarity(), NewEngine(), tokenize(), Scope, SearchResult, Option, queryDailyStudyMinutesFn (+16 more)

### Community 13 - "Community 13"
Cohesion: 0.13
Nodes (30): activateTask(), appBridge(), askReaderAI(), askSocratic(), completeReading(), completeReviewSession(), confirmNotebookSyllabus(), deleteNotebook() (+22 more)

### Community 14 - "Community 14"
Cohesion: 0.1
Nodes (19): NewApp(), resolveAppDir(), resolveDBPath(), resolveNotebookDir(), ingestionProgressPayload, llmProviderInterface, main(), notebookHandler() (+11 more)

### Community 15 - "Community 15"
Cohesion: 0.11
Nodes (19): Config, ModelLimits, openAIMessage, openAIRequest, openAIResponse, Provider, firstEnv(), firstEnvInt() (+11 more)

### Community 16 - "Community 16"
Cohesion: 0.1
Nodes (6): CompletionResult, NotebookTopicTreeNode, NotebookTopicTreeTopic, QuizAnswer, StudyQueueTask, SyllabusChapterDraft

### Community 17 - "Community 17"
Cohesion: 0.17
Nodes (16): TestParsePDFCPUBookmarkDraftFromJSON_EmptyPayload(), TestParsePDFCPUBookmarkDraftFromJSON_NestedPayload(), LLMProvider, extractPDFCPUBookmarkDraft(), findPDFCPUExecutable(), firstInt(), firstString(), ParsePDFCPUBookmarkDraftFromJSON() (+8 more)

### Community 18 - "Community 18"
Cohesion: 0.24
Nodes (13): analyzeDuplicates(), analyzeFile(), countUniqueFiles(), findSimilarities(), main(), printDuplicateFunctions(), printDuplicateStructs(), printRecommendations() (+5 more)

### Community 19 - "Community 19"
Cohesion: 0.27
Nodes (4): copyFile(), fileSHA256(), NewAssetValidator(), AssetValidator

### Community 20 - "Community 20"
Cohesion: 0.33
Nodes (1): AssessmentFSRSRecord

## Knowledge Gaps
- **99 isolated node(s):** `llmProviderInterface`, `ingestionProgressPayload`, `OpenAIRequest`, `OpenAIMessage`, `OpenAIResponse` (+94 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **Thin community `Community 20`** (6 nodes): `AssessmentFSRSRecord`, `.GetDueAt()`, `.GetLastReviewedAt()`, `.GetSourceChunkID()`, `.GetState()`, `.GetTopicID()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Errorf()` connect `Community 0` to `Community 1`, `Community 2`, `Community 3`, `Community 5`, `Community 6`, `Community 8`, `Community 9`, `Community 10`, `Community 11`, `Community 12`, `Community 14`, `Community 15`, `Community 17`, `Community 19`?**
  _High betweenness centrality (0.540) - this node is a cross-community bridge._
- **Why does `EventsEmit()` connect `Community 14` to `Community 7`?**
  _High betweenness centrality (0.098) - this node is a cross-community bridge._
- **Are the 171 inferred relationships involving `Errorf()` (e.g. with `.startup()` and `resolveAppDir()`) actually correct?**
  _`Errorf()` has 171 INFERRED edges - model-reasoned connections that need verification._
- **Are the 49 inferred relationships involving `initDBForTest()` (e.g. with `TestUpdateFlashcardReviewTransactionalSave()` and `TestUpdateFlashcardReviewRollsBackCardOnLogInsertFailure()`) actually correct?**
  _`initDBForTest()` has 49 INFERRED edges - model-reasoned connections that need verification._
- **Are the 47 inferred relationships involving `EnsureTopic()` (e.g. with `mustInsertActiveQuizTask()` and `TestGetNotebookTopicTreeReturnsNestedTopics()`) actually correct?**
  _`EnsureTopic()` has 47 INFERRED edges - model-reasoned connections that need verification._
- **Are the 35 inferred relationships involving `CreateNotebook()` (e.g. with `mustInsertActiveQuizTask()` and `TestGetNotebookTopicTreeReturnsNestedTopics()`) actually correct?**
  _`CreateNotebook()` has 35 INFERRED edges - model-reasoned connections that need verification._
- **What connects `llmProviderInterface`, `ingestionProgressPayload`, `OpenAIRequest` to the rest of the system?**
  _99 weakly-connected nodes found - possible documentation gaps or missing edges._