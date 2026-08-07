package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"ai-tutor/internal/llm"
	"ai-tutor/internal/models"
	"ai-tutor/internal/study"
	"ai-tutor/internal/utils"

	"github.com/google/uuid"
)

func (a *App) GetUserSettings() map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": errDatabaseNotInitialized}
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
		"analytics_enabled":          s.AnalyticsEnabled,
		"anonymous_user_id":          s.AnonymousUserID,
		"target_session_words":       s.TargetSessionWords,
	}
}

func (a *App) UpdateUserSettings(s models.UserSettings) map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": errDatabaseNotInitialized}
	}
	if s.MaxFlashcardsPerSession < 5 || s.MaxFlashcardsPerSession > 200 {
		return map[string]interface{}{"error": "max flashcards per session must be between 5 and 200"}
	}
	if s.TargetSessionWords > 0 {
		if s.TargetSessionWords < 1000 || s.TargetSessionWords > 20000 || s.TargetSessionWords%500 != 0 {
			return map[string]interface{}{"error": "target session words must be between 1000 and 20000 and a multiple of 500"}
		}
	} else {
		s.TargetSessionWords = 5000
	}
	if s.StudyStartTime != "" {
		if _, err := time.Parse("15:04", s.StudyStartTime); err != nil {
			return map[string]interface{}{"error": "invalid study start time: must match format HH:MM"}
		}
	}
	if s.StudyEndTime != "" {
		if _, err := time.Parse("15:04", s.StudyEndTime); err != nil {
			return map[string]interface{}{"error": "invalid study end time: must match format HH:MM"}
		}
	}
	if s.DefaultRemedialStrategy == "" {
		s.DefaultRemedialStrategy = "CLASSIC"
	}
	if s.DefaultRemedialStrategy != "FAST" && s.DefaultRemedialStrategy != "CLASSIC" {
		return map[string]interface{}{"error": "default remedial strategy must be CLASSIC or FAST"}
	}
	// Persist settings first so SQLite is never stale if runtime mutation fails.
	if err := repo.UpdateUserSettings(s); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}

	// Only mutate runtime after successful persistence.
	a.aiMutex.Lock()
	if !s.RAGEnabled && a.embedder != nil {
		utils.Infof("RAG disabled dynamically in settings. Closing ONNX embedder.")
		if err := a.embedder.Close(); err != nil {
			a.aiMutex.Unlock()
			return map[string]interface{}{"error": fmt.Sprintf("failed to close embedder: %v", err)}
		}
		a.embedder = nil
		a.aiReady = false
	}
	a.aiMutex.Unlock()

	if !s.RAGEnabled {
		if err := a.reloadRetrievalEngine(); err != nil {
			utils.Errorf("reloadRetrievalEngine after RAG disable: %v", err)
			return map[string]interface{}{"error": "failed to reload retrieval engine: " + err.Error()}
		}
	}

	return map[string]interface{}{"ok": true}
}

func (a *App) TrackAnalyticsEvent(eventType, fileHash string, pageNumber int, metadata string) map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": errDatabaseNotInitialized}
	}
	if err := repo.TrackAnalyticsEvent(eventType, fileHash, pageNumber, metadata); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"ok": true}
}

func (a *App) GetLLMSettings() map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": errDatabaseNotInitialized}
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
		return map[string]interface{}{"error": errDatabaseNotInitialized}
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
		return map[string]interface{}{"error": errDatabaseNotInitialized}
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
		return map[string]interface{}{"error": errDatabaseNotInitialized}
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
		return map[string]interface{}{"error": errDatabaseNotInitialized}
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
		return map[string]interface{}{"error": errDatabaseNotInitialized}
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return map[string]interface{}{"error": "profile name is required"}
	}
	deadlineTime, err := time.Parse(dateFormatYYYYMMDD, deadlineStr)
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
		return map[string]interface{}{"error": errDatabaseNotInitialized}
	}
	id = strings.TrimSpace(id)
	name = strings.TrimSpace(name)
	if id == "" || name == "" {
		return map[string]interface{}{"error": "id and name are required"}
	}
	deadlineTime, err := time.Parse(dateFormatYYYYMMDD, deadlineStr)
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
		return map[string]interface{}{"error": errDatabaseNotInitialized}
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
		return map[string]interface{}{"error": errDatabaseNotInitialized}
	}
	if err := repo.AssignNotebookToProfile(notebookID, profileID); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"ok": true}
}

func (a *App) UpdateNotebookStudyStatus(notebookID, studyStatus string) map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": errDatabaseNotInitialized}
	}
	if err := repo.UpdateNotebookStudyStatus(notebookID, studyStatus); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"ok": true}
}

