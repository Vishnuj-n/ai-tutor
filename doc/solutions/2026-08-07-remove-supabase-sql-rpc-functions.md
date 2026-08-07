# Solution: Removal of Supabase SQL RPC Functions in favor of Go Backend Logic

**Date:** 2026-08-07  
**Module:** `cmd/cloud-server`

## Overview

Previously, authentication (`login_user`, `signup_user`) and dashboard data aggregation (`get_classroom_dashboard`) relied on PostgreSQL stored procedures (PL/pgSQL functions) in Supabase called via `/rest/v1/rpc/*`.

This created schema migration friction, silent dashboard failures when table schemas changed, and violated Rule #5 of `AGENTS.md` ("Frontend does not own business logic — UI is thin; all decisions happen in Go backend").

## Changes Made

Refactored `cmd/cloud-server/main.go` to handle authentication and dashboard aggregation natively via direct REST table operations (`user_accounts`, `student_notebooks`, `student_review_logs`):

1. **`handleLogin`**: Performs a `GET` request on `/rest/v1/user_accounts?username=eq.<username>`, verifies password hash in Go, and strips sensitive fields from response.
2. **`handleSignup`**: Performs a duplicate check `GET` on `/rest/v1/user_accounts` and inserts via `POST` `/rest/v1/user_accounts`.
3. **`handleDashboard`**: Queries `student_notebooks` and `student_review_logs` via standard REST endpoints and aggregates student progress into JSON format for Vue frontend.

---

## SQL Statements to Cleanup Legacy Supabase Functions

Run the following SQL in the **Supabase SQL Editor** to drop the legacy RPC stored functions:

```sql
-- Drop legacy stored procedure RPCs
DROP FUNCTION IF EXISTS public.login_user(text, text, boolean);
DROP FUNCTION IF EXISTS public.login_user(text, text);
DROP FUNCTION IF EXISTS public.signup_user(text, text, text, text);
DROP FUNCTION IF EXISTS public.get_classroom_dashboard(text);
```
