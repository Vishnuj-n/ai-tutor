package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"ai-tutor/internal/db"
	"ai-tutor/internal/embeddings"
	"ai-tutor/internal/llm"
	"ai-tutor/internal/models"
	"ai-tutor/internal/notebook"
	"ai-tutor/internal/rag"
	"ai-tutor/internal/runtime"
	"ai-tutor/internal/scheduler"
	"ai-tutor/internal/utils"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

type llmProviderInterface interface {
	GenerateAnswer(prompt string) (string, error)
}

type ragPipelineInterface interface {
	ProcessQuery(topicID, question string, startPage, endPage int) (*rag.Response, error)
}

// App struct
// Uses interface types for easy contract testing of LLM and RAG behavior.
type App struct {
	ctx               context.Context
	ragPipeline       ragPipelineInterface
	embedStore        *rag.EmbeddingStore
	embedder          *embeddings.OnnxEmbedder
	fastLLMProvider   llmProviderInterface
	heavyLLMProvider  llmProviderInterface
	scheduler         scheduler.Service
	notebookService   *notebook.Service
	notebookUploadDir string
	aiReady           bool
	aiInitError       string
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Load local .env if present. Missing file is fine.
	_ = godotenv.Load()

	// Validate required assets for local RAG
	assetValidator := runtime.NewAssetValidator("asset")
	if err := assetValidator.ValidateAll(); err != nil {
		a.aiInitError = err.Error()
		utils.Warnf("local RAG assets missing: %v", err)
		utils.Warnf("Ask AI features may be unavailable. Ensure asset/ contains tokenizer.json, model_int8.onnx, onnxruntime.dll, vec0.dll")
	}

	appDir, err := resolveAppDir()
	if err != nil {
		a.aiInitError = err.Error()
		utils.Errorf("resolving app directory: %v", err)
		return
	}

	runtimeAssets, err := assetValidator.PrepareRuntimeAssets(appDir)
	if err != nil {
		a.aiInitError = err.Error()
		utils.Warnf("could not stage runtime assets to app-data: %v", err)
	}

	// Initialize persistent database
	dbPath, err := resolveDBPath()
	if err != nil {
		a.aiInitError = err.Error()
		utils.Errorf("resolving database path: %v", err)
		return
	}

	vec0DllPath := assetValidator.Vec0DllPath()
	if stagedVec0Path, ok := runtimeAssets[filepath.Base(vec0DllPath)]; ok {
		vec0DllPath = stagedVec0Path
	}
	if err := db.Init(dbPath, vec0DllPath); err != nil {
		a.aiInitError = err.Error()
		utils.Errorf("initializing database: %v", err)
		return
	}
	utils.Infof("Database initialized at %s", dbPath)

	// Initialize scheduler after database is ready so it can query due cards and active topics.
	a.scheduler = scheduler.New()

	// Initialize ONNX embedder for local vector generation.
	var embedder *embeddings.OnnxEmbedder
	onnxRuntimePath := assetValidator.OnnxRuntimePath()
	if stagedRuntimePath, ok := runtimeAssets[filepath.Base(onnxRuntimePath)]; ok {
		onnxRuntimePath = stagedRuntimePath
	}
	embedder, err = embeddings.NewOnnxEmbedder(assetValidator.ModelPath(), assetValidator.TokenizerPath(), onnxRuntimePath)
	if err != nil {
		a.aiInitError = err.Error()
		utils.Warnf("could not initialize ONNX embedder: %v", err)
		a.aiReady = false
	} else {
		if err := embeddings.InitPromptTokenizer(assetValidator.TokenizerPath()); err != nil {
			a.aiInitError = fmt.Sprintf("could not initialize prompt tokenizer: %v", err)
			utils.Warnf("%s", a.aiInitError)
			a.aiReady = false
			if closeErr := embedder.Close(); closeErr != nil {
				utils.Warnf("could not close embedder after tokenizer init failure: %v", closeErr)
			}
			embedder = nil
		} else {
			a.aiReady = true
			a.aiInitError = ""
			a.embedder = embedder

			if err := db.InitWithVectorDimension(embedder.GetDimension()); err != nil {
				utils.Warnf("could not initialize vector table; Ask AI will use lexical fallback: %v", err)
			} else {
				indexer := rag.NewVectorIndexer(embedder, rag.IndexerConfig{
					RecomputeOnHashMismatch: true,
					ForceReindex:            false,
				})
				go func() {
					if err := indexer.IndexAllTopics(); err != nil {
						utils.Warnf("vector indexing failed: %v", err)
					}
				}()
			}
		}
	}

	// Initialize retrieval store with vector-first retrieval and lexical fallback
	embedStore := rag.NewEmbeddingStore(embedder)
	a.embedStore = embedStore

	// Load chunks for lexical fallback retrieval path.
	topicIDs, err := db.GetAllTopicIDs()
	if err != nil {
		utils.Warnf("could not list topics for lexical fallback: %v", err)
		topicIDs = []string{"os-scheduling"}
	}

	// Batch load all chunks in a single query
	chunksByTopic, err := db.GetChunksForTopics(topicIDs)
	if err != nil {
		utils.Warnf("could not load chunks for topics: %v", err)
		// Fall back to individual queries on batch failure
		for _, topicID := range topicIDs {
			chunks, err := db.GetChunksForTopic(topicID)
			if err != nil {
				utils.Warnf("could not load chunks for topic %s: %v", topicID, err)
				continue
			}
			utils.Infof("Loaded %d chunks for topic %s", len(chunks), topicID)
			for _, chunk := range chunks {
				embedStore.AddChunk(chunk)
			}
		}
	} else {
		// Process batch results
		for _, topicID := range topicIDs {
			chunks := chunksByTopic[topicID]
			utils.Infof("Loaded %d chunks for topic %s", len(chunks), topicID)
			for _, chunk := range chunks {
				embedStore.AddChunk(chunk)
			}
		}
	}

	// Initialize both LLM tiers from .env / environment variables.
	fastLLMConfig := llm.LoadConfigFromEnvForPrefix("FAST_LLM")
	heavyLLMConfig := llm.LoadConfigFromEnvForPrefix("HEAVY_LLM")

	fastLLMProvider := llm.NewProvider(fastLLMConfig)
	heavyLLMProvider := llm.NewProvider(heavyLLMConfig)
	a.fastLLMProvider = fastLLMProvider
	a.heavyLLMProvider = heavyLLMProvider

	// Create RAG pipeline
	a.ragPipeline = rag.NewPipeline(embedStore, heavyLLMProvider)

	// Initialize notebook service
	notebookDir, err := resolveNotebookDir()
	if err != nil {
		utils.Errorf("resolving notebook directory: %v", err)
		return
	}
	a.notebookUploadDir = notebookDir
	a.notebookService = notebook.NewService(notebookDir)
	utils.Infof("Notebook service initialized at %s", notebookDir)

	utils.Infof("App initialized successfully")
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

// GetTopicContent retrieves the content for a specific topic
func (a *App) GetTopicContent(topicID string) map[string]interface{} {
	content, err := db.GetTopicContent(topicID)
	if err != nil {
		return map[string]interface{}{
			"error": err.Error(),
		}
	}
	return content
}

// GetReaderTopicBundle returns notebook metadata plus ordered sections for reader navigation.
func (a *App) GetReaderTopicBundle(topicID string, notebookID string) map[string]interface{} {
	bundle, err := db.GetReaderTopicBundle(topicID, notebookID)
	if err != nil {
		return map[string]interface{}{
			"error": err.Error(),
		}
	}

	topicStartPage, topicEndPage, boundsErr := db.GetTopicPageBounds(topicID)
	if boundsErr != nil {
		topicStartPage = 0
		topicEndPage = 0
	}

	if bundle.NotebookURL != "" {
		bundle.NotebookURL = notebookAssetURL(bundle.NotebookURL)
	}

	lightSections := make([]map[string]interface{}, 0, len(bundle.Sections))
	for _, s := range bundle.Sections {
		lightSections = append(lightSections, map[string]interface{}{
			"id":       s.ID,
			"heading":  s.Heading,
			"page_num": s.PageNum,
			"order":    s.Order,
		})
	}

	return map[string]interface{}{
		"topic_id":         bundle.TopicID,
		"topic_title":      bundle.TopicTitle,
		"topic_start_page": topicStartPage,
		"topic_end_page":   topicEndPage,
		"notebook_id":      bundle.NotebookID,
		"notebook_title":   bundle.NotebookTitle,
		"notebook_url":     bundle.NotebookURL,
		"file_type":        bundle.FileType,
		"page_count":       bundle.PageCount,
		"sections":         lightSections,
	}
}

// GetAvailableTopics returns a list of available topics
func (a *App) GetAvailableTopics() []map[string]string {
	topics, err := db.GetAllTopics()
	if err != nil {
		return []map[string]string{}
	}

	return topics
}

// AskAI processes a question using RAG pipeline
func (a *App) AskAI(topicID string, question string) map[string]interface{} {
	if !a.aiReady {
		reason := a.aiInitError
		if reason == "" {
			reason = "local AI runtime is not ready"
		}
		return map[string]interface{}{
			"error": "Ask AI unavailable: " + reason,
		}
	}

	if a.ragPipeline == nil {
		return map[string]interface{}{
			"error": "RAG pipeline not initialized",
		}
	}

	result, err := a.ragPipeline.ProcessQuery(topicID, question, 0, 0)
	if err != nil {
		return map[string]interface{}{
			"error": err.Error(),
		}
	}

	return map[string]interface{}{
		"answer":           result.Answer,
		"cited_sections":   result.CitedSections,
		"chunks_retrieved": result.ChunksRetrieved,
		"sections_used":    result.SectionsUsed,
	}
}

// ExplainReaderSection explains one reader section without relying on topic-wide retrieval.
func (a *App) ExplainReaderSection(sectionID string, question string) map[string]interface{} {
	sectionID = strings.TrimSpace(sectionID)
	question = strings.TrimSpace(question)
	if sectionID == "" {
		return map[string]interface{}{
			"error": "section ID is required",
		}
	}

	if a.fastLLMProvider == nil {
		return map[string]interface{}{
			"error": "FAST_LLM provider not initialized",
		}
	}

	section, err := db.GetParentSection(sectionID)
	if err != nil {
		return map[string]interface{}{
			"error": "failed to fetch reader section: " + err.Error(),
		}
	}

	if question == "" {
		question = "Explain this section in clear study notes."
	}

	prompt := fmt.Sprintf(`You are an AI study companion.
Use ONLY the section below. Do not add outside knowledge.
If a question asks about details missing from the section, reply with: "This section does not contain that detail."

Section heading: %s
Section content:
%s

User request: %s

Return a response with:
1. Plain-language summary (2–3 sentences, main idea)
2. Key takeaway: why this matters or where it applies
3. Recall cue: one memorable phrase or question to test understanding
4. Example (if the section includes a concrete example or scenario, highlight it; otherwise, skip this)`, section["heading"], section["content"], question)

	answer, err := a.fastLLMProvider.GenerateAnswer(prompt)
	if err != nil {
		return map[string]interface{}{
			"error": "section explanation failed: " + err.Error(),
		}
	}

	return map[string]interface{}{
		"answer":         answer,
		"cited_sections": []string{section["heading"]},
		"section_id":     section["id"],
	}
}

// GetEmbeddingDiagnostics runs a live embedding call and returns quick sanity metrics.
func (a *App) GetEmbeddingDiagnostics(text string) map[string]interface{} {
	if !a.aiReady || a.embedder == nil {
		reason := a.aiInitError
		if reason == "" {
			reason = "local AI runtime is not ready"
		}
		return map[string]interface{}{
			"error": "Embedding diagnostics unavailable: " + reason,
		}
	}

	input := strings.TrimSpace(text)
	if input == "" {
		input = "quick embedding diagnostic sentence"
	}

	vector, err := a.embedder.Embed(input)
	if err != nil {
		return map[string]interface{}{
			"error": "embedding run failed: " + err.Error(),
		}
	}

	declaredDim := int(a.embedder.GetDimension())
	length := len(vector)
	count := length
	if count > 8 {
		count = 8
	}

	sample := make([]float32, count)
	copy(sample, vector[:count])

	var sumSquares float64
	for _, value := range vector {
		sumSquares += float64(value * value)
	}

	return map[string]interface{}{
		"ok":                  true,
		"input_chars":         len(input),
		"declared_dimension":  declaredDim,
		"vector_length":       length,
		"dimension_match":     length == declaredDim,
		"sample_norm_l2":      math.Sqrt(sumSquares),
		"sample_first_values": sample,
	}
}

// GetTodayPlan returns a unified daily schedule (review + learning + quiz/socratic when applicable).
func (a *App) GetTodayPlan() map[string]interface{} {
	if a.scheduler == nil {
		return map[string]interface{}{
			"error": "scheduler not initialized",
		}
	}

	now := time.Now()
	plan, err := a.scheduler.BuildTodayPlan(now)
	if err != nil {
		return map[string]interface{}{
			"error": err.Error(),
		}
	}

	insightsAvailable := false

	return map[string]interface{}{
		"date":               plan.Date,
		"total_minutes":      plan.TotalMinutes,
		"review_minutes":     plan.ReviewMinutes,
		"learning_minutes":   plan.LearningMinutes,
		"due_review_cards":   plan.DueReviewCards,
		"active_topics":      plan.ActiveTopics,
		"tasks":              plan.Tasks,
		"generated_at_unix":  now.Unix(),
		"data_fresh":         true,
		"is_estimate":        plan.IsEstimate,
		"insights_available": insightsAvailable,
		"plan_source":        "scheduler-v2-context-locked",
	}
}

// GetDailyStudySettings returns persisted scheduler settings for sprint-12 pacing.
func (a *App) GetDailyStudySettings() map[string]interface{} {
	minutes, err := db.GetDailyStudyMinutes()
	if err != nil {
		return map[string]interface{}{
			"error": err.Error(),
		}
	}

	return map[string]interface{}{
		"daily_study_minutes": minutes,
	}
}

// UpdateDailyStudyMinutes stores the global daily study limit used by scheduler math.
func (a *App) UpdateDailyStudyMinutes(minutes int) map[string]interface{} {
	if minutes < 15 || minutes > 480 {
		return map[string]interface{}{
			"error": "daily study minutes must be between 15 and 480",
		}
	}

	if err := db.UpsertDailyStudyMinutes(minutes); err != nil {
		return map[string]interface{}{
			"error": err.Error(),
		}
	}

	return map[string]interface{}{
		"ok":                  true,
		"daily_study_minutes": minutes,
	}
}

type quizLLMQuestion struct {
	Prompt          string   `json:"prompt"`
	Options         []string `json:"options"`
	CorrectAnswer   string   `json:"correct_answer"`
	Explanation     string   `json:"explanation"`
	Hint            string   `json:"hint"`
	SourceHeading   string   `json:"source_heading"`
	SourceSnippet   string   `json:"source_snippet"`
	SourcePageStart int      `json:"source_page_start"`
	SourcePageEnd   int      `json:"source_page_end"`
}

type quizLLMResponse struct {
	Questions []quizLLMQuestion `json:"questions"`
}

type flashcardLLMCard struct {
	Prompt string `json:"prompt"`
	Answer string `json:"answer"`
}

type flashcardLLMResponse struct {
	Cards []flashcardLLMCard `json:"cards"`
}

type shortAnswerPromptLLMResponse struct {
	Prompt string `json:"prompt"`
}

type shortAnswerScoreLLMResponse struct {
	Score    int    `json:"score"`
	Feedback string `json:"feedback"`
}

// GenerateQuiz creates topic-scoped multiple-choice questions and stores them.
// Enforces exactly 5 questions per generation to keep quizzes consistent.
func (a *App) GenerateQuiz(topicID string) map[string]interface{} {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return map[string]interface{}{"error": "topic ID is required"}
	}

	if a.fastLLMProvider == nil {
		return map[string]interface{}{"error": "FAST_LLM provider not initialized"}
	}

	existingQuestions, err := db.GetQuestionsForTopic(topicID)
	if err != nil {
		return map[string]interface{}{"error": "failed to load existing quiz questions: " + err.Error()}
	}
	if len(existingQuestions) == 5 {
		return map[string]interface{}{
			"topic_id":  topicID,
			"questions": existingQuestions,
			"existing":  true,
		}
	}

	content, err := db.GetTopicContent(topicID)
	if err != nil {
		return map[string]interface{}{"error": "failed to fetch topic content: " + err.Error()}
	}

	sections, ok := content["sections"].([]map[string]interface{})
	if !ok || len(sections) == 0 {
		return map[string]interface{}{"error": "topic has no sections for quiz generation"}
	}

	prompt := buildQuizPrompt(topicID, sections)
	raw, err := a.fastLLMProvider.GenerateAnswer(prompt)
	if err != nil {
		return map[string]interface{}{"error": "quiz generation failed: " + err.Error()}
	}

	parsed, err := parseQuizLLMResponse(raw)
	if err != nil {
		return map[string]interface{}{"error": "quiz generation parsing failed: " + err.Error()}
	}

	headingPageRanges, rangeErr := db.GetTopicHeadingPageRanges(topicID)
	if rangeErr != nil {
		utils.Warnf("could not resolve topic heading page ranges for quiz lineage (topic=%s): %v", topicID, rangeErr)
		headingPageRanges = map[string][2]int{}
	}
	modelName := providerModelName(a.fastLLMProvider)

	const expectedQuestionCount = 5

	questions := make([]models.QuizQuestion, 0, len(parsed.Questions))
	for _, q := range parsed.Questions {
		if strings.TrimSpace(q.Prompt) == "" || len(q.Options) < 2 || strings.TrimSpace(q.CorrectAnswer) == "" {
			continue
		}

		// Validate CorrectAnswer matches one of the Options (case-insensitive, whitespace-normalized)
		correctAnswerCanonical := strings.TrimSpace(strings.ToLower(q.CorrectAnswer))
		var matchedOption string
		for _, opt := range q.Options {
			optCanonical := strings.TrimSpace(strings.ToLower(opt))
			if optCanonical == correctAnswerCanonical {
				matchedOption = strings.TrimSpace(opt)
				break
			}
		}
		if matchedOption == "" {
			// Skip question if CorrectAnswer doesn't match any option
			continue
		}

		sourcePageStart := q.SourcePageStart
		sourcePageEnd := q.SourcePageEnd
		if sourcePageStart <= 0 || sourcePageEnd <= 0 || sourcePageEnd < sourcePageStart {
			rangeByHeading, ok := headingPageRanges[normalizeHeadingKey(q.SourceHeading)]
			if ok {
				sourcePageStart = rangeByHeading[0]
				sourcePageEnd = rangeByHeading[1]
			}
		}
		if sourcePageStart > 0 && sourcePageEnd <= 0 {
			sourcePageEnd = sourcePageStart
		}
		if sourcePageEnd > 0 && sourcePageStart <= 0 {
			sourcePageStart = sourcePageEnd
		}

		questions = append(questions, models.QuizQuestion{
			ID:              uuid.NewString(),
			TopicID:         topicID,
			Prompt:          strings.TrimSpace(q.Prompt),
			Options:         q.Options,
			CorrectAnswer:   matchedOption,
			Explanation:     strings.TrimSpace(q.Explanation),
			Hint:            strings.TrimSpace(q.Hint),
			SourceHeading:   strings.TrimSpace(q.SourceHeading),
			SourceSnippet:   strings.TrimSpace(q.SourceSnippet),
			SourcePageStart: sourcePageStart,
			SourcePageEnd:   sourcePageEnd,
			LLMModel:        modelName,
			PromptVersion:   "quiz-v1",
		})
	}

	// Enforce exactly the expected number of questions
	if len(questions) != expectedQuestionCount {
		return map[string]interface{}{
			"error": fmt.Sprintf("quiz generation produced %d valid questions; expected exactly %d", len(questions), expectedQuestionCount),
		}
	}

	if err := db.ReplaceQuestionsForTopic(topicID, questions); err != nil {
		return map[string]interface{}{"error": "failed to save generated quiz: " + err.Error()}
	}

	return map[string]interface{}{
		"topic_id":  topicID,
		"questions": questions,
	}
}

