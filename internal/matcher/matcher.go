package matcher

import (
	"regexp"
)

// Match represents a linkifiable span found in a line.
type Match struct {
	// Position in the ANSI-stripped line
	Start, End int

	// Where the visible link text starts/ends (file:line portion)
	FileStart, FileEnd int
	LineEnd            int // end of :line portion (0 if no line number)

	// Resolved values
	File string
	Line string
	Col  string
}

// Matcher finds linkifiable spans in lines of output.
// Stateful matchers (e.g. GoMatcher) may track context across lines.
type Matcher interface {
	Name() string
	Match(line string) []Match
}

// RegexMatcher is a stateless matcher driven by a compiled regex
// with named capture groups: file, line, col.
type RegexMatcher struct {
	MatcherName string
	Pattern     *regexp.Regexp
	Extract     func(pattern *regexp.Regexp, line string, loc []int) []Match
}

func (m *RegexMatcher) Name() string { return m.MatcherName }

func (m *RegexMatcher) Match(line string) []Match {
	var results []Match
	for _, loc := range m.Pattern.FindAllStringSubmatchIndex(line, -1) {
		if m.Extract != nil {
			results = append(results, m.Extract(m.Pattern, line, loc)...)
		} else {
			if match := ExtractNamedGroups(m.Pattern, line, loc); match != nil {
				results = append(results, *match)
			}
		}
	}
	return results
}

// ExtractNamedGroups pulls file, line, col from named regex groups.
func ExtractNamedGroups(pat *regexp.Regexp, s string, loc []int) *Match {
	m := &Match{
		Start: loc[0],
		End:   loc[1],
	}
	for i, name := range pat.SubexpNames() {
		if i == 0 || name == "" || loc[2*i] < 0 {
			continue
		}
		val := s[loc[2*i]:loc[2*i+1]]
		switch name {
		case "file":
			m.File = val
			m.FileStart = loc[2*i]
			m.FileEnd = loc[2*i+1]
		case "line":
			m.Line = val
			m.LineEnd = loc[2*i+1]
		case "col":
			m.Col = val
		}
	}
	if m.File == "" {
		return nil
	}
	return m
}

// RunAll runs all matchers against a stripped line.
// Specific matchers run first; later matches that overlap earlier ones are skipped.
func RunAll(matchers []Matcher, stripped string) []Match {
	var all []Match
	for _, m := range matchers {
		for _, match := range m.Match(stripped) {
			if !overlaps(match, all) {
				all = append(all, match)
			}
		}
	}
	return all
}

func overlaps(m Match, existing []Match) bool {
	for _, prev := range existing {
		if m.Start < prev.End && m.End > prev.Start {
			return true
		}
	}
	return false
}
