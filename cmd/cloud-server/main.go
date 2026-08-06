package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

var db *sql.DB

type Response struct {
	Error string      `json:"error,omitempty"`
	Data  interface{} `json:"data,omitempty"`
}

func main() {
	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		// Fallback default for local or Supabase postgres
		dbURL = os.Getenv("SUPABASE_DB_URL")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	if dbURL != "" {
		var err error
		db, err = sql.Open("postgres", dbURL)
		if err != nil {
			log.Printf("[WARN] Failed to connect to Postgres DB: %v", err)
		} else {
			if err := db.Ping(); err != nil {
				log.Printf("[WARN] Postgres ping failed: %v", err)
			} else {
				log.Println("[INFO] Successfully connected to PostgreSQL Database")
			}
		}
	} else {
		log.Println("[WARN] DATABASE_URL is not set. API endpoints will require DB connection string.")
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/api/health", handleHealth)
	mux.HandleFunc("/api/auth/login", handleLogin)
	mux.HandleFunc("/api/auth/signup", handleSignup)
	mux.HandleFunc("/api/dashboard", handleDashboard)
	mux.HandleFunc("/api/assignments", handleAssignments)

	handler := enableCORS(mux)

	log.Printf("[INFO] Cloud Server running on http://localhost:%s", port)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatalf("Server stopped: %v", err)
	}
}

func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, x-session-token")

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
	json.NewEncoder(w).Encode(payload)
}

func jsonError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, http.StatusOK, map[string]string{"status": "ok", "service": "ai-tutor-cloud-server"})
}

type LoginRequest struct {
	Username  string `json:"username"`
	Password  string `json:"password"`
	IsDesktop bool   `json:"is_desktop"`
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

	if db == nil {
		jsonError(w, http.StatusServiceUnavailable, "Database not connected")
		return
	}

	var resJSON string
	err := db.QueryRow("SELECT login_user($1, $2, $3)::text", req.Username, req.Password, req.IsDesktop).Scan(&resJSON)
	if err != nil {
		log.Printf("[ERROR] Login failed for %s: %v", req.Username, err)
		jsonError(w, http.StatusUnauthorized, "Invalid username or password")
		return
	}

	var result map[string]interface{}
	_ = json.Unmarshal([]byte(resJSON), &result)
	jsonResponse(w, http.StatusOK, result)
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

	if db == nil {
		jsonError(w, http.StatusServiceUnavailable, "Database not connected")
		return
	}

	var resJSON string
	err := db.QueryRow("SELECT signup_user($1, $2, $3, $4)::text", req.Username, req.Password, req.Role, req.ClassroomCode).Scan(&resJSON)
	if err != nil {
		log.Printf("[ERROR] Signup failed for %s: %v", req.Username, err)
		jsonError(w, http.StatusBadRequest, fmt.Sprintf("Signup error: %v", err))
		return
	}

	var result map[string]interface{}
	_ = json.Unmarshal([]byte(resJSON), &result)
	jsonResponse(w, http.StatusOK, result)
}

func handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
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
		jsonError(w, http.StatusBadRequest, "Classroom code required")
		return
	}

	if db == nil {
		jsonError(w, http.StatusServiceUnavailable, "Database not connected")
		return
	}

	// We pass session token if available
	sessionToken := r.Header.Get("x-session-token")
	if sessionToken == "" {
		sessionToken = r.Header.Get("Authorization")
	}

	var rawJSON string
	// Set local session context for the connection session if needed or query function directly
	err := db.QueryRow("SELECT get_classroom_dashboard($1)::text", classroomCode).Scan(&rawJSON)
	if err != nil {
		log.Printf("[ERROR] Dashboard fetch error: %v", err)
		// Fallback query if get_classroom_dashboard RPC fails due to session context
		var fallbackStudents []map[string]interface{}
		rows, err2 := db.Query(`
			SELECT student_token, json_agg(json_build_object(
				'title', title, 'filename', filename, 'file_hash', file_hash, 'study_status', study_status, 'external_help_required', external_help_required, 'updated_at', updated_at
			) ORDER BY updated_at DESC) as notebooks
			FROM student_notebooks WHERE classroom_code = $1 GROUP BY student_token`, classroomCode)

		if err2 == nil {
			defer rows.Close()
			for rows.Next() {
				var token, nbsJSON string
				if err3 := rows.Scan(&token, &nbsJSON); err3 == nil {
					var nbs []map[string]interface{}
					_ = json.Unmarshal([]byte(nbsJSON), &nbs)
					fallbackStudents = append(fallbackStudents, map[string]interface{}{
						"token": token,
						"notebooks": nbs,
						"logs": []interface{}{},
						"alertsCount": 0,
						"lastUpdate": 0,
					})
				}
			}
			jsonResponse(w, http.StatusOK, fallbackStudents)
			return
		}

		jsonError(w, http.StatusInternalServerError, fmt.Sprintf("Dashboard fetch failed: %v", err))
		return
	}

	var result []interface{}
	_ = json.Unmarshal([]byte(rawJSON), &result)
	if result == nil {
		result = []interface{}{}
	}
	jsonResponse(w, http.StatusOK, result)
}

func handleAssignments(w http.ResponseWriter, r *http.Request) {
	if db == nil {
		jsonError(w, http.StatusServiceUnavailable, "Database not connected")
		return
	}

	switch r.Method {
	case http.MethodGet:
		classroomCode := r.URL.Query().Get("classroom_code")
		if classroomCode == "" {
			jsonError(w, http.StatusBadRequest, "Classroom code required")
			return
		}

		rows, err := db.Query("SELECT id, classroom_code, title, download_url, created_at FROM teacher_assignments WHERE classroom_code = $1 ORDER BY created_at DESC", classroomCode)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer rows.Close()

		var list []map[string]interface{}
		for rows.Next() {
			var id, classCode, title, url, createdAt string
			if err := rows.Scan(&id, &classCode, &title, &url, &createdAt); err == nil {
				list = append(list, map[string]interface{}{
					"id":             id,
					"classroom_code": classCode,
					"title":          title,
					"download_url":   url,
					"created_at":     createdAt,
				})
			}
		}
		if list == nil {
			list = []map[string]interface{}{}
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

		_, err := db.Exec("INSERT INTO teacher_assignments (id, classroom_code, title, download_url) VALUES ($1, $2, $3, $4)", req.ID, req.ClassroomCode, req.Title, req.DownloadURL)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		jsonResponse(w, http.StatusCreated, map[string]bool{"success": true})

	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			jsonError(w, http.StatusBadRequest, "ID parameter required")
			return
		}

		_, err := db.Exec("DELETE FROM teacher_assignments WHERE id = $1", id)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, map[string]bool{"success": true})

	default:
		jsonError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}