// CompleteReadingSession generates and stores incremental assessment questions for a locked page window.
func (a *App) CompleteReadingSession(topicID string, startPage int, targetPage int) map[string]interface{} {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return map[string]interface{}{"error": "topic ID is required"}
	}
	if targetPage <= 0 {
		return map[string]interface{}{"error": "target page must be positive"}
	}
	if a.fastLLMProvider == nil {
		return map[string]interface{}{"error": "FAST_LLM provider not initialized"}
	}

	topicStartPage, topicEndPage, err := db.GetTopicPageBounds(topicID)
	if err != nil {
		return map[string]interface{}{"error": "failed to fetch topic page bounds: " + err.Error()}
	}
	if topicEndPage <= 0 {
		return map[string]interface{}{"error": "topic has no page bounds configured"}
	}

	if startPage <= 0 {
		if topicStartPage > 0 {
			startPage = topicStartPage
		} else {
			startPage = 1
		}
	}
	if topicStartPage > 0 && startPage < topicStartPage {
		startPage = topicStartPage
	}
	if targetPage > topicEndPage {
		targetPage = topicEndPage
	}
	if targetPage < startPage {
		return map[string]interface{}{"error": "invalid completion window: target page must be >= start page"}
	}

	contextEndPage := targetPage + 1
	if contextEndPage > topicEndPage {
		contextEndPage = topicEndPage
	}

	parentPassages, err := db.GetParentPassagesForTopicPageRange(topicID, startPage, contextEndPage)
	if err != nil {
		return map[string]interface{}{"error": "failed to fetch completion passages: " + err.Error()}
	}
	if len(parentPassages) == 0 {
		return map[string]interface{}{"error": "no passage content found for completion window"}
	}

	raw, err := a.fastLLMProvider.GenerateAnswer(buildReaderCompletionQuizPrompt(topicID, startPage, targetPage, contextEndPage, parentPassages))
	if err != nil {
		return map[string]interface{}{"error": "completion quiz generation failed: " + err.Error()}
	}

	parsed, err := parseQuizLLMResponse(raw)
	if err != nil {
		return map[string]interface{}{"error": "completion quiz parsing failed: " + err.Error()}
	}

	const expectedQuestionCount = 5
	modelName := providerModelName(a.fastLLMProvider)
	questions := make([]models.QuizQuestion, 0, len(parsed.Questions))
	for _, q := range parsed.Questions {
		if strings.TrimSpace(q.Prompt) == "" || len(q.Options) < 2 || strings.TrimSpace(q.CorrectAnswer) == "" {
			continue
		}

		correctAnswerCanonical := strings.TrimSpace(strings.ToLower(q.CorrectAnswer))
		var matchedOption string
		for _, opt := range q.Options {
			optCanonical := strings.TrimSpace(strings.ToLower(opt))
			if optCanonical == correctAnswerCanonical {
				matchedOption = strings.TrimSpace(opt)
				break
			}
		}
		if matchedOption == "" {
			continue
		}

		sourcePageStart := q.SourcePageStart
		sourcePageEnd := q.SourcePageEnd
		if sourcePageStart <= 0 || sourcePageEnd <= 0 || sourcePageEnd < sourcePageStart {
			sourcePageStart = startPage
			sourcePageEnd = contextEndPage
		}
		if sourcePageStart < startPage {
			sourcePageStart = startPage
		}
		if sourcePageEnd > contextEndPage {
			sourcePageEnd = contextEndPage
		}
		if sourcePageEnd < sourcePageStart {
			sourcePageEnd = sourcePageStart
		}

		questions = append(questions, models.QuizQuestion{
			ID:              uuid.NewString(),
			TopicID:         topicID,
			Prompt:          strings.TrimSpace(q.Prompt),
			Options:         q.Options,
			CorrectAnswer:   matchedOption,
			Explanation:     strings.TrimSpace(q.Explanation),
			Hint:            strings.TrimSpace(q.Hint),
			SourceHeading:   strings.TrimSpace(q.SourceHeading),
			SourceSnippet:   strings.TrimSpace(q.SourceSnippet),
			SourcePageStart: sourcePageStart,
			SourcePageEnd:   sourcePageEnd,
			LLMModel:        modelName,
			PromptVersion:   "reader-complete-v1",
		})
	}

	if len(questions) != expectedQuestionCount {
		return map[string]interface{}{
			"error": fmt.Sprintf("completion quiz produced %d valid questions; expected exactly %d", len(questions), expectedQuestionCount),
		}
	}

	// Validate cursor before appending questions to prevent replayed/backwards advances
	currentCursor, err := db.GetTopicCurrentPageCursor(topicID)
	if err != nil {
		return map[string]interface{}{"error": "failed to fetch current page cursor: " + err.Error()}
	}
	if startPage != currentCursor {
		return map[string]interface{}{"error": fmt.Sprintf("invalid completion window: start page %d does not match current cursor %d", startPage, currentCursor)}
	}
	if targetPage <= currentCursor {
		return map[string]interface{}{"error": fmt.Sprintf("invalid completion window: target page %d must be greater than current cursor %d", targetPage, currentCursor)}
	}

	nextCursor := targetPage + 1
	markLearned := targetPage >= topicEndPage
	if err := db.AppendQuestionsAndAdvanceCursor(topicID, questions, nextCursor, markLearned); err != nil {
		return map[string]interface{}{"error": "failed to append completion questions and update cursor: " + err.Error()}
	}

	status := "reading"
	if markLearned {
		status = "learned"
	}

	return map[string]interface{}{
		"ok":                  true,
		"topic_id":            topicID,
		"source_page_start":   startPage,
		"source_page_end":     contextEndPage,
		"target_page":         targetPage,
		"questions_generated": len(questions),
		"prompt_version":      "reader-complete-v1",
		"current_page_cursor": nextCursor,
		"topic_status":        status,
	}
}

