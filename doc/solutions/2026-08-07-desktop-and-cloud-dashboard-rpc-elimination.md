# Solution: Desktop & Teacher Portal Legacy Supabase RPC Elimination

**Date:** 2026-08-07  
**Module:** `cmd/cloud-server`, `cloud-dashboard`, `internal/app`, `internal/study`

## Problem

After stored procedure RPC functions (`login_user`, `signup_user`, `get_classroom_dashboard`, `toggle_classroom_lock`, `remove_student_from_classroom`) were dropped from PostgreSQL/Supabase, the Teacher Portal (`cloud-dashboard`) and desktop Wails backend (`internal/app/app_settings.go`) failed with:
`Could not find the function public.login_user(p_is_desktop, p_password, p_username) in the schema cache`.

## Architecture & Resolution

### 1. Zero SQL Stored Procedures
All authentication and dashboard calls now bypass Supabase PL/pgSQL stored procedures completely:
- **Primary Route**: Requests hit `cmd/cloud-server` Go REST handlers (`/api/auth/login`, `/api/auth/signup`, `/api/dashboard`).
- **Fallback Route**: If the Go Cloud Server is not running, both the desktop app and Teacher Portal execute standard direct HTTPS queries against Supabase's built-in REST table endpoints (`/rest/v1/user_accounts`, `/rest/v1/student_notebooks`, `/rest/v1/student_review_logs`).

### 2. URL Resolution & Single Production Constant
To ensure smooth deployment without code scatter or requiring students to manage `.env` files for compiled `.exe` desktop builds:

1. **Local Development**:
   Uses `.env` overrides:
   - `CLOUD_SERVER_URL=http://localhost:8080` (or `VITE_API_URL`)
   - `SUPABASE_URL` / `VITE_SUPABASE_URL`
   - `SUPABASE_ANON_KEY` / `VITE_SUPABASE_ANON_KEY`

2. **Production Desktop `.exe` Build**:
   Controlled by a single central Go constant in `internal/study/sync.go`:
   ```go
   const DefaultProductionCloudServerURL = "http://localhost:8080"
   ```
   *When deploying to production (e.g. Render, Fly.io, Railway), update only this single line to `https://your-cloud-server.onrender.com`.*

3. **Resolution Order**:
   - `LoginStudent` / `SignUpStudent`: `CLOUD_SERVER_URL` or `VITE_API_URL` env var → `DefaultProductionCloudServerURL` → Direct Supabase REST table query fallback.

---

## Files Modified

- [`internal/study/sync.go`](file:///c:/Users/vishn/PROJECT/ai-tutor/internal/study/sync.go): Added `DefaultProductionCloudServerURL` and `ResolveCloudServerURL()`.
- [`cmd/cloud-server/main.go`](file:///c:/Users/vishn/PROJECT/ai-tutor/cmd/cloud-server/main.go): Updated `handleLogin` and `handleSignup` to output top-level JSON fields (`session_token`, `role`, `classroom_code`, `username`).
- [`internal/app/app_settings.go`](file:///c:/Users/vishn/PROJECT/ai-tutor/internal/app/app_settings.go): Refactored `LoginStudent` and `SignUpStudent` to use `ResolveCloudServerURL()` with fallback to direct `user_accounts` table queries and mock HTTP server support.
- [`cloud-dashboard/src/composables/useDashboard.js`](file:///c:/Users/vishn/PROJECT/ai-tutor/cloud-dashboard/src/composables/useDashboard.js): Replaced RPC calls with `/api/auth/*` and direct Supabase REST table operations.

---

## Verification

- Running `go test ./...` passed (100% PASS).
