# Developer Onboarding & Cloud Dashboard Handover Guide

Welcome to the **AI Tutor** codebase! This guide serves as a comprehensive starting point for new developers to set up the local environment, run the app for development, understand the underlying SQLite study queue architecture, and successfully inherit/manage the Supabase cloud dashboard.

---

## 1. Project Overview

AI Tutor is a local-first desktop application designed to guide learners through a structured study loop (Reading → Comprehension Quiz → Spaced Repetition Review). 

### Key Technical Stack:
* **Desktop Shell & Backend:** Go 1.26 + Wails v2
* **Local Data & Embeddings:** SQLite + `sqlite-vec` (via `vec0` extension)
* **Frontend Apps:**
  * **Desktop Frontend:** Vue 3 (contained in `frontend/`)
  * **Teacher Cloud Dashboard:** Vue 3 + Vite (contained in `cloud-dashboard/`)
* **Spaced Repetition:** `go-fsrs/v4`
* **Local RAG Pipeline:** ONNX Runtime (`onnxruntime`) + Local INT8 Sentence Transformers model

---

## 2. Desktop App: Setup & Local Development

Follow these steps to run the core desktop client locally.

### Prerequisites
Make sure you have the following installed on your machine:
* **Go 1.26+**
* **Node.js 20+**
* **Wails CLI** (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)
* **CGO-capable compiler toolchain:**
  * **Windows:** MSVC Build Tools or MinGW-w64 (required to compile CGO extensions)
  * **macOS:** Xcode Command Line Tools
  * **Linux:** `build-essential`

### Step 1: Initialize Local RAG Assets
Local RAG requires runtime dependencies (DLL/dylib/so files) and the embedding model. Run the dependency sync script to download them:

* **Windows (PowerShell):**
  ```powershell
  ./windows-sync-deps.ps1
  ```
* **macOS / Linux (Shell):**
  ```bash
  ./sync-deps.sh
  ```

This script will verify your platform and download the required binary libraries (`vec0`, `onnxruntime`) and model assets into the [asset/](./asset) folder.

### Step 2: System Health Check
Run the Wails diagnostic command to ensure your developer environment is ready:
```bash
wails doctor
```

### Step 3: Run Development Server
Start Wails in development mode with the `sqlite_extension` build tag (needed for `sqlite-vec` support):
```bash
wails dev -tags sqlite_extension
```
This runs the Go backend, starts the hot-reloading Vue frontend, and launches the desktop app window.

---

## 3. SQLite Schema & Task Queue

All learning progression in the application is managed via a **deterministic queue**, backed entirely by SQLite. There are no background schedulers or hidden in-memory state machines.

### Invariants:
1. **SQLite is the Source of Truth:** Every state change is a direct database transaction.
2. **Deterministic Priority Ordering:**
   1. `FLASHCARD_GENERATE` (Cloud sync recovery)
   2. `SOCRATIC_REMEDIAL` (Remediation lane for consecutive failures)
   3. `FLASHCARD_REVIEW` (Due cards via FSRS)
   4. `REREAD` (Remediation study tasks)
   5. `QUIZ` (Short-term comprehension check)
   6. `READING` (New conceptual material)

Review the full schema details in [doc/SCHEMA.md](./doc/SCHEMA.md) and module mapping in [doc/AGENT_MAP.md](./doc/AGENT_MAP.md).

---

## 4. Cloud Sync Architecture

When students are online, the client syncs study progress and spaced repetition logs to the cloud.

* **Database Engine:** Supabase (PostgreSQL)
* **API Paradigm:** "Backend-less" direct RPC & REST querying (no intermediate application server needed)
* **Client Code:** Sync logic resides in `internal/study/sync.go`
* **Sync Payload:** POST payload containing:
  * `user_token` (UUID session token generated upon login)
  * `classroom_code`
  * `notebooks` (Array of metadata: filename, title, hash, and status)
  * `logs` (Array of space-repetition review logs since `last_synced_at`)

---

## 5. Cloud Dashboard Handover Guide