// GenerateFlashcards creates flashcards for a topic or returns the existing set.
// Enforces exactly 8 flashcards per generation to keep decks consistent and predictable.
func (a *App) GenerateFlashcards(topicID string) map[string]interface{} {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return map[string]interface{}{"error": "topic ID is required"}
	}

	if a.fastLLMProvider == nil {
		return map[string]interface{}{"error": "FAST_LLM provider not initialized"}
	}

	content, err := db.GetTopicContent(topicID)
	if err != nil {
		return map[string]interface{}{"error": "failed to fetch topic content: " + err.Error()}
	}

	sections, ok := content["sections"].([]map[string]interface{})
	if !ok || len(sections) == 0 {
		return map[string]interface{}{"error": "topic has no sections for flashcard generation"}
	}

	// Generate flashcard candidates from LLM
	raw, err := a.fastLLMProvider.GenerateAnswer(buildFlashcardPrompt(topicID, sections))
	if err != nil {
		return map[string]interface{}{"error": "flashcard generation failed: " + err.Error()}
	}

	parsed, err := parseFlashcardLLMResponse(raw)
	if err != nil {
		return map[string]interface{}{"error": "flashcard generation parsing failed: " + err.Error()}
	}

	const expectedFlashcardCount = 8

	// Filter valid cards
	now := time.Now().Unix()
	cards := make([]models.Flashcard, 0, len(parsed.Cards))
	states := make(map[string]models.FlashcardState, len(parsed.Cards))
	for _, candidate := range parsed.Cards {
		prompt := strings.TrimSpace(candidate.Prompt)
		answer := strings.TrimSpace(candidate.Answer)
		if prompt == "" || answer == "" {
			continue
		}

		id := uuid.NewString()
		cards = append(cards, models.Flashcard{
			ID:        id,
			TopicID:   topicID,
			Prompt:    prompt,
			Answer:    answer,
			DueAt:     now,
			Suspended: false,
		})
		states[id] = models.FlashcardState{}
	}

	// Enforce exactly 8 cards
	if len(cards) != expectedFlashcardCount {
		return map[string]interface{}{
			"error": fmt.Sprintf("flashcard generation produced %d valid cards; expected exactly %d", len(cards), expectedFlashcardCount),
		}
	}

	// Use atomic get-or-create to prevent race condition:
	// If cards already exist for this topic, return them
	// Otherwise, insert the generated cards transactionally
	cards, wasExisting, err := db.GetOrCreateFlashcardsForTopic(topicID, cards, states)
	if err != nil {
		return map[string]interface{}{"error": "failed to persist flashcards: " + err.Error()}
	}
	if wasExisting {
		states = make(map[string]models.FlashcardState, len(cards))
		for _, card := range cards {
			_, state, getErr := db.GetFlashcardByID(card.ID)
			if getErr != nil {
				return map[string]interface{}{"error": "failed to load existing flashcard state: " + getErr.Error()}
			}
			if state != nil {
				states[card.ID] = *state
			}
		}
	}

	return map[string]interface{}{
		"topic_id": topicID,
		"cards":    cards,
		"states":   states,
		"existing": wasExisting,
	}
}

