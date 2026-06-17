package utils

import (
	"log"
	"os"
	"strings"
	"sync"
)

const (
	levelDebug = iota
	levelInfo
	levelWarn
	levelError
	levelOff
)

var (
	configuredLevel int
	levelOnce       sync.Once
)

func currentLogLevel() int {
	levelOnce.Do(func() {
		configuredLevel = parseLogLevel()
	})
	return configuredLevel
}

func parseLogLevel() int {
	raw := strings.TrimSpace(os.Getenv("AI_TUTOR_LOG_LEVEL"))
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv("LOG_LEVEL"))
	}
	if raw == "" {
		// Default to warnings and errors to keep startup logs concise.
		return levelWarn
	}

	switch strings.ToLower(raw) {
	case "debug":
		return levelDebug
	case "info":
		return levelInfo
	case "warn", "warning":
		return levelWarn
	case "error":
		return levelError
	case "off", "none", "silent":
		return levelOff
	default:
		return levelWarn
	}
}


func Debugf(format string, args ...interface{}) {
	if currentLogLevel() <= levelDebug {
		log.Printf("DEBUG: "+format, args...)
	}
}

func Infof(format string, args ...interface{}) {
	if currentLogLevel() <= levelInfo {
		log.Printf("INFO: "+format, args...)
	}
}

func Warnf(format string, args ...interface{}) {
	if currentLogLevel() <= levelWarn {
		log.Printf("WARN: "+format, args...)
	}
}

func Errorf(format string, args ...interface{}) {
	if currentLogLevel() <= levelError {
		log.Printf("ERROR: "+format, args...)
	}
}

// ---------- Queue Lifecycle Logging ----------

// LogQueueTransition logs structured queue transition events.
// Format: [QUEUE] task=<id> type=<type> from=<old> to=<new> reason=<reason>
func LogQueueTransition(taskID, taskType, oldStatus, newStatus, reason string) {
	if currentLogLevel() > levelWarn {
		return
	}
	if taskID == "" {
		taskID = "unknown"
	}
	if taskType == "" {
		taskType = "unknown"
	}
	if oldStatus == "" {
		oldStatus = "none"
	}
	if newStatus == "" {
		newStatus = "unknown"
	}
	if reason == "" {
		reason = "transition"
	}
	log.Printf("[QUEUE] task=%s type=%s from=%s to=%s reason=%s", taskID, taskType, oldStatus, newStatus, reason)
}

// LogQueueTaskCreated logs when a new task is inserted into the queue.
func LogQueueTaskCreated(taskID, taskType, notebookID, topicID string) {
	if currentLogLevel() > levelWarn {
		return
	}
	if taskID == "" {
		taskID = "unknown"
	}
	if taskType == "" {
		taskType = "unknown"
	}
	log.Printf("[QUEUE] task=%s type=%s notebook=%s topic=%s event=task_inserted", taskID, taskType, notebookID, topicID)
}

// LogQuizResult logs quiz completion with pass/fail outcome.
func LogQuizResult(taskID string, score int, passed bool, rereadTaskID string) {
	if currentLogLevel() > levelWarn {
		return
	}
	if taskID == "" {
		taskID = "unknown"
	}
	outcome := "failed"
	if passed {
		outcome = "passed"
	}
	rereadInfo := "reread=none"
	if rereadTaskID != "" {
		rereadInfo = "reread=" + rereadTaskID
	}
	log.Printf("[QUEUE] task=%s type=QUIZ score=%d outcome=%s %s event=quiz_completed", taskID, score, outcome, rereadInfo)
}

// LogRereadInsertion logs when a reread task is generated after quiz failure.
func LogRereadInsertion(taskID, topicID, attemptCount, maxAttempts string) {
	if currentLogLevel() > levelWarn {
		return
	}
	if taskID == "" {
		taskID = "unknown"
	}
	log.Printf("[QUEUE] task=%s type=REREAD topic=%s attempt=%s/%s event=reread_inserted", taskID, topicID, attemptCount, maxAttempts)
}

// LogReviewSession logs review session lifecycle events.
func LogReviewSession(taskID, notebookID, cardCount, event string) {
	if currentLogLevel() > levelWarn {
		return
	}
	if taskID == "" {
		taskID = "unknown"
	}
	log.Printf("[QUEUE] task=%s type=FLASHCARD_REVIEW notebook=%s cards=%s event=%s", taskID, notebookID, cardCount, event)
}


// LogSchedulerDecision logs adaptive reading window decisions.
func LogSchedulerDecision(topicID string, startPage, endPage int, tokenBudget, reason string) {
	if currentLogLevel() > levelDebug {
		return
	}
	if topicID == "" {
		topicID = "unknown"
	}
	log.Printf("[SCHEDULER] topic=%s window=%d-%d tokenBudget=%s reason=%s", topicID, startPage, endPage, tokenBudget, reason)
}

// ---------- Boot / Init Logging ----------

// LogBoot logs a named RAG/boot initialization step with its outcome.
// stage is a short name like "rag-assets", "vec-extension", "onnx-embedder".
// outcome is "ok", "skipped", or "failed".
func LogBoot(stage, outcome, detail string) {
	if currentLogLevel() > levelInfo {
		return
	}
	log.Printf("[BOOT] stage=%s outcome=%s detail=%q", stage, outcome, detail)
}

// ---------- Retrieval Logging ----------

// LogRetrieval logs a single retrieval call with its scope, mode, and result count.
// mode is "vector" or "lexical". scope is "topic" or "notebook".
func LogRetrieval(scope, mode, id string, topK, got int, reason string) {
	if currentLogLevel() > levelInfo {
		return
	}
	log.Printf("[RETRIEVAL] scope=%s mode=%s id=%s topK=%d got=%d reason=%q", scope, mode, id, topK, got, reason)
}
