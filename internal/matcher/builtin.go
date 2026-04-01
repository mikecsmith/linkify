package matcher

import "regexp"

// Builtins returns the default matcher pipeline.
// Order matters: specific matchers first, generic fallback last.
func Builtins(cwd string) []Matcher {
	return []Matcher{
		// Python tracebacks: File "path", line N
		&RegexMatcher{
			MatcherName: "python-traceback",
			Pattern:     regexp.MustCompile(`File "(?P<file>[^"]+\.[\w]+)", line (?P<line>\d+)`),
		},

		// Go test/build output: package paths, test names
		NewGoMatcher(cwd),

		// Standard file:line:col (Go, TypeScript, Rust, etc.)
		&RegexMatcher{
			MatcherName: "file-line-col",
			Pattern:     regexp.MustCompile(`(?P<file>(?:[a-zA-Z]:)?(?:[./][\w./-]+|[\w][\w./-]+\.[\w]+)):(?P<line>\d+)(?::(?P<col>\d+))?`),
		},

		// Node.js stack traces: at Something (file:line:col)
		&RegexMatcher{
			MatcherName: "node-stack",
			Pattern:     regexp.MustCompile(`\((?P<file>(?:[a-zA-Z]:)?[/.][\w./-]+):(?P<line>\d+):(?P<col>\d+)\)`),
		},

		// Bare file paths without line numbers (Jest FAIL, pytest FAILED, etc.)
		&RegexMatcher{
			MatcherName: "bare-path",
			Pattern:     regexp.MustCompile(`(?P<file>(?:[a-zA-Z]:)?[\w./][\w./-]*/[\w./-]*\.[\w]+)(?::(?P<line>\d+))?`),
		},
	}
}