// GetFlashcards returns topic-scoped flashcards, optionally filtered to due cards only.
func (a *App) GetFlashcards(topicID string, dueOnly bool) map[string]interface{} {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return map[string]interface{}{"error": "topic ID is required"}
	}

	var now int64
	if dueOnly {
		now = time.Now().Unix()
	}

	cards, err := db.GetFlashcardsForTopic(topicID, dueOnly, now)
	if err != nil {
		return map[string]interface{}{"error": "failed to fetch flashcards: " + err.Error()}
	}

	return map[string]interface{}{
		"topic_id": topicID,
		"cards":    cards,
	}
}

// RecordFlashcardReview applies a review rating and schedules the next due date.
func (a *App) RecordFlashcardReview(cardID string, rating string) map[string]interface{} {
	cardID = strings.TrimSpace(cardID)
	rating = strings.ToLower(strings.TrimSpace(rating))
	if cardID == "" {
		return map[string]interface{}{"error": "flashcard ID is required"}
	}

	ratingCode, ok := mapReviewRating(rating)
	if !ok {
		return map[string]interface{}{"error": "rating must be one of again, hard, good, easy"}
	}

	card, state, err := db.GetFlashcardByID(cardID)
	if err != nil {
		return map[string]interface{}{"error": "failed to fetch flashcard: " + err.Error()}
	}
	if card == nil || state == nil {
		return map[string]interface{}{"error": "flashcard not found"}
	}

	stateBeforeJSONBytes, err := json.Marshal(state)
	if err != nil {
		return map[string]interface{}{"error": "failed to encode flashcard state: " + err.Error()}
	}

	now := time.Now().Unix()
	elapsedSeconds := now - card.DueAt
	elapsedDays := 0
	if elapsedSeconds > 0 {
		elapsedDays = int(elapsedSeconds / (24 * 60 * 60))
	}
	state.ElapsedDays = elapsedDays

	nextState := scheduler.NextFSRSState(*state, ratingCode)
	dueAt := now + int64(nextState.ScheduledDays)*24*60*60
	if nextState.ScheduledDays == 0 {
		dueAt = now
	}

	stateAfterJSONBytes, err := json.Marshal(nextState)
	if err != nil {
		return map[string]interface{}{"error": "failed to encode updated flashcard state: " + err.Error()}
	}

	reviewLog := models.FSRSReviewLog{
		ID:              uuid.NewString(),
		TopicID:         card.TopicID,
		ActivityType:    "flashcard",
		ReferenceID:     card.ID,
		ReviewedAt:      now,
		Rating:          ratingCode,
		ScheduledDays:   nextState.ScheduledDays,
		StateBeforeJSON: string(stateBeforeJSONBytes),
		StateAfterJSON:  string(stateAfterJSONBytes),
	}

	if err := db.UpdateFlashcardReview(cardID, dueAt, card.DueAt, nextState, reviewLog); err != nil {
		return map[string]interface{}{"error": "failed to update flashcard review: " + err.Error()}
	}

	card.DueAt = dueAt
	return map[string]interface{}{
		"card":          card,
		"state":         &nextState,
		"review_log_id": reviewLog.ID,
	}
}

