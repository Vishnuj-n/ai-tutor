# Automatic Classroom Study Profile Isolation & Lifecycle

**Date:** 2026-08-07  
**Module:** Backend Settings & Auth (`internal/app/app_settings.go`), Frontend Auth Composable (`frontend/src/composables/useAuth.js`)

---

## Problem Overview

Previously, when a student signed in or registered with a classroom code (e.g. `BCD601` with username `testing`), the desktop app attached the resulting Supabase session credentials (`ClassroomCode`, `StudentUsername`, `CloudAPIToken`) to whichever local profile happened to be active (e.g. `TESTING`). Logging into a new classroom would overwrite the existing local profile's credentials instead of isolating each classroom under its own dedicated Study Profile.

---

## Key Solutions Implemented

### 1. Automatic Classroom Profile Creation & Matching (`internal/app/app_settings.go`)

In `LoginStudent` (which is also called on successful registration via `SignUpStudent`):
- All local profiles are fetched via `repo.GetProfiles()`.
- The backend checks for an existing profile matching `profile.ClassroomCode == loginResp.ClassroomCode`.
- **Existing Profile Match**: Updates cloud credentials (`classroom_code`, `student_username`, `cloud_api_token`) on the matching profile and activates it via `settings.ActiveProfileID`.
- **New Classroom**: If no profile exists for that classroom code, a new `models.StudyProfile` is automatically created:
  - `Name`: `loginResp.ClassroomCode` (e.g. `BCD601`)
  - `DeadlineAt`: 90 days from creation (`time.Now().AddDate(0, 3, 0).Unix()`)
  - `ClassroomCode`, `StudentUsername`, `CloudAPIToken`: populated from auth response.
  - Persisted to SQLite via `repo.CreateProfile` and set as `ActiveProfileID`.

### 2. Profile Lifecycle & Data Protection

- **Existing Profiles Preserved**: Unrelated local profiles (e.g. `TESTING`) are **never deleted or overwritten** when joining a new class.
- **Deactivation on New Class Login**: When logging into a new classroom, previously active profiles remain saved in SQLite and transition to inactive status under **Settings > Study Profiles**.
- **Switching & Re-Activation**: Students can switch back to any study profile anytime via the profile picker. Logging back into a previously joined classroom re-matches and re-activates its existing profile without creating duplicates.

### 3. Automated Test Coverage (`internal/app/app_test.go`)

- Added `TestLoginStudent_AutomaticProfileCreation` to test:
  1. Login to a new classroom code automatically generates and activates a dedicated Study Profile.
  2. Repeated login to the same classroom code reuses the existing classroom profile without duplicating rows.

---

## Verification

- `go test ./internal/app/...` passed cleanly.
- `go test ./...` passed across all packages.