func (a *App) IsOnboarded() map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": errDatabaseNotInitialized, "onboarded": false}
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
		return map[string]interface{}{"error": errDatabaseNotInitialized}
	}
	if err := study.TriggerCloudSync(repo); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"ok": true}
}

// app_settings.go end

// LoginStudent handles student login via Go cloud-server or direct Supabase REST user_accounts.
func (a *App) LoginStudent(username, password string) map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": errDatabaseNotInitialized}
	}
	settings, err := repo.GetUserSettings()
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}

	syncURL := study.ResolveCloudSyncURL(settings.CloudSyncURL)
	if syncURL == "" {
		return map[string]interface{}{"error": "Cloud connection URL is not configured"}
	}
	anonKey := study.ResolveAnonKey()

	client := &http.Client{Timeout: 10 * time.Second}
	var loginResp struct {
		SessionToken  string `json:"session_token"`
		Role          string `json:"role"`
		ClassroomCode string `json:"classroom_code"`
		Username      string `json:"username"`
	}

	baseURL := syncURL
	if idx := strings.Index(baseURL, "/rest/v1/"); idx != -1 {
		baseURL = baseURL[:idx]
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	// 1. Try Go cloud server endpoint /api/auth/login
	serverURL := study.ResolveCloudServerURL()
	if serverURL == study.DefaultProductionCloudServerURL && baseURL != "" && !strings.Contains(baseURL, "your-supabase-project") {
		serverURL = baseURL
	}

	payload := map[string]interface{}{
		"username":   username,
		"password":   password,
		"is_desktop": true,
	}
	jsonBytes, _ := json.Marshal(payload)

	var authenticated bool
	req, reqErr := http.NewRequest("POST", fmt.Sprintf("%s/api/auth/login", serverURL), bytes.NewBuffer(jsonBytes))
	if reqErr == nil {
		req.Header.Set("Content-Type", "application/json")
		if anonKey != "" {
			req.Header.Set("apikey", anonKey)
		}
		resp, err := client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			if json.NewDecoder(resp.Body).Decode(&loginResp) == nil && loginResp.SessionToken != "" {
				authenticated = true
			}
		}
	}

	// 2. Fallback: query Supabase REST user_accounts table or mock server directly
	if !authenticated {
		tableURL := fmt.Sprintf("%s/rest/v1/user_accounts?username=eq.%s&select=*", baseURL, url.QueryEscape(username))
		req, err := http.NewRequest("GET", tableURL, nil)
		if err == nil {
			if anonKey != "" {
				req.Header.Set("apikey", anonKey)
				if strings.Count(anonKey, ".") == 2 {
					req.Header.Set("Authorization", "Bearer "+anonKey)
				}
			}
			resp, err := client.Do(req)
			if err == nil {
				defer resp.Body.Close()
				bodyBytes, _ := io.ReadAll(resp.Body)
				if resp.StatusCode == http.StatusOK {
					if json.Unmarshal(bodyBytes, &loginResp) == nil && loginResp.SessionToken != "" {
						authenticated = true
					} else {
						var users []map[string]interface{}
						if json.Unmarshal(bodyBytes, &users) == nil && len(users) > 0 {
							user := users[0]
							pwd, _ := user["password_hash"].(string)
							if pwd == "" {
								pwd, _ = user["password"].(string)
							}
							if pwd == password {
								role, _ := user["role"].(string)
								classCode, _ := user["classroom_code"].(string)
								uname, _ := user["username"].(string)
								sessToken := fmt.Sprintf("%v", user["id"])
								if sessToken == "" || sessToken == "<nil>" {
									sessToken = uname
								}
								loginResp.SessionToken = sessToken
								loginResp.Role = role
								loginResp.ClassroomCode = classCode
								loginResp.Username = uname
								authenticated = true
							}
						}
					}
				}
			}
		}
	}

	if !authenticated {
		return map[string]interface{}{"error": "invalid username or password"}
	}

	settings.CloudAPIToken = loginResp.SessionToken
	settings.ClassroomCode = loginResp.ClassroomCode
	settings.StudentUsername = loginResp.Username
	if settings.CloudSyncURL == "" {
		settings.CloudSyncURL = fmt.Sprintf("%s/rest/v1/rpc/handle_cloud_sync", strings.TrimSuffix(baseURL, "/"))
	}

	// Match or create dedicated Study Profile for this classroom
	profiles, err := repo.GetProfiles()
	if err != nil {
		return map[string]interface{}{"error": "failed to query study profiles: " + err.Error()}
	}

	var targetProfileID string
	for _, p := range profiles {
		if strings.EqualFold(p.ClassroomCode, loginResp.ClassroomCode) && loginResp.ClassroomCode != "" {
			targetProfileID = p.ID
			break
		}
	}

	if targetProfileID != "" {
		if err := repo.UpdateProfileCloudCredentials(targetProfileID, loginResp.ClassroomCode, loginResp.Username, loginResp.SessionToken); err != nil {
			log.Printf("warning: failed to save profile cloud credentials: %v", err)
		}
	} else {
		// Automatically create a new study profile for this new classroom
		profileName := loginResp.ClassroomCode
		if profileName == "" {
			profileName = loginResp.Username
		}
		if profileName == "" {
			profileName = "Classroom Profile"
		}
		newProfile := models.StudyProfile{
			ID:              uuid.NewString(),
			Name:            profileName,
			DeadlineAt:      time.Now().AddDate(0, 3, 0).Unix(),
			ClassroomCode:   loginResp.ClassroomCode,
			StudentUsername: loginResp.Username,
			CloudAPIToken:   loginResp.SessionToken,
		}
		if err := repo.CreateProfile(newProfile); err != nil {
			return map[string]interface{}{"error": "failed to create classroom study profile: " + err.Error()}
		}
		targetProfileID = newProfile.ID
	}

	settings.ActiveProfileID = targetProfileID
	if err := repo.UpdateUserSettings(*settings); err != nil {
		return map[string]interface{}{"error": "failed to save settings: " + err.Error()}
	}

	return map[string]interface{}{
		"ok":             true,
		"session_token":  loginResp.SessionToken,
		"classroom_code": loginResp.ClassroomCode,
		"username":       loginResp.Username,
		"profile_id":     targetProfileID,
	}
}