// ScoreAnswer validates an answer and stores score metadata.
func (a *App) ScoreAnswer(questionID, userAnswer string) map[string]interface{} {
	questionID = strings.TrimSpace(questionID)
	userAnswer = strings.TrimSpace(userAnswer)
	if questionID == "" || userAnswer == "" {
		return map[string]interface{}{"error": "question ID and user answer are required"}
	}

	question, err := db.GetQuestionByID(questionID)
	if err != nil {
		return map[string]interface{}{"error": "failed to fetch question: " + err.Error()}
	}
	if question == nil {
		return map[string]interface{}{"error": "question not found"}
	}

	expected := normalizeQuizAnswer(question.CorrectAnswer, question.Options)
	actual := normalizeQuizAnswer(userAnswer, question.Options)
	correct := expected != "" && expected == actual

	hint := question.Hint
	if hint == "" {
		hint = "Review the cited section and compare each option against the source."
	}
	score := models.QuizScore{
		QuestionID:    question.ID,
		Correct:       correct,
		Score:         0,
		Expected:      question.CorrectAnswer,
		Feedback:      question.Explanation,
		Hint:          hint,
		UserAnswer:    userAnswer,
		SourceHeading: question.SourceHeading,
	}
	if correct {
		score.Score = 100
		if score.Hint == "Review the cited section and compare each option against the source." {
			score.Hint = "Great job. Move to the next question."
		}
	} else if strings.TrimSpace(question.Explanation) == "" {
		score.Feedback = "That answer is not correct."
	}

	if err := db.SaveUserAnswer(score); err != nil {
		return map[string]interface{}{"error": "failed to save score: " + err.Error()}
	}

	return map[string]interface{}{
		"question_id":    score.QuestionID,
		"correct":        score.Correct,
		"score":          score.Score,
		"expected":       score.Expected,
		"feedback":       score.Feedback,
		"hint":           score.Hint,
		"user_answer":    score.UserAnswer,
		"source_heading": score.SourceHeading,
	}
}

// GenerateShortAnswerPrompt creates one grounded short-answer question from topic content.
func (a *App) GenerateShortAnswerPrompt(topicID string) map[string]interface{} {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return map[string]interface{}{"error": "topic ID is required"}
	}
	if a.fastLLMProvider == nil {
		return map[string]interface{}{"error": "FAST_LLM provider not initialized"}
	}
	if a.ragPipeline == nil {
		return map[string]interface{}{"error": "RAG pipeline not initialized"}
	}

	request := `Generate exactly one short-answer assessment question grounded in the retrieved material.
Return STRICT JSON only in this shape: {"prompt":"..."}.
Rules:
- Ask exactly one question.
- Keep it concise (max 30 words).
- Require understanding, not pure definition recall.
- Do not include answer choices, rubric, preamble, or markdown.`

	result, err := a.ragPipeline.ProcessQuery(topicID, request, 0, 0)
	if err != nil {
		return map[string]interface{}{"error": "short-answer prompt generation failed: " + err.Error()}
	}

	parsed, err := parseShortAnswerPromptLLMResponse(result.Answer)
	if err != nil {
		return map[string]interface{}{"error": "short-answer prompt parsing failed: " + err.Error()}
	}

	questionPrompt := strings.TrimSpace(parsed.Prompt)
	if questionPrompt == "" {
		return map[string]interface{}{"error": "short-answer prompt generation returned empty prompt"}
	}

	questionID := fmt.Sprintf("%s:%s", topicID, uuid.NewString())
	return map[string]interface{}{
		"questionID": questionID,
		"prompt":     questionPrompt,
		"topicID":    topicID,
	}
}

