package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"os"
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

	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

// App struct
type App struct {
	ctx               context.Context
	ragPipeline       *rag.Pipeline
	embedStore        *rag.EmbeddingStore
	embedder          *embeddings.OnnxEmbedder
	llmProvider       *llm.Provider
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
		fmt.Printf("Warning: local RAG assets missing: %v\n", err)
		fmt.Println("Ask AI features may be unavailable. Ensure asset/ contains tokenizer.json, model_int8.onnx, onnxruntime.dll, vec0.dll")
	}

	appDir, err := resolveAppDir()
	if err != nil {
		a.aiInitError = err.Error()
		fmt.Printf("Error resolving app directory: %v\n", err)
		return
	}

	runtimeAssets, err := assetValidator.PrepareRuntimeAssets(appDir)
	if err != nil {
		a.aiInitError = err.Error()
		fmt.Printf("Warning: could not stage runtime assets to app-data: %v\n", err)
	}

	// Initialize persistent database
	dbPath, err := resolveDBPath()
	if err != nil {
		a.aiInitError = err.Error()
		fmt.Printf("Error resolving database path: %v\n", err)
		return
	}

	vec0DllPath := assetValidator.Vec0DllPath()
	if stagedVec0Path, ok := runtimeAssets[filepath.Base(vec0DllPath)]; ok {
		vec0DllPath = stagedVec0Path
	}
	if err := db.Init(dbPath, vec0DllPath); err != nil {
		a.aiInitError = err.Error()
		fmt.Printf("Error initializing database: %v\n", err)
		return
	}
	fmt.Printf("Database initialized at %s\n", dbPath)

	// Initialize ONNX embedder for local vector generation.
	var embedder *embeddings.OnnxEmbedder
	onnxRuntimePath := assetValidator.OnnxRuntimePath()
	if stagedRuntimePath, ok := runtimeAssets[filepath.Base(onnxRuntimePath)]; ok {
		onnxRuntimePath = stagedRuntimePath
	}
	embedder, err = embeddings.NewOnnxEmbedder(assetValidator.ModelPath(), assetValidator.TokenizerPath(), onnxRuntimePath)
	if err != nil {
		a.aiInitError = err.Error()
		fmt.Printf("Warning: could not initialize ONNX embedder: %v\n", err)
		a.aiReady = false
	} else {
		if err := embeddings.InitPromptTokenizer(assetValidator.TokenizerPath()); err != nil {
			a.aiInitError = fmt.Sprintf("could not initialize prompt tokenizer: %v", err)
			fmt.Printf("Warning: %s\n", a.aiInitError)
			a.aiReady = false
			if closeErr := embedder.Close(); closeErr != nil {
				fmt.Printf("Warning: could not close embedder after tokenizer init failure: %v\n", closeErr)
			}
			embedder = nil
		} else {
			a.aiReady = true
			a.aiInitError = ""
			a.embedder = embedder

			if err := db.InitWithVectorDimension(embedder.GetDimension()); err != nil {
				fmt.Printf("Warning: could not initialize vector table; Ask AI will use lexical fallback: %v\n", err)
			} else {
				indexer := rag.NewVectorIndexer(embedder, rag.IndexerConfig{
					RecomputeOnHashMismatch: true,
					ForceReindex:            false,
				})
				go func() {
					if err := indexer.IndexAllTopics(); err != nil {
						fmt.Printf("Warning: vector indexing failed: %v\n", err)
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
		fmt.Printf("Warning: could not list topics for lexical fallback: %v\n", err)
		topicIDs = []string{"os-scheduling"}
	}

	for _, topicID := range topicIDs {
		chunks, err := db.GetChunksForTopic(topicID)
		if err != nil {
			fmt.Printf("Warning: could not load chunks for topic %s: %v\n", topicID, err)
			continue
		}
		fmt.Printf("Loaded %d chunks for topic %s\n", len(chunks), topicID)
		for _, chunk := range chunks {
			embedStore.AddChunk(chunk)
		}
	}

	// Initialize LLM provider from .env / environment variables.
	llmConfig := llm.LoadConfigFromEnv()

	llmProvider := llm.NewProvider(llmConfig)
	a.llmProvider = llmProvider

	// Create RAG pipeline
	a.ragPipeline = rag.NewPipeline(embedStore, llmProvider)
	a.scheduler = scheduler.New()

	// Initialize notebook service
	notebookDir, err := resolveNotebookDir()
	if err != nil {
		fmt.Printf("Error resolving notebook directory: %v\n", err)
		return
	}
	a.notebookUploadDir = notebookDir
	a.notebookService = notebook.NewService(notebookDir)
	fmt.Printf("Notebook service initialized at %s\n", notebookDir)

	fmt.Println("App initialized successfully")
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
		"topic_id":       bundle.TopicID,
		"topic_title":    bundle.TopicTitle,
		"notebook_id":    bundle.NotebookID,
		"notebook_title": bundle.NotebookTitle,
		"notebook_url":   bundle.NotebookURL,
		"file_type":      bundle.FileType,
		"page_count":     bundle.PageCount,
		"sections":       lightSections,
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

	result, err := a.ragPipeline.ProcessQuery(topicID, question)
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

	if a.llmProvider == nil {
		return map[string]interface{}{
			"error": "LLM provider not initialized",
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

	answer, err := a.llmProvider.GenerateAnswer(prompt)
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

	plan, err := a.scheduler.BuildTodayPlan(time.Now())
	if err != nil {
		return map[string]interface{}{
			"error": err.Error(),
		}
	}

	return map[string]interface{}{
		"date":             plan.Date,
		"total_minutes":    plan.TotalMinutes,
		"review_minutes":   plan.ReviewMinutes,
		"learning_minutes": plan.LearningMinutes,
		"due_review_cards": plan.DueReviewCards,
		"active_topics":    plan.ActiveTopics,
		"tasks":            plan.Tasks,
	}
}

type quizLLMQuestion struct {
	Prompt        string   `json:"prompt"`
	Options       []string `json:"options"`
	CorrectAnswer string   `json:"correct_answer"`
	Explanation   string   `json:"explanation"`
	Hint          string   `json:"hint"`
	SourceHeading string   `json:"source_heading"`
	SourceSnippet string   `json:"source_snippet"`
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

// GenerateQuiz creates topic-scoped multiple-choice questions and stores them.
// Enforces exactly 5 questions per generation to keep quizzes consistent.
func (a *App) GenerateQuiz(topicID string) map[string]interface{} {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return map[string]interface{}{"error": "topic ID is required"}
	}

	if a.llmProvider == nil {
		return map[string]interface{}{"error": "LLM provider not initialized"}
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
	raw, err := a.llmProvider.GenerateAnswer(prompt)
	if err != nil {
		return map[string]interface{}{"error": "quiz generation failed: " + err.Error()}
	}

	parsed, err := parseQuizLLMResponse(raw)
	if err != nil {
		return map[string]interface{}{"error": "quiz generation parsing failed: " + err.Error()}
	}

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

		questions = append(questions, models.QuizQuestion{
			ID:            uuid.NewString(),
			TopicID:       topicID,
			Prompt:        strings.TrimSpace(q.Prompt),
			Options:       q.Options,
			CorrectAnswer: matchedOption,
			Explanation:   strings.TrimSpace(q.Explanation),
			Hint:          strings.TrimSpace(q.Hint),
			SourceHeading: strings.TrimSpace(q.SourceHeading),
			SourceSnippet: strings.TrimSpace(q.SourceSnippet),
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

// GenerateFlashcards creates flashcards for a topic or returns the existing set.
// Enforces exactly 8 flashcards per generation to keep decks consistent and predictable.
func (a *App) GenerateFlashcards(topicID string) map[string]interface{} {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return map[string]interface{}{"error": "topic ID is required"}
	}

	if a.llmProvider == nil {
		return map[string]interface{}{"error": "LLM provider not initialized"}
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
	raw, err := a.llmProvider.GenerateAnswer(buildFlashcardPrompt(topicID, sections))
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

	// Limit to top 5 sections with truncation to avoid exceeding token limits
	const (
		maxSections          = 5
		maxContentPerSection = 500
		maxTotalContent      = 2500
	)

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

		// Truncate section to max content size (rune-safe to preserve UTF-8)
		if len(content) > maxContentPerSection {
			runes := []rune(content)
			if len(runes) > maxContentPerSection {
				content = string(runes[:maxContentPerSection]) + "..."
			}
		}

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

	const (
		maxSections          = 5
		maxContentPerSection = 500
		maxTotalContent      = 2500
	)

	totalContentLength := 0
	sectionCount := 0
	for _, section := range sections {
		if sectionCount >= maxSections {
			break
		}

		remainingBudget := maxTotalContent - totalContentLength
		if remainingBudget <= 0 {
			break
		}

		heading, _ := section["heading"].(string)
		content, _ := section["content"].(string)
		if strings.TrimSpace(content) == "" {
			continue
		}

		runes := []rune(content)
		ellipsisRunes := len([]rune("..."))
		allowedRunes := minInt(maxContentPerSection, remainingBudget)
		if allowedRunes <= 0 {
			break
		}

		appendedRunes := len(runes)
		wasTrimmed := false
		if appendedRunes > allowedRunes {
			if allowedRunes <= ellipsisRunes {
				break
			}
			appendedRunes = allowedRunes - ellipsisRunes
			if appendedRunes <= 0 {
				break
			}
			wasTrimmed = true
		}

		content = string(runes[:appendedRunes])
		if wasTrimmed {
			content += "..."
		}

		b.WriteString("[Heading] ")
		b.WriteString(heading)
		b.WriteString("\n")
		b.WriteString(content)
		b.WriteString("\n---\n")

		totalContentLength += appendedRunes
		if wasTrimmed {
			totalContentLength += ellipsisRunes
		}
		sectionCount++
	}

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

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
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
	path := strings.TrimSpace(filepath.ToSlash(filePath))
	if path == "" || path == "." || path == ".." {
		return ""
	}
	return "/notebooks/" + url.PathEscape(path)
}
