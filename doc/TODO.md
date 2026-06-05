# LIST  
- [ ] fix app.go 

# REASON
- [1] 
1. `app.go` is intended to be a thin Wails/UI bridge and does this well for AI features. 
2. `AskAI`, `AskReaderAI`, `AskSocratic`, and `ExplainReaderSection` correctly delegate to services. 
3. `startup()` is fine—it acts as the dependency wiring/composition root. 
4. `GetTodayPlan()` duplicates scheduling/business logic already present in `scheduler.BuildTodayPlan()`.  
5. `InitializeReadingSession()` contains substantial reading-session workflow logic. 
6. `CompleteReading()` contains quiz-generation workflow logic. 
7. `queueTaskToScheduledTask()` contains business-rule transformations. 
8. These workflows would fit better inside dedicated services.
9. The main issue is responsibility leakage, not file size.
10. Overall: ~70% orchestrator, ~20% startup wiring, ~10% business logic that should be moved into services. 
