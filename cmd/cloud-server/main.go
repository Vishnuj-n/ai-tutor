package main

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

	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

var (
	supabaseURL string
	supabaseKey string
	httpClient  = &http.Client{Timeout: 10 * time.Second}
)

type Response struct {
	Error string      `json:"error,omitempty"`
	Data  interface{} `json:"data,omitempty"`
}

func main() {
	_ = godotenv.Load()

	supabaseURL = os.Getenv("SUPABASE_URL")
	supabaseKey = os.Getenv("SUPABASE_SERVICE_ROLE_KEY")
	if supabaseKey == "" {
		supabaseKey = os.Getenv("SUPABASE_PUBLISHABLE_KEY")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	if supabaseURL != "" && supabaseKey != "" {
		log.Printf("[INFO] Cloud Server initialized with Supabase REST API at: %s", supabaseURL)
	} else {
		log.Println("[WARN] SUPABASE_URL or SUPABASE_PUBLISHABLE_KEY is not set in environment.")
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/api/auth/login", handleLogin)
	mux.HandleFunc("/api/auth/signup", handleSignup)
	mux.HandleFunc("/api/dashboard", handleDashboard)
	mux.HandleFunc("/api/assignments", handleAssignments)
	mux.HandleFunc("/api/sync", handleSync)

	handler := corsMiddleware(mux)

	log.Printf("[INFO] Cloud Server running on http://localhost:%s", port)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatalf("[FATAL] Server failed: %v", err)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, apikey, x-session-token, X-Session-Token")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func jsonResponse(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func jsonError(w http.ResponseWriter, status int, message string) {
	jsonResponse(w, status, map[string]interface{}{
		"error":   message,
		"success": false,
	})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, http.StatusOK, map[string]string{"status": "ok", "service": "ai-tutor-cloud-server", "mode": "supabase-rest"})
}

type LoginRequest struct {
	Username  string `json:"username"`
	Password  string `json:"password"`
	IsDesktop bool   `json:"is_desktop"`
}

func checkPassword(storedPassword, providedPassword string) bool {
	if storedPassword == providedPassword {
		return true
	}
	if err := bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(providedPassword)); err == nil {
		return true
	}
	return false
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	if supabaseURL == "" || supabaseKey == "" {
		jsonError(w, http.StatusServiceUnavailable, "Supabase API credentials not configured")
		return
	}

	cleanUsername := strings.TrimSpace(req.Username)
	cleanPassword := strings.TrimSpace(req.Password)

	// Query user_accounts table directly via REST (case-insensitive username query)
	targetURL := fmt.Sprintf("%s/rest/v1/user_accounts?username=ilike.%s&select=*", supabaseURL, url.QueryEscape(cleanUsername))
	httpReq, err := http.NewRequest(http.MethodGet, targetURL, nil)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpReq.Header.Set("apikey", supabaseKey)
	httpReq.Header.Set("Authorization", "Bearer "+supabaseKey)

	res, err := httpClient.Do(httpReq)
	if err != nil {
		log.Printf("[ERROR] Login REST query HTTP failed: %v", err)
		jsonError(w, http.StatusInternalServerError, "Failed to connect to authentication backend")
		return
	}
	defer res.Body.Close()

	respBody, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		log.Printf("[ERROR] Login REST error (%d): %s", res.StatusCode, string(respBody))
		jsonError(w, http.StatusUnauthorized, "Invalid username or password")
		return
	}

	var users []map[string]interface{}
	if err := json.Unmarshal(respBody, &users); err != nil || len(users) == 0 {
		jsonError(w, http.StatusUnauthorized, "Invalid username or password")
		return
	}

	user := users[0]
	// Verify password (supports bcrypt hash and plaintext)
	pwd, _ := user["password_hash"].(string)
	if pwd == "" {
		pwd, _ = user["password"].(string)
	}

	if !checkPassword(pwd, cleanPassword) {
		jsonError(w, http.StatusUnauthorized, "Invalid username or password")
		return
	}

	role, _ := user["role"].(string)
	classCode, _ := user["classroom_code"].(string)
	uname, _ := user["username"].(string)
	sessionToken := fmt.Sprintf("%v", user["id"])
	if sessionToken == "" || sessionToken == "<nil>" {
		sessionToken = uname
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success":        true,
		"user":           user,
		"token":          sessionToken,
		"session_token":  sessionToken,
		"role":           role,
		"classroom_code": classCode,
		"username":       uname,
	})
}