// SignUpStudent handles new student account registration using Go cloud-server API or direct Supabase user_accounts REST insert.
func (a *App) SignUpStudent(username, password, classroomCode string) map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": errDatabaseNotInitialized}
	}
	settings, err := repo.GetUserSettings()
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}

	client := &http.Client{Timeout: 10 * time.Second}
	var signedUp bool

	serverURL := study.ResolveCloudServerURL()
	if serverURL != "" {
		signupURL := fmt.Sprintf("%s/api/auth/signup", serverURL)
		payload := map[string]string{
			"username":       username,
			"password":       password,
			"role":           "student",
			"classroom_code": classroomCode,
		}
		jsonBytes, _ := json.Marshal(payload)
		req, reqErr := http.NewRequest("POST", signupURL, bytes.NewBuffer(jsonBytes))
		if reqErr == nil {
			req.Header.Set("Content-Type", "application/json")
			resp, err := client.Do(req)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					signedUp = true
				}
			}
		}
	}

	// Fallback to direct Supabase REST table insert if cloud-server unreachable
	if !signedUp {
		syncURL := study.ResolveCloudSyncURL(settings.CloudSyncURL)
		if syncURL == "" {
			return map[string]interface{}{"error": "Cloud connection URL is not configured"}
		}
		anonKey := study.ResolveAnonKey()
		if anonKey == "" {
			return map[string]interface{}{"error": "Supabase Anon Key is not configured in environment"}
		}

		baseURL := syncURL
		if idx := strings.Index(baseURL, "/rest/v1/"); idx != -1 {
			baseURL = baseURL[:idx]
		}
		insertURL := fmt.Sprintf("%s/rest/v1/user_accounts", strings.TrimSuffix(baseURL, "/"))

		newUser := map[string]string{
			"username":       username,
			"password_hash":  password,
			"role":           "student",
			"classroom_code": classroomCode,
		}
		jsonBytes, _ := json.Marshal(newUser)
		req, err := http.NewRequest("POST", insertURL, bytes.NewBuffer(jsonBytes))
		if err != nil {
			return map[string]interface{}{"error": err.Error()}
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("apikey", anonKey)
		if strings.Count(anonKey, ".") == 2 {
			req.Header.Set("Authorization", "Bearer "+anonKey)
		}
		req.Header.Set("Prefer", "return=representation")

		resp, err := client.Do(req)
		if err != nil {
			return map[string]interface{}{"error": "network error: " + err.Error()}
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			return map[string]interface{}{"error": fmt.Sprintf("signup failed: %s", string(bodyBytes))}
		}
	}

	// On successful signup, perform immediate login to establish active session token
	return a.LoginStudent(username, password)
}

// LogoutStudent signs out the student by clearing saved sync credentials from the SQLite store.
func (a *App) LogoutStudent() map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": errDatabaseNotInitialized}
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
	if settings.ActiveProfileID != "" {
		if err := repo.UpdateProfileCloudCredentials(settings.ActiveProfileID, "", "", ""); err != nil {
			log.Printf("warning: failed to clear active profile cloud credentials: %v", err)
		}
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
