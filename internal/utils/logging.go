package utils

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

var (
	queueLogFile *os.File
	ragLogFile   *os.File
	errLogFile   *os.File

	// QueueLogger writes structured queue lifecycle events to queue.log.
	QueueLogger *slog.Logger
	// RagLogger writes structured RAG/boot events to rag_engine.log.
	RagLogger *slog.Logger

	logMutex sync.Mutex
)

func init() {
	// Default loggers write to stdout/stderr until InitMultiFileLogger is called.
	QueueLogger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	RagLogger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))
}

// InitMultiFileLogger creates the logs subdirectory under appDataDir and
// redirects QueueLogger, RagLogger, and the default slog logger to their
// respective files.
func InitMultiFileLogger(appDataDir string) error {
	logMutex.Lock()
	defer logMutex.Unlock()

	logDir := filepath.Join(appDataDir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}

	// Close existing files if any are open (for safety in multi-call or testing environments).
	if queueLogFile != nil {
		_ = queueLogFile.Close()
		queueLogFile = nil
	}
	if ragLogFile != nil {
		_ = ragLogFile.Close()
		ragLogFile = nil
	}
	if errLogFile != nil {
		_ = errLogFile.Close()
		errLogFile = nil
	}

	var err error
	queueLogFile, err = os.OpenFile(filepath.Join(logDir, "queue.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open queue log file: %w", err)
	}

	ragLogFile, err = os.OpenFile(filepath.Join(logDir, "rag_engine.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		_ = queueLogFile.Close()
		queueLogFile = nil
		return fmt.Errorf("failed to open rag engine log file: %w", err)
	}

	var openErr error
	errLogFile, openErr = os.OpenFile(filepath.Join(logDir, "system_errors.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if openErr != nil {
		_ = queueLogFile.Close()
		_ = ragLogFile.Close()
		queueLogFile = nil
		ragLogFile = nil
		errLogFile = nil
		return fmt.Errorf("failed to open system errors log file: %w", openErr)
	}

	QueueLogger = slog.New(slog.NewJSONHandler(queueLogFile, nil))
	RagLogger = slog.New(slog.NewJSONHandler(ragLogFile, nil))
	slog.SetDefault(slog.New(slog.NewJSONHandler(errLogFile, nil)))

	return nil
}

// CloseMultiFileLogger flushes and closes all active file handles.
func CloseMultiFileLogger() {
	logMutex.Lock()
	defer logMutex.Unlock()

	if queueLogFile != nil {
		_ = queueLogFile.Sync()
		_ = queueLogFile.Close()
		queueLogFile = nil
	}
	if ragLogFile != nil {
		_ = ragLogFile.Sync()
		_ = ragLogFile.Close()
		ragLogFile = nil
	}
	if errLogFile != nil {
		_ = errLogFile.Sync()
		_ = errLogFile.Close()
		errLogFile = nil
	}

	// Revert to stdout/stderr fallback loggers on close to avoid nil dereference.
	QueueLogger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	RagLogger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))
}

// ---------- Global Level Helpers ----------

func Debugf(format string, args ...any) {
	slog.Debug(fmt.Sprintf(format, args...))
}

func Infof(format string, args ...any) {
	slog.Info(fmt.Sprintf(format, args...))
}

func Warnf(format string, args ...any) {
	slog.Warn(fmt.Sprintf(format, args...))
}

func Errorf(format string, args ...any) {
	slog.Error(fmt.Sprintf(format, args...))
}

// ---------- Queue Lifecycle Logging ----------

// LogQueueTransition logs structured queue transition events.
func LogQueueTransition(taskID, taskType, oldStatus, newStatus, reason string) {
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
	QueueLogger.Info("queue_transition",
		"task", taskID, "type", taskType,
		"from", oldStatus, "to", newStatus, "reason", reason)
}

// LogQueueTaskCreated logs when a new task is inserted into the queue.
func LogQueueTaskCreated(taskID, taskType, notebookID, topicID string) {
	if taskID == "" {
		taskID = "unknown"
	}
	if taskType == "" {
		taskType = "unknown"
	}
	QueueLogger.Info("task_inserted",
		"task", taskID, "type", taskType,
		"notebook", notebookID, "topic", topicID)
}

// LogQuizResult logs quiz completion with pass/fail outcome.
func LogQuizResult(taskID string, score int, passed bool, rereadTaskID string) {
	if taskID == "" {
		taskID = "unknown"
	}
	outcome := "failed"
	if passed {
		outcome = "passed"
	}
	QueueLogger.Info("quiz_completed",
		"task", taskID, "type", "QUIZ",
		"score", score, "outcome", outcome, "reread", rereadTaskID)
}

// LogRereadInsertion logs when a reread task is generated after quiz failure.
func LogRereadInsertion(taskID, topicID, attemptCount, maxAttempts string) {
	if taskID == "" {
		taskID = "unknown"
	}
	QueueLogger.Info("reread_inserted",
		"task", taskID, "type", "REREAD",
		"topic", topicID, "attempt", attemptCount, "max", maxAttempts)
}

// LogReviewSession logs review session lifecycle events.
func LogReviewSession(taskID, notebookID, cardCount, event string) {
	if taskID == "" {
		taskID = "unknown"
	}
	QueueLogger.Info(event,
		"task", taskID, "type", "FLASHCARD_REVIEW",
		"notebook", notebookID, "cards", cardCount)
}

// LogSchedulerDecision logs adaptive reading window decisions.
func LogSchedulerDecision(topicID string, startPage, endPage int, tokenBudget, reason string) {
	if topicID == "" {
		topicID = "unknown"
	}
	QueueLogger.Info("scheduler_decision",
		"topic", topicID, "startPage", startPage, "endPage", endPage,
		"tokenBudget", tokenBudget, "reason", reason)
}

// ---------- Boot / Init Logging ----------

// LogBoot logs a named RAG/boot initialization step with its outcome.
func LogBoot(stage, outcome, detail string) {
	RagLogger.Info("boot", "stage", stage, "outcome", outcome, "detail", detail)
}

// ---------- Retrieval Logging ----------

// LogRetrieval logs a single retrieval call with its scope, mode, and result count.
func LogRetrieval(scope, mode, id string, topK, got int, reason string) {
	RagLogger.Info("retrieval", "scope", scope, "mode", mode, "id", id, "topK", topK, "got", got, "reason", reason)
}
