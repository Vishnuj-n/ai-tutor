package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"ai-tutor/internal/llm"
	"ai-tutor/internal/models"
	"ai-tutor/internal/study"
	"ai-tutor/internal/utils"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)


func (a *App) GetUserSettings() map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	s, err := repo.GetUserSettings()
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{
		"max_flashcards_per_session": s.MaxFlashcardsPerSession,
		"study_start_time":           s.StudyStartTime,
		"study_end_time":             s.StudyEndTime,
		"reminders_enabled":          s.RemindersEnabled,
		"active_profile_id":          s.ActiveProfileID,
		"skip_to_reading_active":     s.SkipToReadingActive,
		"cloud_sync_url":             s.CloudSyncURL,
		"cloud_api_token":            s.CloudAPIToken,
		"theme":                      s.Theme,
		"rag_enabled":                s.RAGEnabled,
		"rag_notebook_chapter":       s.RAGNotebookChapter,
		"rag_entire_notebook":        s.RAGEntireNotebook,
		"rag_queue_study":            s.RAGQueueStudy,
		"default_remedial_strategy":  s.DefaultRemedialStrategy,
		"classroom_code":             s.ClassroomCode,
		"student_username":           s.StudentUsername,
	}
}

func (a *App) UpdateUserSettings(maxFlashcards int, startTime string, endTime string, remindersEnabled bool, activeProfileID string, skipToReading bool, syncURL, apiToken string, theme string, ragEnabled bool, ragNotebookChapter bool, ragEntireNotebook bool, ragQueueStudy bool, defaultRemedialStrategy string, classroomCode string) map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	if maxFlashcards < 5 || maxFlashcards > 200 {
		return map[string]interface{}{"error": "max flashcards per session must be between 5 and 200"}
	}
	if _, err := time.Parse("15:04", startTime); err != nil {
		return map[string]interface{}{"error": "invalid study start time: must match format HH:MM"}
	}
	if _, err := time.Parse("15:04", endTime); err != nil {
		return map[string]interface{}{"error": "invalid study end time: must match format HH:MM"}
	}
	if defaultRemedialStrategy == "" {
		defaultRemedialStrategy = "CLASSIC"
	}
	if defaultRemedialStrategy != "CLASSIC" && defaultRemedialStrategy != "FAST" {
		return map[string]interface{}{"error": "default remedial strategy must be CLASSIC or FAST"}
	}
	s := models.UserSettings{
		MaxFlashcardsPerSession: maxFlashcards,
		StudyStartTime:          startTime,
		StudyEndTime:            endTime,
		RemindersEnabled:        remindersEnabled,
		ActiveProfileID:         activeProfileID,
		SkipToReadingActive:     skipToReading,
		CloudSyncURL:            syncURL,
		CloudAPIToken:           apiToken,
		Theme:                   theme,
		RAGEnabled:              ragEnabled,
		RAGNotebookChapter:      ragNotebookChapter,
		RAGEntireNotebook:       ragEntireNotebook,
		RAGQueueStudy:           ragQueueStudy,
		DefaultRemedialStrategy: defaultRemedialStrategy,
		ClassroomCode:           classroomCode,
	}
	// Persist settings first so SQLite is never stale if runtime mutation fails.
	if err := repo.UpdateUserSettings(s); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}

	// Only mutate runtime after successful persistence.
	a.aiMutex.Lock()
	if !ragEnabled && a.embedder != nil {
		utils.Infof("RAG disabled dynamically in settings. Closing ONNX embedder.")
		_ = a.embedder.Close()
		a.embedder = nil
		a.aiReady = false
	}
	a.aiMutex.Unlock()

	if !ragEnabled {
		if err := a.reloadRetrievalEngine(); err != nil {
			utils.Errorf("reloadRetrievalEngine after RAG disable: %v", err)
			return map[string]interface{}{"error": "failed to reload retrieval engine: " + err.Error()}
		}
	}

	return map[string]interface{}{"ok": true}
}

func (a *App) GetLLMSettings() map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	settings, err := repo.GetLLMSettings()
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	settings.Fast.HasAPIKey = settings.Fast.HasAPIKey || llm.HasAPIKey("fast") || envHasLLMAPIKey("FAST_LLM")
	settings.Heavy.HasAPIKey = settings.Heavy.HasAPIKey || llm.HasAPIKey("heavy") || envHasLLMAPIKey("HEAVY_LLM")
	settings.UseSameForHeavy = sameLLMSettingsForUI(settings.Fast, settings.Heavy)
	return map[string]interface{}{"settings": settings}
}

