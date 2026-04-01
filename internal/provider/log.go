package provider

import "github.com/mikecsmith/linkify/internal/logutil"

// LogOpen writes a log entry. Delegates to the shared logutil package.
func LogOpen(format string, args ...any) {
	logutil.Log(format, args...)
}