type SignupRequest struct {
	Username      string `json:"username"`
	Password      string `json:"password"`
	Role          string `json:"role"`
	ClassroomCode string `json:"classroom_code"`
}

func handleSignup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req SignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	if supabaseURL == "" || supabaseKey == "" {
		jsonError(w, http.StatusServiceUnavailable, "Supabase API credentials not configured")
		return
	}

	// Check if username already exists
	checkURL := fmt.Sprintf("%s/rest/v1/user_accounts?username=eq.%s&select=id", supabaseURL, req.Username)
	cReq, _ := http.NewRequest(http.MethodGet, checkURL, nil)
	cReq.Header.Set("apikey", supabaseKey)
	cReq.Header.Set("Authorization", "Bearer "+supabaseKey)
	cRes, cErr := httpClient.Do(cReq)
	if cErr == nil {
		defer cRes.Body.Close()
		cBody, _ := io.ReadAll(cRes.Body)
		var existing []map[string]interface{}
		if json.Unmarshal(cBody, &existing) == nil && len(existing) > 0 {
			jsonError(w, http.StatusBadRequest, "Username already exists")
			return
		}
	}

	// Insert into user_accounts table directly via REST
	newUser := map[string]interface{}{
		"username":       req.Username,
		"password_hash":  req.Password,
		"role":           req.Role,
		"classroom_code": req.ClassroomCode,
	}

	payload, _ := json.Marshal(newUser)
	httpReq, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/rest/v1/user_accounts", supabaseURL), bytes.NewBuffer(payload))
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("apikey", supabaseKey)
	httpReq.Header.Set("Authorization", "Bearer "+supabaseKey)
	httpReq.Header.Set("Prefer", "return=representation")

	res, err := httpClient.Do(httpReq)
	if err != nil {
		log.Printf("[ERROR] Signup REST insert HTTP failed: %v", err)
		jsonError(w, http.StatusInternalServerError, "Failed to connect to authentication backend")
		return
	}
	defer res.Body.Close()

	respBody, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 400 {
		log.Printf("[ERROR] Signup REST insert error (%d): %s", res.StatusCode, string(respBody))
		jsonError(w, http.StatusBadRequest, fmt.Sprintf("Signup error: %s", string(respBody)))
		return
	}

	var created []map[string]interface{}
	_ = json.Unmarshal(respBody, &created)
	var user map[string]interface{}
	if len(created) > 0 {
		user = created[0]
		delete(user, "password_hash")
		delete(user, "password")
	} else {
		user = map[string]interface{}{
			"username":       req.Username,
			"role":           req.Role,
			"classroom_code": req.ClassroomCode,
		}
	}

	role, _ := user["role"].(string)
	classCode, _ := user["classroom_code"].(string)
	uname, _ := user["username"].(string)
	if role == "" { role = req.Role }
	if classCode == "" { classCode = req.ClassroomCode }
	if uname == "" { uname = req.Username }
	sessionToken := fmt.Sprintf("%v", user["id"])
	if sessionToken == "" || sessionToken == "<nil>" {
		sessionToken = uname
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success":        true,
		"user":           user,
		"token":          sessionToken,
		"session_token":  sessionToken,
		"role":           role,
		"classroom_code": classCode,
		"username":       uname,
	})
}