func (a *App) GetLLMProviderPreset(provider string) map[string]interface{} {
	provider = strings.TrimSpace(strings.ToLower(provider))
	switch provider {
	case "groq":
		return map[string]interface{}{
			"provider": "groq",
			"base_url": "https://api.groq.com/openai",
			"model":    "openai/gpt-oss-120b",
		}
	case "openai":
		return map[string]interface{}{
			"provider": "openai",
			"base_url": "https://api.openai.com",
			"model":    "gpt-4.1-mini",
		}
	case "openrouter":
		return map[string]interface{}{
			"provider": "openrouter",
			"base_url": "https://openrouter.ai/api",
			"model":    "openai/gpt-4.1-mini",
		}
	default:
		return map[string]interface{}{
			"provider": "custom",
			"base_url": "",
			"model":    "",
		}
	}
}

func (a *App) UpdateLLMSettings(settings models.LLMSettings) map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	current, err := repo.GetLLMSettings()
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	settings.Fast.Tier = "fast"
	if settings.Fast.TimeoutMs <= 0 {
		settings.Fast.TimeoutMs = 30000
	}
	if settings.UseSameForHeavy {
		settings.Heavy = settings.Fast
		settings.Heavy.Tier = "heavy"
	} else {
		settings.Heavy.Tier = "heavy"
		if settings.Heavy.TimeoutMs <= 0 {
			settings.Heavy.TimeoutMs = 90000
		}
	}
	settings.Fast.HasAPIKey = current.Fast.HasAPIKey || llm.HasAPIKey("fast") || envHasLLMAPIKey("FAST_LLM")
	settings.Heavy.HasAPIKey = current.Heavy.HasAPIKey || llm.HasAPIKey("heavy") || envHasLLMAPIKey("HEAVY_LLM")
	if settings.UseSameForHeavy {
		settings.Heavy.HasAPIKey = settings.Fast.HasAPIKey
	}
	if err := repo.UpdateLLMSettings(settings); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	if err := a.reloadLLMProviders(); err != nil {
		return map[string]interface{}{"error": "settings saved but LLM reload failed: " + err.Error()}
	}
	return map[string]interface{}{"ok": true}
}

func (a *App) SaveLLMAPIKey(tier string, key string) map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	tier = normalizeLLMTierForApp(tier)
	if tier == "" {
		return map[string]interface{}{"error": "tier must be fast or heavy"}
	}
	if err := llm.SaveAPIKey(tier, key); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	if err := repo.MarkLLMKeyStored(tier, true); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	if err := a.reloadLLMProviders(); err != nil {
		return map[string]interface{}{"error": "key saved but LLM reload failed: " + err.Error()}
	}
	return map[string]interface{}{"ok": true}
}

func (a *App) DeleteLLMAPIKey(tier string) map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	tier = normalizeLLMTierForApp(tier)
	if tier == "" {
		return map[string]interface{}{"error": "tier must be fast or heavy"}
	}
	if err := llm.DeleteAPIKey(tier); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	if err := repo.MarkLLMKeyStored(tier, false); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	if err := a.reloadLLMProviders(); err != nil {
		return map[string]interface{}{"error": "key deleted but LLM reload failed: " + err.Error()}
	}
	return map[string]interface{}{"ok": true}
}

func (a *App) reloadLLMProviders() error {
	repo := a.getRepo()
	if repo == nil {
		return fmt.Errorf("reloadLLMProviders: repository not initialized")
	}
	settings, err := repo.GetLLMSettings()
	if err != nil {
		return err
	}

	fastKey, _ := llm.GetAPIKey("fast")
	heavyKey, _ := llm.GetAPIKey("heavy")
	if settings.UseSameForHeavy && heavyKey == "" {
		heavyKey = fastKey
	}
	fastProvider := llm.NewProvider(llm.LoadConfigFromSettingsForPrefix("FAST_LLM", settings.Fast, fastKey))
	heavyProvider := llm.NewProvider(llm.LoadConfigFromSettingsForPrefix("HEAVY_LLM", settings.Heavy, heavyKey))

	a.aiMutex.Lock()
	a.fastLLMProvider = fastProvider
	a.heavyLLMProvider = heavyProvider
	engine := a.retrievalEngine
	a.studyService = study.NewStudyService(study.Config{
		Repo:             repo,
		FastLLMProvider:  fastProvider,
		HeavyLLMProvider: heavyProvider,
		RetrievalEngine:  engine,
	})
	a.aiMutex.Unlock()
	return nil
}

