# Walkthrough — Keyring-backed LLM Provider Setup

All changes have been successfully implemented across the codebase in a single cohesive phase. Below is a detailed summary of the architectural and UI enhancements.

## Changes Made

### 1. LLM configuration model and persistence (`internal/models/models.go`, `internal/db/schema.go`, `internal/db/store.go`)
- Added dedicated `LLMSettings` and `LLMTierSettings` models so the app can store fast and heavy provider config separately without duplicating provider code.
- Added a new `llm_settings` SQLite table seeded with sane defaults for `fast` and `heavy` tiers.
- Added repository helpers to read, write, normalize, and compare LLM settings deterministically.

### 2. Secret storage integration (`internal/llm/keyring.go`, `go.mod`, `go.sum`)
- Added `github.com/zalando/go-keyring` as the secret store for API keys.
- Implemented tier-based key helpers for save, read, delete, and test initialization.
- Kept API keys out of SQLite; only non-sensitive provider metadata remains in the database.

### 3. LLM provider resolution and boot wiring (`internal/llm/provider.go`, `internal/runtime/boot.go`)
- Added a settings-based config loader that resolves values from environment variables first, then SQLite settings, then provider defaults.
- Updated provider startup so both fast and heavy tiers can be built from the persisted config plus keyring secrets.
- Preserved the existing fast/heavy split already used by the study service, with heavy falling back to fast credentials when the user chooses one shared setup.

### 4. Wails backend endpoints for LLM settings (`app.go`)
- Added endpoints to fetch and update LLM settings from the frontend.
- Added endpoints to save and delete API keys through the OS credential store.
- Reloaded backend provider instances after settings changes so the app can apply edits without a restart.

### 5. Onboarding flow (`frontend/src/pages/Onboarding.vue`, `frontend/src/services/appApi.js`)
- Inserted a new onboarding step for provider selection before cloud sync and RAG setup.
- Added provider presets for Groq, ChatGPT / OpenAI, OpenRouter, and custom OpenAI-compatible endpoints.
- Added a “use same provider and model for heavy AI tasks” toggle to avoid duplicate configuration unless the user wants it.
- Routed key storage through the backend and kept the UI limited to configuration and status.

### 6. Settings screen (`frontend/src/pages/Settings.vue`, `frontend/src/services/appApi.js`)
- Added a persistent LLM settings section for editing provider, base URL, model, and API keys after onboarding.
- Exposed stored-key status for fast and heavy tiers without revealing secret values.
- Added key removal support that clears the OS credential store entry and updates the saved metadata.

## Verification Plan

### Automated Tests
- `go test ./...` passed after the backend changes, including the new keyring and LLM config tests.
- `npm run build` in `frontend/` passed after the onboarding and settings UI changes.

### Manual Smoke Testing
1. Open onboarding and verify the new AI provider step appears before cloud sync and RAG setup.
2. Choose a preset provider, enter an API key, complete onboarding, and confirm the app reaches the dashboard.
3. Open Settings, confirm the LLM provider section loads, update the model/base URL, and verify the change persists after reload.
