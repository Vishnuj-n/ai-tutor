package utils

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

var (
	queueLogFile *os.File
	ragLogFile   *os.File
	errLogFile   *os.File

	QueueLogger *log.Logger
	RagLogger   *log.Logger
	ErrLogger   *log.Logger

	logMutex sync.Mutex
)

func init() {
	// Initialize default loggers to write to stdout/stderr so they don't panic if not initialized
	QueueLogger = log.New(os.Stdout, "[QUEUE] ", log.LstdFlags)
	RagLogger = log.New(os.Stdout, "[RAG] ", log.LstdFlags)
	ErrLogger = log.New(os.Stderr, "[CRITICAL_ERROR] ", log.LstdFlags)
}

// InitMultiFileLogger programmatically verifies/creates the logs subdirectory
// under appDataDir and initializes the domain-separated file loggers.
func InitMultiFileLogger(appDataDir string) error {
	logMutex.Lock()
	defer logMutex.Unlock()

	logDir := filepath.Join(appDataDir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}

	// Close existing files if any are open (for safety in multi-call or testing environments)
	if queueLogFile != nil {
		_ = queueLogFile.Close()
	}
	if ragLogFile != nil {
		_ = ragLogFile.Close()
	}
	if errLogFile != nil {
		_ = errLogFile.Close()
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

	errLogFile, err = os.OpenFile(filepath.Join(logDir, "system_errors.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		_ = queueLogFile.Close()
		_ = ragLogFile.Close()
		queueLogFile = nil
		ragLogFile = nil
		return fmt.Errorf("failed to open system errors log file: %w", err)
	}

	// Initialize the thread-safe loggers pointing to the respective files
	QueueLogger = log.New(queueLogFile, "[QUEUE] ", log.LstdFlags)
	RagLogger = log.New(ragLogFile, "[RAG] ", log.LstdFlags)
	ErrLogger = log.New(errLogFile, "[CRITICAL_ERROR] ", log.LstdFlags)

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

	// Revert to fallback loggers on close to avoid nil dereference
	QueueLogger = log.New(os.Stdout, "[QUEUE] ", log.LstdFlags)
	RagLogger = log.New(os.Stdout, "[RAG] ", log.LstdFlags)
	ErrLogger = log.New(os.Stderr, "[CRITICAL_ERROR] ", log.LstdFlags)
}

// Global level helpers as wrappers pointing directly to standard logging formats

func Debugf(format string, args ...interface{}) {
	log.Printf("DEBUG: "+format, args...)
}

func Infof(format string, args ...interface{}) {
	log.Printf("INFO: "+format, args...)
}

func Warnf(format string, args ...interface{}) {
	log.Printf("WARN: "+format, args...)
}

func Errorf(format string, args ...interface{}) {
	log.Printf("ERROR: "+format, args...)
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
	QueueLogger.Printf("task=%s type=%s from=%s to=%s reason=%s", taskID, taskType, oldStatus, newStatus, reason)
}

// LogQueueTaskCreated logs when a new task is inserted into the queue.
func LogQueueTaskCreated(taskID, taskType, notebookID, topicID string) {
	if taskID == "" {
		taskID = "unknown"
	}
	if taskType == "" {
		taskType = "unknown"
	}
	QueueLogger.Printf("task=%s type=%s notebook=%s topic=%s event=task_inserted", taskID, taskType, notebookID, topicID)
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
	rereadInfo := "reread=none"
	if rereadTaskID != "" {
		rereadInfo = "reread=" + rereadTaskID
	}
	QueueLogger.Printf("task=%s type=QUIZ score=%d outcome=%s %s event=quiz_completed", taskID, score, outcome, rereadInfo)
}

// LogRereadInsertion logs when a reread task is generated after quiz failure.
func LogRereadInsertion(taskID, topicID, attemptCount, maxAttempts string) {
	if taskID == "" {
		taskID = "unknown"
	}
	QueueLogger.Printf("task=%s type=REREAD topic=%s attempt=%s/%s event=reread_inserted", taskID, topicID, attemptCount, maxAttempts)
}

// LogReviewSession logs review session lifecycle events.
func LogReviewSession(taskID, notebookID, cardCount, event string) {
	if taskID == "" {
		taskID = "unknown"
	}
	QueueLogger.Printf("task=%s type=FLASHCARD_REVIEW notebook=%s cards=%s event=%s", taskID, notebookID, cardCount, event)
}

// LogSchedulerDecision logs adaptive reading window decisions.
func LogSchedulerDecision(topicID string, startPage, endPage int, tokenBudget, reason string) {
	if topicID == "" {
		topicID = "unknown"
	}
	QueueLogger.Printf("topic=%s window=%d-%d tokenBudget=%s reason=%s", topicID, startPage, endPage, tokenBudget, reason)
}

// ---------- Boot / Init Logging ----------

// LogBoot logs a named RAG/boot initialization step with its outcome.
func LogBoot(stage, outcome, detail string) {
	RagLogger.Printf("stage=%s outcome=%s detail=%q", stage, outcome, detail)
}

// ---------- Retrieval Logging ----------

// LogRetrieval logs a single retrieval call with its scope, mode, and result count.
func LogRetrieval(scope, mode, id string, topK, got int, reason string) {
	RagLogger.Printf("scope=%s mode=%s id=%s topK=%d got=%d reason=%q", scope, mode, id, topK, got, reason)
}
