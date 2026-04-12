package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
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

// GenerateQuiz creates topic-scoped multiple-choice questions and stores them.
func (a *App) GenerateQuiz(topicID string) map[string]interface{} {
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

	questions := make([]models.QuizQuestion, 0, len(parsed.Questions))
	for _, q := range parsed.Questions {
		if strings.TrimSpace(q.Prompt) == "" || len(q.Options) < 2 || strings.TrimSpace(q.CorrectAnswer) == "" {
			continue
		}
		questions = append(questions, models.QuizQuestion{
			ID:            uuid.NewString(),
			TopicID:       topicID,
			Prompt:        strings.TrimSpace(q.Prompt),
			Options:       q.Options,
			CorrectAnswer: strings.TrimSpace(q.CorrectAnswer),
			Explanation:   strings.TrimSpace(q.Explanation),
			SourceHeading: strings.TrimSpace(q.SourceHeading),
			SourceSnippet: strings.TrimSpace(q.SourceSnippet),
		})
	}

	if len(questions) == 0 {
		return map[string]interface{}{"error": "quiz generation produced no valid questions"}
	}

	if err := db.ReplaceQuestionsForTopic(topicID, questions); err != nil {
		return map[string]interface{}{"error": "failed to save generated quiz: " + err.Error()}
	}

	return map[string]interface{}{
		"topic_id":  topicID,
		"questions": questions,
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

	score := models.QuizScore{
		QuestionID:    question.ID,
		Correct:       correct,
		Score:         0,
		Expected:      question.CorrectAnswer,
		Feedback:      question.Explanation,
		Hint:          "Review the cited section and compare each option against the source.",
		UserAnswer:    userAnswer,
		SourceHeading: question.SourceHeading,
	}
	if correct {
		score.Score = 100
		score.Hint = "Great job. Move to the next question."
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
	b.WriteString("You are an AI tutor quiz generator. Return STRICT JSON only. No markdown.\\n")
	b.WriteString("Generate exactly 5 multiple-choice questions for topic: ")
	b.WriteString(topicID)
	b.WriteString("\\nJSON format: {\\\"questions\\\":[{\\\"prompt\\\":string,\\\"options\\\":[string,string,string,string],\\\"correct_answer\\\":string,\\\"explanation\\\":string,\\\"hint\\\":string,\\\"source_heading\\\":string,\\\"source_snippet\\\":string}]}\\n")
	b.WriteString("Rules: correct_answer must match one option exactly; keep options concise; explanations grounded in source text.\\n")
	b.WriteString("Source sections:\\n")
	for _, section := range sections {
		heading, _ := section["heading"].(string)
		content, _ := section["content"].(string)
		if strings.TrimSpace(content) == "" {
			continue
		}
		b.WriteString("[Heading] ")
		b.WriteString(heading)
		b.WriteString("\\n")
		b.WriteString(content)
		b.WriteString("\\n---\\n")
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
