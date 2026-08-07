# Solution: Supabase REST/RPC Migration for Go Cloud Server

**Date:** 2026-08-07  
**Module:** `cmd/cloud-server`

---

## Problem Overview

Previously, the Go Cloud Server (`cmd/cloud-server/main.go`) connected directly to the Supabase PostgreSQL database instance over port 5432 using `database/sql`, `github.com/lib/pq`, and a raw PostgreSQL connection string (`DATABASE_URL=postgres://postgres:[PASSWORD]@...`).

This created several operational and security drawbacks:
1. **Password Exposure Risk**: Required raw database superuser/admin credentials in environment variables (`DATABASE_URL`).
2. **Connection Pooling & Firewall Friction**: Direct TCP database connections (port 5432) can easily exhaust connection pools or get blocked by restricted firewall/serverless environments.
3. **Architectural Mismatch**: The Vue 3 Teacher Dashboard and Desktop App sync modules communicate with Supabase via HTTP REST/RPC using **API Keys** (`SUPABASE_URL` and `SUPABASE_PUBLISHABLE_KEY`), making direct DB connections in the Go proxy redundant.

---

## Solution

Refactored `cmd/cloud-server/main.go` to operate entirely over **Supabase REST and RPC HTTP Endpoints** using standard HTTP API keys (`SUPABASE_URL` and `SUPABASE_PUBLISHABLE_KEY` / `SUPABASE_SERVICE_ROLE_KEY`).

### Key Changes:

1. **Dependency Cleanup**:
   - Removed `database/sql` and `github.com/lib/pq` imports and database connection lifecycle code (`var db *sql.DB`).
   - Replaced direct SQL execution with stateless HTTP REST calls using Go's native `net/http` client.

2. **Endpoint Refactoring**:
   - **`handleLogin`**: Executes HTTP POST to `${SUPABASE_URL}/rest/v1/rpc/login_user` with parameters (`p_username`, `p_password`, `p_is_desktop`).
   - **`handleSignup`**: Executes HTTP POST to `${SUPABASE_URL}/rest/v1/rpc/signup_user` with parameters (`p_username`, `p_password`, `p_role`, `p_classroom_code`).
   - **`handleDashboard`**: Executes HTTP POST to `${SUPABASE_URL}/rest/v1/rpc/get_classroom_dashboard` with `p_classroom_code` and passes the user's `x-session-token` header when present.
   - **`handleAssignments`**:
     - `GET`: Queries `${SUPABASE_URL}/rest/v1/teacher_assignments?classroom_code=eq.<CODE>&order=created_at.desc`.
     - `POST`: Inserts via POST to `${SUPABASE_URL}/rest/v1/teacher_assignments`.
     - `DELETE`: Removes via DELETE to `${SUPABASE_URL}/rest/v1/teacher_assignments?id=eq.<ID>`.

3. **Environment & Configuration**:
   - Removed `DATABASE_URL` requirement from `.env`.
   - Boot checks now verify `SUPABASE_URL` and `SUPABASE_PUBLISHABLE_KEY` (or `SUPABASE_SERVICE_ROLE_KEY`).

---

## Verification & Impact

- **Build Verification**: Executed `go build -o bin/cloud-server.exe ./cmd/cloud-server` — compiled cleanly with zero errors.
- **Suite Verification**: Executed `go test ./...` — all test packages passed.
- **Developer Onboarding Simplicity**: Developers no longer need to obtain raw Postgres database passwords to run `go run ./cmd/cloud-server`.
