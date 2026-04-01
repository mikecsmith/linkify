package linkify

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mikecsmith/linkify/internal/matcher"
)

const (
	OscOpen  = "\033]8;;"
	OscSep   = "\033\\"
	OscClose = "\033]8;;\033\\"
)

// AnsiPattern matches ANSI escape sequences.
var AnsiPattern = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// LinkifyLineDryRun returns the line with matched URLs appended as plain text.
func LinkifyLineDryRun(line, urlTemplate, cwd, pid string, matchers []matcher.Matcher) string {
	stripped := AnsiPattern.ReplaceAllString(line, "")

	matches := FilterMatches(matcher.RunAll(matchers, stripped))
	if len(matches) == 0 {
		return line
	}

	var urls []string
	for _, m := range matches {
		u := BuildURL(urlTemplate, ResolvePath(m.File, cwd), m.Line, m.Col, pid)
		urls = append(urls, u)
	}

	return line + "\n  → " + strings.Join(urls, "\n  → ")
}

// LinkifyLine wraps file:line references in OSC8 hyperlinks. Returns the
// modified line and the updated link ID counter.
func LinkifyLine(line, urlTemplate, cwd, pid string, matchers []matcher.Matcher, linkID int) (string, int) {
	stripped := AnsiPattern.ReplaceAllString(line, "")

	matches := FilterMatches(matcher.RunAll(matchers, stripped))
	if len(matches) == 0 {
		return line, linkID
	}

	posMap := BuildPositionMap(line)

	result := line
	for i := len(matches) - 1; i >= 0; i-- {
		m := matches[i]
		linkID++
		url := BuildURL(urlTemplate, ResolvePath(m.File, cwd), m.Line, m.Col, pid)

		linkStart := posMap[m.FileStart]
		linkEnd := posMap[m.FileEnd]
		if m.LineEnd > 0 {
			linkEnd = posMap[m.LineEnd]
		}

		// OSC8 id= param ensures terminals treat each link as unique
		osc := fmt.Sprintf("\033]8;id=lfy%d;%s\033\\", linkID, url)
		visibleText := result[linkStart:linkEnd]
		linked := result[posMap[m.Start]:linkStart] + osc + visibleText + OscClose + result[linkEnd:posMap[m.End]]
		result = result[:posMap[m.Start]] + linked + result[posMap[m.End]:]
	}

	return result, linkID
}

// FilterMatches validates and normalizes matches.
func FilterMatches(matches []matcher.Match) []matcher.Match {
	var result []matcher.Match
	for _, m := range matches {
		if !LooksLikeFile(m.File) {
			continue
		}
		if m.Line == "" {
			m.Line = "1"
		}
		result = append(result, m)
	}
	return result
}

// BuildURL fills in the URL template placeholders.
// The file path is encoded to handle spaces and URL-sensitive characters
// while preserving path separators for compatibility with both path-based
// (file://{file}) and query-based (lfy://open?file={file}) templates.
func BuildURL(tmpl, file, line, col, pid string) string {
	if col == "" {
		col = "1"
	}
	u := tmpl
	u = strings.ReplaceAll(u, "{file}", encodeFilePath(file))
	u = strings.ReplaceAll(u, "{line}", line)
	u = strings.ReplaceAll(u, "{col}", col)
	u = strings.ReplaceAll(u, "{pid}", pid)
	return u
}

// encodeFilePath encodes characters that are problematic in URLs while
// preserving path separators. This is a middle ground between path encoding
// (too permissive for query params) and query encoding (breaks path-based URLs).
func encodeFilePath(path string) string {
	var b strings.Builder
	b.Grow(len(path))
	for i := 0; i < len(path); i++ {
		c := path[i]
		switch c {
		case ' ', '&', '?', '#', '%':
			fmt.Fprintf(&b, "%%%02X", c)
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// BuildPositionMap maps stripped-text indices to original-string indices,
// skipping over ANSI escape sequences.
func BuildPositionMap(original string) []int {
	locs := AnsiPattern.FindAllStringIndex(original, -1)
	posMap := make([]int, 0, len(original)+1)
	origIdx := 0
	locIdx := 0

	for origIdx <= len(original) {
		if locIdx < len(locs) && origIdx == locs[locIdx][0] {
			origIdx = locs[locIdx][1]
			locIdx++
			continue
		}
		posMap = append(posMap, origIdx)
		if origIdx < len(original) {
			origIdx++
		} else {
			break
		}
	}
	return posMap
}

// ResolvePath makes a relative path absolute using cwd.
func ResolvePath(path, cwd string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(cwd, path))
}