// ScoreShortAnswer scores one short-answer response, validates the score range, and logs a generic FSRS review event.
func (a *App) ScoreShortAnswer(questionID, prompt, userAnswer string) map[string]interface{} {
	questionID = strings.TrimSpace(questionID)
	prompt = strings.TrimSpace(prompt)
	userAnswer = strings.TrimSpace(userAnswer)

	if questionID == "" || prompt == "" || userAnswer == "" {
		return map[string]interface{}{"error": "question ID, prompt, and user answer are required"}
	}
	if a.fastLLMProvider == nil {
		return map[string]interface{}{"error": "FAST_LLM provider not initialized"}
	}

	topicID, ok := topicIDFromShortAnswerQuestionID(questionID)
	if !ok {
		return map[string]interface{}{"error": "invalid question ID format"}
	}

	scorePrompt := fmt.Sprintf(`You are grading a student's short answer.
Return STRICT JSON only in this shape: {"score":number,"feedback":"..."}.

Scoring rubric:
- Score must be an integer from 1 to 10.
- 1-4 = major misunderstandings or mostly incorrect.
- 5-7 = partially correct with gaps.
- 8-9 = correct with minor omissions.
- 10 = fully correct, precise, and concise.
- Feedback must be concise (max 2 sentences), specific, and actionable.

Question: %s
Student answer: %s`, prompt, userAnswer)

	raw, err := a.fastLLMProvider.GenerateAnswer(scorePrompt)
	if err != nil {
		return map[string]interface{}{"error": "short-answer scoring failed: " + err.Error()}
	}

	parsed, err := parseShortAnswerScoreLLMResponse(raw)
	if err != nil {
		return map[string]interface{}{"error": "short-answer scoring parse failed: " + err.Error()}
	}

	score := parsed.Score
	if score < 1 {
		score = 1
	}
	if score > 10 {
		score = 10
	}

	ratingLabel, ratingCode := shortAnswerScoreToFSRSRating(score)

	stateBefore := map[string]string{
		"prompt":      prompt,
		"user_answer": userAnswer,
	}
	stateBeforeJSON, err := json.Marshal(stateBefore)
	if err != nil {
		return map[string]interface{}{"error": "failed to encode short-answer review state: " + err.Error()}
	}

	stateAfter := map[string]interface{}{
		"score_out_of_10": score,
		"feedback":        strings.TrimSpace(parsed.Feedback),
		"rating":          ratingLabel,
	}
	stateAfterJSON, err := json.Marshal(stateAfter)
	if err != nil {
		return map[string]interface{}{"error": "failed to encode short-answer review result: " + err.Error()}
	}

	reviewLog := models.FSRSReviewLog{
		ID:              uuid.NewString(),
		TopicID:         topicID,
		ActivityType:    "short_answer",
		ReferenceID:     questionID,
		ReviewedAt:      time.Now().Unix(),
		Rating:          ratingCode,
		ScheduledDays:   0,
		StateBeforeJSON: string(stateBeforeJSON),
		StateAfterJSON:  string(stateAfterJSON),
	}

	if err := db.InsertFSRSReviewLog(reviewLog); err != nil {
		return map[string]interface{}{"error": "failed to log short-answer review: " + err.Error()}
	}

	return map[string]interface{}{
		"questionID":  questionID,
		"prompt":      prompt,
		"score":       score,
		"feedback":    strings.TrimSpace(parsed.Feedback),
		"fsrsRating":  ratingLabel,
		"reviewLogID": reviewLog.ID,
	}
}

// buildContextString builds a context string from sections with truncation limits
func buildContextString(sections []map[string]interface{}, maxSections, maxContentPerSection, maxTotalContent int) string {
	var b strings.Builder
	totalContentLength := 0
	sectionCount := 0

	for _, section := range sections {
		if sectionCount >= maxSections || totalContentLength >= maxTotalContent {
			break
		}

		heading, _ := section["heading"].(string)
		content, _ := section["content"].(string)
		if strings.TrimSpace(content) == "" {
			continue
		}

		content = semanticSnippet(content, maxContentPerSection)

		b.WriteString("[Heading] ")
		b.WriteString(heading)
		b.WriteString("\n")
		b.WriteString(content)
		b.WriteString("\n---\n")

		totalContentLength += len(content)
		sectionCount++
	}

	return b.String()
}

func buildQuizPrompt(topicID string, sections []map[string]interface{}) string {
	var b strings.Builder
	b.WriteString("You are an AI tutor quiz generator. Return STRICT JSON only. No markdown.\n")
	b.WriteString("Generate exactly 5 multiple-choice questions for topic: ")
	b.WriteString(topicID)
	b.WriteString("\nJSON format: {\"questions\":[{\"prompt\":string,\"options\":[string,string,string,string],\"correct_answer\":string,\"explanation\":string,\"hint\":string,\"source_heading\":string,\"source_snippet\":string}]}\n")
	b.WriteString("\n=== QUESTION DIVERSITY (CRITICAL) ===\n")
	b.WriteString("Cover different concepts and question types. AVOID repetition.\n")
	b.WriteString("Required mix:\n")
	b.WriteString("  - 1–2 definitional/recall questions\n")
	b.WriteString("  - 2–3 application/analysis questions (test understanding, not just memory)\n")
	b.WriteString("  - 1 misconception/tricky question (common student errors)\n")
	b.WriteString("\n=== DISTRACTORS ===\n")
	b.WriteString("- Make wrong options plausible and specific (not obviously wrong).\n")
	b.WriteString("- Common misconceptions as distractors are encouraged.\n")
	b.WriteString("\n=== RULES ===\n")
	b.WriteString("- correct_answer must match one option exactly.\n")
	b.WriteString("- Keep each option short (< 15 words).\n")
	b.WriteString("- Explanations grounded in source text (quote when helpful).\n")
	b.WriteString("- Each question must require understanding, not just recall.\n")
	b.WriteString("\nSource sections:\n")

	context := buildContextString(sections, 5, 500, 2500)
	b.WriteString(context)

	return b.String()
}

