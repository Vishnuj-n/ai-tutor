package main

import (
	"ai-tutor/internal/llm"
	"ai-tutor/internal/models"
	"ai-tutor/internal/study"
	"ai-tutor/internal/utils"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (a *App) GetDailyStudySettings() map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	minutes, err := repo.GetDailyStudyMinutes()
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"daily_study_minutes": minutes}
}

func (a *App) UpdateDailyStudyMinutes(minutes int) map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	if minutes < 15 || minutes > 480 {
		return map[string]interface{}{"error": "daily study minutes must be between 15 and 480"}
	}
	if err := repo.UpsertDailyStudyMinutes(minutes); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"ok": true, "daily_study_minutes": minutes}
}
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
		"daily_study_minutes":    s.DailyStudyMinutes,
		"active_profile_id":      s.ActiveProfileID,
		"skip_to_reading_active": s.SkipToReadingActive,
		"cloud_sync_url":         s.CloudSyncURL,
		"cloud_api_token":        s.CloudAPIToken,
		"theme":                  s.Theme,
		"rag_enabled":            s.RAGEnabled,
		"rag_notebook_chapter":   s.RAGNotebookChapter,
		"rag_entire_notebook":    s.RAGEntireNotebook,
		"rag_queue_study":        s.RAGQueueStudy,
	}
}

func (a *App) UpdateUserSettings(minutes int, activeProfileID string, skipToReading bool, syncURL, apiToken string, theme string, ragEnabled bool, ragNotebookChapter bool, ragEntireNotebook bool, ragQueueStudy bool) map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	if minutes < 15 || minutes > 480 {
		return map[string]interface{}{"error": "daily study minutes must be between 15 and 480"}
	}
	s := models.UserSettings{
		DailyStudyMinutes:   minutes,
		ActiveProfileID:     activeProfileID,
		SkipToReadingActive: skipToReading,
		CloudSyncURL:        syncURL,
		CloudAPIToken:       apiToken,
		Theme:               theme,
		RAGEnabled:          ragEnabled,
		RAGNotebookChapter:  ragNotebookChapter,
		RAGEntireNotebook:   ragEntireNotebook,
		RAGQueueStudy:       ragQueueStudy,
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
		settings.Heavy.TimeoutMs = 90000
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
	settings, err := a.repo.GetLLMSettings()
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
		Repo:             a.repo,
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
	if err := study.TriggerCloudSync(a.repo); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"ok": true}
}

// app_settings.go end
