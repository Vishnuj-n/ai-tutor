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