func normalizeLLMTierForApp(tier string) string {
	tier = strings.TrimSpace(strings.ToLower(tier))
	switch tier {
	case "fast", "heavy":
		return tier
	default:
		return ""
	}
}

func envHasLLMAPIKey(prefix string) bool {
	prefix = strings.TrimSuffix(strings.TrimSpace(prefix), "_")
	keys := []string{"LLM_API_KEY", "OPENAI_API_KEY", "API_KEY"}
	for _, key := range keys {
		if prefix != "" && strings.TrimSpace(os.Getenv(prefix+"_"+key)) != "" {
			return true
		}
		if strings.TrimSpace(os.Getenv(key)) != "" {
			return true
		}
	}
	return false
}

func sameLLMSettingsForUI(a, b models.LLMTierSettings) bool {
	return strings.EqualFold(a.Provider, b.Provider) &&
		strings.TrimSpace(a.BaseURL) == strings.TrimSpace(b.BaseURL) &&
		strings.TrimSpace(a.Model) == strings.TrimSpace(b.Model) &&
		a.TimeoutMs == b.TimeoutMs &&
		a.HasAPIKey == b.HasAPIKey
}

func (a *App) GetProfiles() map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	profiles, err := repo.GetProfiles()
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"profiles": profiles}
}

func (a *App) CreateProfile(name string, deadlineStr string) map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return map[string]interface{}{"error": "profile name is required"}
	}
	deadlineTime, err := time.Parse("2006-01-02", deadlineStr)
	if err != nil {
		return map[string]interface{}{"error": "failed to parse deadline: " + err.Error()}
	}
	p := models.StudyProfile{
		ID:         uuid.NewString(),
		Name:       name,
		DeadlineAt: deadlineTime.Unix(),
	}
	if err := repo.CreateProfile(p); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}

	// If no active profile is set yet, make this the default automatically.
	// First profile created = default active profile.
	s, err := repo.GetUserSettings()
	if err == nil && s != nil && s.ActiveProfileID == "" {
		s.ActiveProfileID = p.ID
		if err := repo.UpdateUserSettings(*s); err != nil {
			return map[string]interface{}{"error": "profile created but failed to set as active: " + err.Error()}
		}
	}

	return map[string]interface{}{"ok": true, "profile": p}
}

func (a *App) UpdateProfile(id string, name string, deadlineStr string) map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	id = strings.TrimSpace(id)
	name = strings.TrimSpace(name)
	if id == "" || name == "" {
		return map[string]interface{}{"error": "id and name are required"}
	}
	deadlineTime, err := time.Parse("2006-01-02", deadlineStr)
	if err != nil {
		return map[string]interface{}{"error": "failed to parse deadline: " + err.Error()}
	}
	p := models.StudyProfile{
		ID:         id,
		Name:       name,
		DeadlineAt: deadlineTime.Unix(),
	}
	if err := repo.UpdateProfile(p); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"ok": true}
}

func (a *App) DeleteProfile(id string) map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return map[string]interface{}{"error": "profile id is required"}
	}
	if err := repo.DeleteProfile(id); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"ok": true}
}

func (a *App) AssignNotebookToProfile(notebookID, profileID string) map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	if err := repo.AssignNotebookToProfile(notebookID, profileID); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"ok": true}
}

func (a *App) UpdateNotebookStudyStatus(notebookID, studyStatus string) map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	if err := repo.UpdateNotebookStudyStatus(notebookID, studyStatus); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"ok": true}
}

func (a *App) IsOnboarded() map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized", "onboarded": false}
	}
	profiles, err := repo.GetProfiles()
	if err != nil {
		return map[string]interface{}{"error": err.Error(), "onboarded": false}
	}
	onboarded := len(profiles) > 0
	return map[string]interface{}{"onboarded": onboarded}
}

