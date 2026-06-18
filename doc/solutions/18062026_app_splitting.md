. app.go — Removed all duplicated settings methods (GetDailyStudySettings through TriggerCloudSync, ~400 lines) and trimmed imports (encoding/json, os, time, database/sql, models, uuid). Kept llm (needed by llmProviderInterface).
2. app_settings.go — Added missing package main, import block (os, strings, time, llm, models, study, utils, uuid), and study import.
3. app_study.go — Trimmed unused imports (context, os, sync, embeddings, llm, notebook, retrieval, runtime, wailsruntime, study).
File organization now:
- app.go — App struct, NewApp, startup/shutdown, API methods still in app.go (Greet, GetReaderTopicBundle, GetAvailableTopics, AskSocratic, AskReaderAI, GetEmbeddingDiagnostics, InitializeRAG, reloadRetrievalEngine, emitRagSetupFailed)
- app_settings.go — All settings/profile/onboarding/cloud-sync methods
- app_study.go — All study/task/queue/quiz/flashcard/review methodsP