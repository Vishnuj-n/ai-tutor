All done. All go build ./... and go test ./... pass.
Summary of changes:
File	What changed
internal/study/service.go:28	Added GetLimits() llmpkg.ModelLimits to LLMProvider interface
internal/study/service.go:508-511	Deleted DefaultMaxInputTokens / DefaultMaxOutputTokens constants
internal/study/flashcard.go:124-132	Removed type-assertion dance, calls llm.GetLimits() directly
internal/study/quiz_sync.go:148-158	Same — direct call instead of type-assertion
internal/study/socratic.go:193-199	Same — removed interface{ GetLimits()...} type assertion
app.go:28-32	Added GetLimits() llm.ModelLimits to llmProviderInterface
app_contract_test.go:845-848	Added GetLimits() to mockLLMProvider
The token limit contract is now explicit in the interface — no more silent fallback constants, no more type-assertion override pattern, no more drift risk.