func (a *App) TriggerCloudSync() map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	if err := study.TriggerCloudSync(repo); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"ok": true}
}

// app_settings.go end

// LoginStudent handles student login using the Supabase login_user RPC.
func (a *App) LoginStudent(username, password, classroomCode string) map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	settings, err := repo.GetUserSettings()
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}

	syncURL := study.ResolveCloudSyncURL(settings.CloudSyncURL)
	if syncURL == "" {
		syncURL = os.Getenv("CLOUD_SYNC_URL")
	}
	if syncURL == "" {
		return map[string]interface{}{"error": "Supabase Sync URL is not configured in the environment"}
	}

	anonKey := os.Getenv("CLOUD_API_TOKEN")
	if anonKey == "" {
		anonKey = os.Getenv("SUPABASE_ANON_KEY")
	}
	if anonKey == "" {
		anonKey = settings.CloudAPIToken
	}
	if anonKey == "" {
		return map[string]interface{}{"error": "Supabase Anon Key is not configured in the environment"}
	}

	baseURL := syncURL
	if strings.Contains(baseURL, "/rest/v1/rpc/") {
		idx := strings.Index(baseURL, "/rest/v1/")
		baseURL = baseURL[:idx]
	}
	loginURL := fmt.Sprintf("%s/rest/v1/rpc/login_user", strings.TrimSuffix(baseURL, "/"))

	type LoginPayload struct {
		Username  string `json:"p_username"`
		Password  string `json:"p_password"`
		IsDesktop bool   `json:"p_is_desktop"`
	}
	payload := LoginPayload{
		Username:  username,
		Password:  password,
		IsDesktop: true,
	}
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return map[string]interface{}{"error": "failed to encode login payload"}
	}

	req, err := http.NewRequest("POST", loginURL, strings.NewReader(string(jsonBytes)))
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", anonKey)
	req.Header.Set("Authorization", "Bearer "+anonKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return map[string]interface{}{"error": "network error: " + err.Error()}
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return map[string]interface{}{"error": fmt.Sprintf("login failed: %s", string(bodyBytes))}
	}

	var loginResp struct {
		SessionToken  string `json:"session_token"`
		Role          string `json:"role"`
		ClassroomCode string `json:"classroom_code"`
		Username      string `json:"username"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return map[string]interface{}{"error": "failed to parse login response: " + err.Error()}
	}

	settings.CloudAPIToken = loginResp.SessionToken
	settings.ClassroomCode = loginResp.ClassroomCode
	settings.StudentUsername = loginResp.Username
	if settings.CloudSyncURL == "" {
		settings.CloudSyncURL = fmt.Sprintf("%s/rest/v1/rpc/handle_cloud_sync", strings.TrimSuffix(baseURL, "/"))
	}

	if err := repo.UpdateUserSettings(*settings); err != nil {
		return map[string]interface{}{"error": "failed to save settings: " + err.Error()}
	}

	go func() {
		if syncErr := study.TriggerCloudSync(repo); syncErr != nil {
			utils.Warnf("[LOGIN] initial post-login sync warning: %v", syncErr)
		}
	}()

	return map[string]interface{}{
		"ok":             true,
		"session_token":  loginResp.SessionToken,
		"classroom_code": loginResp.ClassroomCode,
		"username":       loginResp.Username,
	}
}

// LogoutStudent signs out the student by clearing saved sync credentials from the SQLite store.
func (a *App) LogoutStudent() map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	settings, err := repo.GetUserSettings()
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	settings.CloudAPIToken = ""
	settings.ClassroomCode = ""
	settings.StudentUsername = ""
	if err := repo.UpdateUserSettings(*settings); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"ok": true}
}

// GetCloudConfig returns whether cloud sync is currently configured (either
// via the stored SQLite setting or the CLOUD_SYNC_URL env var). It does NOT
// expose the actual URL, so the frontend can use this to decide whether to show
// the "Sync with Cloud Now" button without leaking the server address.
func (a *App) GetCloudConfig() map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"configured": false}
	}
	settings, err := repo.GetUserSettings()
	if err != nil {
		return map[string]interface{}{"configured": false}
	}
	resolved := study.ResolveCloudSyncURL(settings.CloudSyncURL)
	return map[string]interface{}{
		"configured": resolved != "",
	}
}
