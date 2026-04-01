package matcher

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// GoMatcher understands Go test/build output and resolves:
//   - Package paths (ok/FAIL github.com/foo/bar/internal/tui) → directory
//   - Test function names (--- FAIL: TestFoo) → file:line
//   - Subtest names (=== RUN TestFoo/subcase) → file:line
type GoMatcher struct {
	cwd     string
	modPath string // e.g. "github.com/mikecsmith/ihj"
	modRoot string // e.g. "/Users/mike/code/ihj"
	modOnce sync.Once

	currentPkg string // tracks the current package directory across lines

	// Cache: package dir → map of test func name → file:line
	testIndex   map[string]map[string]testLocation
	testIndexMu sync.Mutex

	// Module-wide index: test func name → file:line (fallback when currentPkg is unknown)
	globalIndex     map[string]testLocation
	globalIndexOnce sync.Once
}

type testLocation struct {
	file string
	line string
}

var (
	goPackageResultRe = regexp.MustCompile(`^(?:ok|FAIL)\s+(\S+)\s+[\d.]+s`)
	goRunRe           = regexp.MustCompile(`^=== RUN\s+(\S+)`)
	goResultRe        = regexp.MustCompile(`---\s+(?:FAIL|PASS|SKIP):\s+(\S+)\s+\(`)
	goTestFuncRe      = regexp.MustCompile(`^func\s+(Test\w+)\s*\(`)
	goSubtestRe       = regexp.MustCompile(`\.Run\(\s*"([^"]+)"`)
)

func NewGoMatcher(cwd string) *GoMatcher {
	return &GoMatcher{
		cwd:       cwd,
		testIndex: make(map[string]map[string]testLocation),
	}
}

func (g *GoMatcher) Name() string { return "go" }

func (g *GoMatcher) Match(line string) []Match {
	g.modOnce.Do(g.loadModule)

	if m := goPackageResultRe.FindStringSubmatchIndex(line); m != nil {
		pkg := line[m[2]:m[3]]
		return g.matchPackageLine(pkg, m)
	}

	if m := goRunRe.FindStringSubmatchIndex(line); m != nil {
		testName := line[m[2]:m[3]]
		return g.matchTestName(testName, m[2], m[3])
	}

	if m := goResultRe.FindStringSubmatchIndex(line); m != nil {
		testName := line[m[2]:m[3]]
		return g.matchTestName(testName, m[2], m[3])
	}

	return nil
}

func (g *GoMatcher) matchPackageLine(pkg string, m []int) []Match {
	if g.modPath == "" {
		return nil
	}

	dir := g.resolvePackageDir(pkg)
	if dir == "" {
		return nil
	}

	g.currentPkg = dir

	return []Match{{
		Start:     m[2],
		End:       m[3],
		FileStart: m[2],
		FileEnd:   m[3],
		File:      dir,
		Line:      "1",
	}}
}

func (g *GoMatcher) matchTestName(testName string, start, end int) []Match {
	topLevel := testName
	subtest := ""
	if idx := strings.IndexByte(testName, '/'); idx >= 0 {
		topLevel = testName[:idx]
		subtest = testName[idx+1:]
	}

	// Try per-package index first (most precise)
	if g.currentPkg != "" {
		loc := g.findTest(g.currentPkg, topLevel, subtest)
		if loc.file != "" {
			return []Match{{
				Start: start, End: end,
				FileStart: start, FileEnd: end,
				File: loc.file, Line: loc.line,
			}}
		}
	}

	// Fallback: module-wide index (handles test names before ok/FAIL line)
	loc := g.findTestGlobal(topLevel, subtest)
	if loc.file == "" {
		return nil
	}

	return []Match{{
		Start: start, End: end,
		FileStart: start, FileEnd: end,
		File: loc.file, Line: loc.line,
	}}
}

func (g *GoMatcher) resolvePackageDir(pkg string) string {
	if !strings.HasPrefix(pkg, g.modPath) {
		return ""
	}

	rel := strings.TrimPrefix(pkg, g.modPath)
	rel = strings.TrimPrefix(rel, "/")

	if rel == "" {
		return g.modRoot
	}
	return filepath.Join(g.modRoot, rel)
}

func (g *GoMatcher) findTest(pkgDir, funcName, subtestName string) testLocation {
	g.testIndexMu.Lock()
	defer g.testIndexMu.Unlock()

	if _, ok := g.testIndex[pkgDir]; !ok {
		g.indexPackageTests(pkgDir)
	}

	idx := g.testIndex[pkgDir]
	if idx == nil {
		return testLocation{}
	}

	if subtestName != "" {
		key := funcName + "/" + subtestName
		if loc, ok := idx[key]; ok {
			return loc
		}
	}

	if loc, ok := idx[funcName]; ok {
		return loc
	}

	return testLocation{}
}

func (g *GoMatcher) findTestGlobal(funcName, subtestName string) testLocation {
	g.modOnce.Do(g.loadModule)
	if g.modRoot == "" {
		return testLocation{}
	}

	g.globalIndexOnce.Do(g.buildGlobalIndex)

	if subtestName != "" {
		key := funcName + "/" + subtestName
		if loc, ok := g.globalIndex[key]; ok {
			return loc
		}
	}

	if loc, ok := g.globalIndex[funcName]; ok {
		return loc
	}

	return testLocation{}
}

func (g *GoMatcher) buildGlobalIndex() {
	g.globalIndex = make(map[string]testLocation)

	_ = filepath.WalkDir(g.modRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		// Skip hidden dirs and vendor
		if d.IsDir() {
			base := d.Name()
			if strings.HasPrefix(base, ".") || base == "vendor" || base == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		g.indexTestFile(path, g.globalIndex)
		return nil
	})
}

func (g *GoMatcher) indexPackageTests(pkgDir string) {
	idx := make(map[string]testLocation)
	g.testIndex[pkgDir] = idx

	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		return
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}

		filePath := filepath.Join(pkgDir, e.Name())
		g.indexTestFile(filePath, idx)
	}
}

func (g *GoMatcher) indexTestFile(filePath string, idx map[string]testLocation) {
	f, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	currentFunc := ""

	for scanner.Scan() {
		lineNum++
		text := scanner.Text()

		if m := goTestFuncRe.FindStringSubmatch(text); m != nil {
			currentFunc = m[1]
			idx[currentFunc] = testLocation{
				file: filePath,
				line: strconv.Itoa(lineNum),
			}
			continue
		}

		if currentFunc != "" {
			if m := goSubtestRe.FindStringSubmatch(text); m != nil {
				key := currentFunc + "/" + m[1]
				idx[key] = testLocation{
					file: filePath,
					line: strconv.Itoa(lineNum),
				}
			}
		}

		if currentFunc != "" && strings.HasPrefix(text, "}") {
			currentFunc = ""
		}
	}
}

func (g *GoMatcher) loadModule() {
	dir := g.cwd
	for {
		modFile := filepath.Join(dir, "go.mod")
		data, err := os.ReadFile(modFile)
		if err == nil {
			g.modRoot = dir
			g.modPath = parseModulePath(data)
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return
		}
		dir = parent
	}
}

func parseModulePath(data []byte) string {
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module"))
		}
	}
	return ""
}