func buildReaderCompletionQuizPrompt(topicID string, startPage int, targetPage int, contextEndPage int, parentPassages []string) string {
	var b strings.Builder
	b.WriteString("You are an AI tutor quiz generator. Return STRICT JSON only. No markdown.\n")
	b.WriteString("Generate exactly 5 multiple-choice questions for this completed reading session.\n")
	b.WriteString("Topic ID: ")
	b.WriteString(topicID)
	b.WriteString("\nLocked completion window: pages ")
	fmt.Fprintf(&b, "%d-%d", startPage, targetPage)
	b.WriteString("\nAssessment context window: pages ")
	fmt.Fprintf(&b, "%d-%d", startPage, contextEndPage)
	fmt.Fprintf(&b, "\nGenerate questions only from pages %d-%d. Page %d is buffer/supporting context only.", startPage, targetPage, contextEndPage)
	b.WriteString("\nJSON format: {\"questions\":[{\"prompt\":string,\"options\":[string,string,string,string],\"correct_answer\":string,\"explanation\":string,\"hint\":string,\"source_heading\":string,\"source_snippet\":string,\"source_page_start\":number,\"source_page_end\":number}]}\n")
	b.WriteString("Rules:\n")
	b.WriteString("- Return exactly 5 questions.\n")
	b.WriteString("- correct_answer must match one option exactly.\n")
	b.WriteString("- Keep all questions grounded in the context below.\n")
	b.WriteString("- source_page_start/source_page_end must be within the context window.\n")
	b.WriteString("- Cover definitions, application, and one misconception.\n")
	b.WriteString("\nContext chunks (ordered):\n")

	// Reserve tokens for system prompt and instructions (estimated 300 tokens)
	const systemPromptTokens = 300
	// Reserve tokens for JSON structure and questions (estimated 800 tokens)
	const outputStructureTokens = 800

	// Get model token budget - use conservative 4k limit for compatibility
	const maxModelTokens = 4096
	availableContextTokens := maxModelTokens - systemPromptTokens - outputStructureTokens

	// Accumulate passages within token budget
	currentTokens := 0
	for _, text := range parentPassages {
		// Count tokens for this passage
		passageTokens, err := embeddings.CountTokens(text)
		if err != nil {
			// Fallback to character-based estimate if tokenizer fails
			passageTokens = len(text) / 4 // rough estimate
		}

		// Check if adding this passage would exceed budget
		if currentTokens+passageTokens > availableContextTokens {
			// Add truncated snippet if we have remaining tokens
			remainingTokens := availableContextTokens - currentTokens
			if remainingTokens > 0 {
				b.WriteString("- ")
				truncatedSnippet, err := semanticSnippetByTokens(text, remainingTokens)
				if err != nil {
					// Fallback to character-based truncation on error
					truncatedSnippet = semanticSnippet(text, remainingTokens*4)
				}
				b.WriteString(truncatedSnippet)
				b.WriteString("\n")
			}
			break
		}

		b.WriteString("- ")
		// Use semantic snippet but truncate by tokens instead of characters
		snippet, err := semanticSnippetByTokens(text, passageTokens)
		if err != nil {
			// Fallback to character-based truncation on error
			snippet = semanticSnippet(text, passageTokens*4)
		}
		b.WriteString(snippet)
		b.WriteString("\n")

		currentTokens += passageTokens
	}
	return b.String()
}

func buildFlashcardPrompt(topicID string, sections []map[string]interface{}) string {
	var b strings.Builder
	b.WriteString("You are an AI tutor flashcard generator optimized for spaced repetition (FSRS). Return STRICT JSON only. No markdown.\n")
	b.WriteString("Generate exactly 8 flashcards for topic: ")
	b.WriteString(topicID)
	b.WriteString("\nJSON format: {\"cards\":[{\"prompt\":string,\"answer\":string}]}\n")
	b.WriteString("\n=== ATOMIC KNOWLEDGE (CRITICAL) ===\n")
	b.WriteString("Each card must test exactly ONE concept. Multi-part answers are forbidden.\n")
	b.WriteString("\n=== PROMPT QUALITY ===\n")
	b.WriteString("- AVOID yes/no questions.\n")
	b.WriteString("- PREFER 'why', 'how', 'what is', 'explain' questions.\n")
	b.WriteString("- Make prompts specific and testable (not vague).\n")
	b.WriteString("- Example bad: 'What is X?' → Example good: 'What is the purpose of X in context Y?'\n")
	b.WriteString("\n=== DIFFICULTY DISTRIBUTION (CRITICAL) ===\n")
	b.WriteString("- 3 cards: Basic (definitions, key terms, simple facts)\n")
	b.WriteString("- 3 cards: Intermediate (relationships, processes, mechanisms)\n")
	b.WriteString("- 2 cards: Challenging (applications, synthesis, edge cases)\n")
	b.WriteString("\n=== ANSWER QUALITY ===\n")
	b.WriteString("- Answers must be short (1–2 sentences max, grounded in source).\n")
	b.WriteString("- Include terminology but keep accessible.\n")
	b.WriteString("- If a formula or number needed, include it.\n")
	b.WriteString("\nSource sections:\n")

	context := buildContextString(sections, 5, 500, 2500)
	b.WriteString(context)

	return b.String()
}

func parseQuizLLMResponse(raw string) (*quizLLMResponse, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty LLM response")
	}

	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		raw = raw[start : end+1]
	}

	var out quizLLMResponse
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	if len(out.Questions) == 0 {
		return nil, fmt.Errorf("no questions in LLM response")
	}
	return &out, nil
}

func parseFlashcardLLMResponse(raw string) (*flashcardLLMResponse, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty LLM response")
	}

	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		raw = raw[start : end+1]
	}

	var out flashcardLLMResponse
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	if len(out.Cards) == 0 {
		return nil, fmt.Errorf("no cards in LLM response")
	}
	return &out, nil
}

func parseShortAnswerPromptLLMResponse(raw string) (*shortAnswerPromptLLMResponse, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty LLM response")
	}

	// Try to extract JSON from the response, but preserve original for fallback
	jsonSlice := raw
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		jsonSlice = raw[start : end+1]
	}

	var out shortAnswerPromptLLMResponse
	if err := json.Unmarshal([]byte(jsonSlice), &out); err != nil {
		// Fallback for providers that ignore JSON-only instruction.
		// Use the original unmodified response, not the extracted JSON slice.
		fallback := strings.TrimSpace(raw)
		if fallback == "" {
			return nil, err
		}
		out.Prompt = fallback
	}
	if strings.TrimSpace(out.Prompt) == "" {
		return nil, fmt.Errorf("no prompt in LLM response")
	}
	return &out, nil
}

// parseShortAnswerScoreLLMResponse parses the LLM response and returns the raw score and feedback.
// Score normalization and range enforcement is handled centrally in ScoreShortAnswer.
func parseShortAnswerScoreLLMResponse(raw string) (*shortAnswerScoreLLMResponse, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty LLM response")
	}

	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		raw = raw[start : end+1]
	}

	var out shortAnswerScoreLLMResponse
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	if strings.TrimSpace(out.Feedback) == "" {
		out.Feedback = "Review key topic concepts and tighten your explanation."
	}
	return &out, nil
}

func providerModelName(provider llmProviderInterface) string {
	typed, ok := provider.(interface{ ModelName() string })
	if !ok {
		return ""
	}
	return strings.TrimSpace(typed.ModelName())
}

func normalizeHeadingKey(heading string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(heading)), " "))
}

func normalizeQuizAnswer(answer string, options []string) string {
	ans := strings.TrimSpace(strings.ToLower(answer))
	if ans == "" {
		return ""
	}

	if len(ans) == 1 {
		idx := int(ans[0] - 'a')
		if idx >= 0 && idx < len(options) {
			return strings.ToLower(strings.TrimSpace(options[idx]))
		}
	}

	return ans
}