func handleDashboard(w http.ResponseWriter, r *http.Request) {
	log.Printf("[INFO] %s %s from %s", r.Method, r.URL.String(), r.RemoteAddr)
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		log.Printf("[WARN] Dashboard method not allowed: %s", r.Method)
		jsonError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	classroomCode := r.URL.Query().Get("classroom_code")
	if classroomCode == "" && r.Method == http.MethodPost {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		classroomCode = body["classroom_code"]
	}

	if classroomCode == "" {
		log.Printf("[WARN] Dashboard request missing classroom code")
		jsonError(w, http.StatusBadRequest, "Classroom code required")
		return
	}

	if supabaseURL == "" || supabaseKey == "" {
		log.Printf("[ERROR] Supabase API credentials not configured in environment")
		jsonError(w, http.StatusServiceUnavailable, "Supabase API credentials not configured")
		return
	}

	log.Printf("[INFO] Fetching dashboard natively for classroom: %s", classroomCode)

	// 1. Fetch registered student accounts for classroom
	userURL := fmt.Sprintf("%s/rest/v1/user_accounts?classroom_code=eq.%s&role=eq.student&select=username", supabaseURL, classroomCode)
	uReq, uErr := http.NewRequest(http.MethodGet, userURL, nil)
	studentMap := make(map[string][]map[string]interface{})
	alertMap := make(map[string]int)

	if uErr == nil {
		uReq.Header.Set("apikey", supabaseKey)
		uReq.Header.Set("Authorization", "Bearer "+supabaseKey)
		if uRes, err := httpClient.Do(uReq); err == nil && uRes.StatusCode == http.StatusOK {
			defer uRes.Body.Close()
			uBody, _ := io.ReadAll(uRes.Body)
			var rawUsers []map[string]interface{}
			if json.Unmarshal(uBody, &rawUsers) == nil {
				for _, u := range rawUsers {
					if uname, ok := u["username"].(string); ok && uname != "" {
						studentMap[uname] = []map[string]interface{}{}
					}
				}
			}
		}
	}

	// 2. Fetch Notebooks for classroom
	nbURL := fmt.Sprintf("%s/rest/v1/student_notebooks?classroom_code=eq.%s&select=*", supabaseURL, classroomCode)
	nbReq, err := http.NewRequest(http.MethodGet, nbURL, nil)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	nbReq.Header.Set("apikey", supabaseKey)
	nbReq.Header.Set("Authorization", "Bearer "+supabaseKey)

	nbRes, err := httpClient.Do(nbReq)
	if err != nil {
		log.Printf("[ERROR] Dashboard notebook query HTTP error: %v", err)
		jsonError(w, http.StatusInternalServerError, fmt.Sprintf("Dashboard fetch failed: %v", err))
		return
	}
	defer nbRes.Body.Close()

	nbBody, _ := io.ReadAll(nbRes.Body)
	var rawNbs []map[string]interface{}
	_ = json.Unmarshal(nbBody, &rawNbs)

	// 3. Group notebooks & count alerts by student_token
	for _, nb := range rawNbs {
		st, _ := nb["student_token"].(string)
		if st != "" {
			studentMap[st] = append(studentMap[st], nb)
			if help, ok := nb["external_help_required"].(bool); ok && help {
				alertMap[st]++
			}
		}
	}

	// 4. Fetch Review Logs (optional/best-effort)
	logURL := fmt.Sprintf("%s/rest/v1/student_review_logs?select=*", supabaseURL)
	lReq, lErr := http.NewRequest(http.MethodGet, logURL, nil)
	logMap := make(map[string][]map[string]interface{})
	if lErr == nil {
		lReq.Header.Set("apikey", supabaseKey)
		lReq.Header.Set("Authorization", "Bearer "+supabaseKey)
		if lRes, err := httpClient.Do(lReq); err == nil && lRes.StatusCode == http.StatusOK {
			defer lRes.Body.Close()
			lBody, _ := io.ReadAll(lRes.Body)
			var rawLogs []map[string]interface{}
			if json.Unmarshal(lBody, &rawLogs) == nil {
				for _, lg := range rawLogs {
					st, _ := lg["student_token"].(string)
					if st != "" {
						logMap[st] = append(logMap[st], lg)
					}
				}
			}
		}
	}

	// 5. Assemble student list payload for Vue Dashboard
	var students []map[string]interface{}
	for token, nbs := range studentMap {
		studentLogs := logMap[token]
		if studentLogs == nil {
			studentLogs = []map[string]interface{}{}
		}
		students = append(students, map[string]interface{}{
			"token":       token,
			"notebooks":   nbs,
			"logs":        studentLogs,
			"alertsCount": alertMap[token],
			"lastUpdate":  time.Now().UnixMilli(),
		})
	}

	if students == nil {
		students = []map[string]interface{}{}
	}

	log.Printf("[INFO] Dashboard fetched natively with %d students for classroom %s", len(students), classroomCode)

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"is_locked": false,
		"students":  students,
	})
}