As a developer inheriting the cloud infrastructure, you will manage both the **Supabase Backend** and the **Teacher Web Dashboard**.

### Part A: Setting Up a New Supabase Instance
To hand over or provision a new backend:
1. Create a new project in the [Supabase Console](https://supabase.com).
2. Go to the **SQL Editor** in your Supabase project dashboard.
3. Open and copy the entire contents of [supabase_schema.sql](./supabase_schema.sql) in this repository.
4. Paste and execute it. This script does the following:
   * Enables the UUID extension.
   * Creates the core tables: `student_notebooks`, `student_review_logs`, `teacher_assignments`, `user_accounts`, and `active_sessions`.
   * Sets up the custom `login_user` and `signup_user` RPC functions.
   * Sets up the `handle_cloud_sync` RPC function (which handles transactional delta upserts).
   * Enforces Row Level Security (RLS) policies requiring validation of the custom `x-session-token` header.

#### Creating Initial User Accounts
Since the system is secure, you can register new accounts using the `signup_user` RPC function via the SQL editor:
```sql
-- Register a teacher account
SELECT signup_user('teacher@school.edu', 'securepassword123', 'teacher', 'BIO101');

-- Register a student account
SELECT signup_user('student1', 'studentpassword123', 'student', 'BIO101');
```

---

### Part B: Deploying the Teacher Web Dashboard
The dashboard allows teachers to view student statistics, recall pass rates, active alerts (such as students stuck in Socratic rescue blocks), and assign textbook PDFs.

#### Directory Structure
All dashboard code is located in the [cloud-dashboard/](./cloud-dashboard) directory:
* [cloud-dashboard/src/App.vue](./cloud-dashboard/src/App.vue) — Core Dashboard logic and template
* [cloud-dashboard/src/style.css](./cloud-dashboard/src/style.css) — Glassmorphism and indigo dark theme stylings

#### Local Setup
1. Navigate to the cloud-dashboard directory:
   ```bash
   cd cloud-dashboard
   ```
2. Install npm dependencies:
   ```bash
   npm install
   ```
3. Configure the environment variables in `cloud-dashboard/.env` (use `.env.example` if available as a template):
   ```env
   VITE_SUPABASE_URL=https://<your-supabase-project-ref>.supabase.co
   VITE_SUPABASE_ANON_KEY=<your-supabase-anon-key>
   VITE_CLASSROOM_CODE=BIO101
   ```
4. Run the development server:
   ```bash
   npm run dev
   ```
   Open `http://localhost:5173` to view the running dashboard.

#### Production Build & Deployment
To bundle the frontend for production hosting (e.g., Vercel, Netlify, Github Pages, or Supabase Hosting):
```bash
npm run build
```
This outputs compiled, static HTML/JS/CSS assets to the `dist/` directory. Deploy the contents of the `dist/` folder to any static hosting provider.

---

### Part C: Connecting the Desktop App to the New Backend
When you hand over the instance:
1. Provide the new developer with the **Supabase URL** and **Anon Key**.
2. To point the desktop app to the new backend, students enter their credentials in the **Settings** -> **Account & Cloud** setup wizard.
3. For local testing with environment variables, set them prior to starting `wails dev`:
   ```powershell
   # PowerShell
   $env:CLOUD_SYNC_URL = "https://<your-supabase-project-ref>.supabase.co/rest/v1/rpc/handle_cloud_sync"
   $env:CLOUD_API_TOKEN = "<your-supabase-anon-key>"
   wails dev -tags sqlite_extension
   ```

---

## 6. Common Verification Commands

Use these commands during onboarding to verify everything works:

* **Go Tests:** `go test ./...`
* **Mock Server Verification:** See [doc/CLOUD_SYNC_TESTING.md](./doc/CLOUD_SYNC_TESTING.md) for mock server setup (Method 1: embedded Node.js server) to test delta logs without hitting Supabase.
* **Database Inspection:** Run `sqlite3` on the client profile `.db` file in the user's local directory to manually query state if tasks become locked.