func semanticSnippet(content string, limit int) string {
	trimmed := strings.TrimSpace(content)
	if limit <= 0 || trimmed == "" {
		return ""
	}

	runes := []rune(trimmed)
	if len(runes) <= limit {
		return trimmed
	}

	// Work with rune slice to respect limit and avoid mid-rune truncation
	cutRunes := runes[:limit]
	cut := string(cutRunes)
	// Prefer sentence/line boundaries to avoid abrupt truncation mid-thought.
	best := strings.LastIndex(cut, ". ")
	if idx := strings.LastIndex(cut, "\n"); idx > best {
		best = idx
	}
	if idx := strings.LastIndex(cut, "? "); idx > best {
		best = idx
	}
	if idx := strings.LastIndex(cut, "! "); idx > best {
		best = idx
	}

	if best > limit/2 {
		candidate := strings.TrimSpace(cut[:best+1])
		if candidate != "" {
			// Convert back to runes and check length to avoid mid-rune truncation
			candidateRunes := []rune(candidate)
			if len(candidateRunes) > limit-3 {
				candidateRunes = candidateRunes[:limit-3]
			}
			return string(candidateRunes) + "..."
		}
	}

	cut = strings.TrimSpace(cut)
	// Convert to runes and cap to limit-3 to ensure "..." fits
	cutRunes = []rune(cut)
	if len(cutRunes) > limit-3 {
		cutRunes = cutRunes[:limit-3]
	}
	return string(cutRunes) + "..."
}

// semanticSnippetByTokens truncates text to fit within a token budget using the tokenizer
func semanticSnippetByTokens(content string, maxTokens int) (string, error) {
	trimmed := strings.TrimSpace(content)
	if maxTokens <= 0 || trimmed == "" {
		return "", nil
	}

	// Check if content already fits within token budget
	tokens, err := embeddings.CountTokens(trimmed)
	if err != nil {
		return "", fmt.Errorf("failed to count tokens: %w", err)
	}

	if tokens <= maxTokens {
		return trimmed, nil
	}

	// Use tokenizer to truncate to token limit
	truncated, err := embeddings.TruncateToTokens(trimmed, maxTokens)
	if err != nil {
		return "", fmt.Errorf("failed to truncate to tokens: %w", err)
	}

	// Re-verify token count after truncation to ensure strict bounds
	truncatedTokens, err := embeddings.CountTokens(truncated)
	if err != nil {
		return "", fmt.Errorf("failed to count truncated tokens: %w", err)
	}

	// If truncated still exceeds maxTokens, truncate more conservatively
	if truncatedTokens > maxTokens {
		conservativeLimit := maxTokens - 10
		if conservativeLimit > 0 {
			conservative, err := embeddings.TruncateToTokens(trimmed, conservativeLimit)
			if err != nil {
				return "", fmt.Errorf("failed to truncate conservatively: %w", err)
			}
			// Verify the conservative truncation
			conservativeTokens, verifyErr := embeddings.CountTokens(conservative)
			if verifyErr != nil {
				return "", fmt.Errorf("failed to count conservative tokens: %w", verifyErr)
			}
			if conservativeTokens <= maxTokens {
				truncated = conservative
				truncatedTokens = conservativeTokens
			} else {
				// If still exceeds, use even more conservative limit
				veryConservativeLimit := maxTokens - 20
				if veryConservativeLimit > 0 {
					veryConservative, veryErr := embeddings.TruncateToTokens(trimmed, veryConservativeLimit)
					if veryErr != nil {
						return "", fmt.Errorf("failed to truncate very conservatively: %w", veryErr)
					}
					veryTokens, veryVerifyErr := embeddings.CountTokens(veryConservative)
					if veryVerifyErr != nil {
						return "", fmt.Errorf("failed to count very conservative tokens: %w", veryVerifyErr)
					}
					if veryTokens <= maxTokens {
						truncated = veryConservative
						truncatedTokens = veryTokens
					}
				}
			}
		}
	}

	// Apply semantic boundary logic to the token-truncated text
	// Prefer sentence boundaries to avoid abrupt truncation
	best := strings.LastIndex(truncated, ". ")
	if idx := strings.LastIndex(truncated, "\n"); idx > best {
		best = idx
	}
	if idx := strings.LastIndex(truncated, "? "); idx > best {
		best = idx
	}
	if idx := strings.LastIndex(truncated, "! "); idx > best {
		best = idx
	}

	// Only use boundary if we keep at least half the content
	if best > len(truncated)/2 {
		candidate := strings.TrimSpace(truncated[:best+1])
		if candidate != "" {
			// Verify candidate still fits within token budget
			candidateTokens, err := embeddings.CountTokens(candidate)
			if err != nil {
				return "", fmt.Errorf("failed to count candidate tokens: %w", err)
			}
			if candidateTokens <= maxTokens {
				return candidate + "...", nil
			}
		}
	}

	// Final verification: ensure truncated text fits within token budget
	if truncatedTokens <= maxTokens {
		return truncated, nil
	}

	return truncated, nil
}

func mapReviewRating(rating string) (int, bool) {
	switch rating {
	case "again":
		return scheduler.Again, true
	case "hard":
		return scheduler.Hard, true
	case "good":
		return scheduler.Good, true
	case "easy":
		return scheduler.Easy, true
	default:
		return 0, false
	}
}

func topicIDFromShortAnswerQuestionID(questionID string) (string, bool) {
	parts := strings.SplitN(questionID, ":", 2)
	if len(parts) != 2 {
		return "", false
	}
	topicID := strings.TrimSpace(parts[0])
	if topicID == "" {
		return "", false
	}
	return topicID, true
}

func shortAnswerScoreToFSRSRating(score int) (string, int) {
	switch {
	case score <= 4:
		return "again", scheduler.Again
	case score <= 7:
		return "hard", scheduler.Hard
	case score <= 9:
		return "good", scheduler.Good
	default:
		return "easy", scheduler.Easy
	}
}

func resolveAppDir() (string, error) {
	baseDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	appDir := filepath.Join(baseDir, "ai-tutor")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return "", err
	}

	return appDir, nil
}

func resolveDBPath() (string, error) {
	appDir, err := resolveAppDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(appDir, "ai-tutor.db"), nil
}

func resolveNotebookDir() (string, error) {
	appDir, err := resolveAppDir()
	if err != nil {
		return "", err
	}
	uploadDir := filepath.Join(appDir, "uploads")
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		return "", err
	}

	return uploadDir, nil
}

func notebookAssetURL(filePath string) string {
	// Normalize backslashes to forward slashes for host-neutral path handling
	normPath := strings.TrimSpace(strings.ReplaceAll(filePath, "\\", "/"))
	if normPath == "" || normPath == "." || normPath == ".." {
		return ""
	}
	// Use path.Base for cross-platform compatibility
	name := strings.TrimSpace(path.Base(normPath))
	if name == "" || name == "." || name == ".." {
		return ""
	}
	return "/notebooks/" + url.PathEscape(name)
}
