package logutil

import (
	"fmt"
	"log/slog"
	"os"
	"sync"
)

const logFile = "/tmp/linkify.log"

var (
	logOnce   sync.Once
	logLogger *slog.Logger
)

func initLogger() {
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	logLogger = slog.New(slog.NewTextHandler(f, nil))
}

// Log writes a log entry to /tmp/linkify.log.
func Log(format string, args ...any) {
	logOnce.Do(initLogger)
	if logLogger == nil {
		return
	}
	logLogger.Info(fmt.Sprintf(format, args...))
}