func handleAssignments(w http.ResponseWriter, r *http.Request) {
	if supabaseURL == "" || supabaseKey == "" {
		jsonError(w, http.StatusServiceUnavailable, "Supabase API credentials not configured")
		return
	}

	switch r.Method {
	case http.MethodGet:
		classroomCode := r.URL.Query().Get("classroom_code")
		if classroomCode == "" {
			jsonError(w, http.StatusBadRequest, "Classroom code required")
			return
		}

		targetURL := fmt.Sprintf("%s/rest/v1/teacher_assignments?classroom_code=eq.%s&order=created_at.desc", supabaseURL, classroomCode)
		httpReq, err := http.NewRequest(http.MethodGet, targetURL, nil)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		httpReq.Header.Set("apikey", supabaseKey)
		httpReq.Header.Set("Authorization", "Bearer "+supabaseKey)

		res, err := httpClient.Do(httpReq)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer res.Body.Close()

		respBody, _ := io.ReadAll(res.Body)
		var list interface{}
		_ = json.Unmarshal(respBody, &list)
		if list == nil {
			list = []interface{}{}
		}
		jsonResponse(w, http.StatusOK, list)

	case http.MethodPost:
		var req struct {
			ID            string `json:"id"`
			ClassroomCode string `json:"classroom_code"`
			Title         string `json:"title"`
			DownloadURL   string `json:"download_url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, http.StatusBadRequest, "Invalid JSON payload")
			return
		}

		payload, _ := json.Marshal(map[string]interface{}{
			"id":             req.ID,
			"classroom_code": req.ClassroomCode,
			"title":          req.Title,
			"download_url":   req.DownloadURL,
		})

		httpReq, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/rest/v1/teacher_assignments", supabaseURL), bytes.NewBuffer(payload))
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("apikey", supabaseKey)
		httpReq.Header.Set("Authorization", "Bearer "+supabaseKey)

		res, err := httpClient.Do(httpReq)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer res.Body.Close()

		if res.StatusCode >= 400 {
			respBody, _ := io.ReadAll(res.Body)
			jsonError(w, res.StatusCode, string(respBody))
			return
		}
		jsonResponse(w, http.StatusCreated, map[string]bool{"success": true})

	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			jsonError(w, http.StatusBadRequest, "ID parameter required")
			return
		}

		targetURL := fmt.Sprintf("%s/rest/v1/teacher_assignments?id=eq.%s", supabaseURL, id)
		httpReq, err := http.NewRequest(http.MethodDelete, targetURL, nil)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		httpReq.Header.Set("apikey", supabaseKey)
		httpReq.Header.Set("Authorization", "Bearer "+supabaseKey)

		res, err := httpClient.Do(httpReq)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer res.Body.Close()

		jsonResponse(w, http.StatusOK, map[string]bool{"success": true})

	default:
		jsonError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func handleSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Sync endpoint ready",
	})
}
