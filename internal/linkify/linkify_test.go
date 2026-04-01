package linkify

import (
	"strings"
	"testing"

	"github.com/mikecsmith/linkify/internal/matcher"
)

func TestLinkifyLine_GoTestOutput(t *testing.T) {
	line := `    server_test.go:42: expected 200, got 500`
	result, _ := LinkifyLine(line, "file://{file}:{line}", "/home/user/project", "", matcher.Builtins("/home/user/project"), 0)
	if result == line {
		t.Fatal("expected line to be linkified, got original")
	}
	if !strings.Contains(result, OscOpen) {
		t.Fatalf("expected OSC8 link, got: %q", result)
	}
	if !strings.Contains(result, "file:///home/user/project/server_test.go:42") {
		t.Fatalf("expected absolute file URL, got: %q", result)
	}
}

func TestLinkifyLine_AbsolutePath(t *testing.T) {
	line := `Error at /usr/src/app/main.go:100:5`
	result, _ := LinkifyLine(line, "file://{file}:{line}", "/whatever", "", matcher.Builtins("/whatever"), 0)
	if !strings.Contains(result, "file:///usr/src/app/main.go:100") {
		t.Fatalf("expected absolute path preserved, got: %q", result)
	}
}

func TestLinkifyLine_TypeScriptOutput(t *testing.T) {
	line := `src/components/Button.tsx:15:3 - error TS2322: Type 'string' is not assignable`
	result, _ := LinkifyLine(line, "file://{file}:{line}:{col}", "/home/user/app", "", matcher.Builtins("/home/user/app"), 0)
	if !strings.Contains(result, OscOpen) {
		t.Fatalf("expected OSC8 link, got: %q", result)
	}
	if !strings.Contains(result, "file:///home/user/app/src/components/Button.tsx:15:3") {
		t.Fatalf("expected correct URL, got: %q", result)
	}
}

func TestLinkifyLine_NodeStackTrace(t *testing.T) {
	line := `    at Object.<anonymous> (/home/user/app/index.js:42:13)`
	result, _ := LinkifyLine(line, "file://{file}:{line}", "/whatever", "", matcher.Builtins("/whatever"), 0)
	if !strings.Contains(result, OscOpen) {
		t.Fatalf("expected OSC8 link, got: %q", result)
	}
	if !strings.Contains(result, "file:///home/user/app/index.js:42") {
		t.Fatalf("expected correct URL, got: %q", result)
	}
}

func TestLinkifyLine_NoMatch(t *testing.T) {
	line := `PASS ok github.com/user/repo 0.042s`
	result, _ := LinkifyLine(line, "file://{file}:{line}", "/home/user", "", matcher.Builtins("/home/user"), 0)
	if result != line {
		t.Fatalf("expected no change, got: %q", result)
	}
}

func TestLinkifyLine_PreservesANSI(t *testing.T) {
	line := "\033[31mserver_test.go:42\033[0m: test failed"
	result, _ := LinkifyLine(line, "file://{file}:{line}", "/home/user/project", "", matcher.Builtins("/home/user/project"), 0)
	if !strings.Contains(result, "\033[31m") {
		t.Fatalf("expected ANSI codes preserved, got: %q", result)
	}
	if !strings.Contains(result, OscOpen) {
		t.Fatalf("expected OSC8 link, got: %q", result)
	}
}

func TestLinkifyLine_MultipleMatches(t *testing.T) {
	line := `main.go:10 imports utils.go:20`
	result, _ := LinkifyLine(line, "file://{file}:{line}", "/project", "", matcher.Builtins("/project"), 0)
	if !strings.Contains(result, "file:///project/main.go:10") {
		t.Fatalf("expected first link, got: %q", result)
	}
	if !strings.Contains(result, "file:///project/utils.go:20") {
		t.Fatalf("expected second link, got: %q", result)
	}
}

func TestLinkifyLine_CustomTemplate(t *testing.T) {
	line := `  main.go:42:5`
	result, _ := LinkifyLine(line, "vscode://file/{file}:{line}:{col}", "/home/user/project", "", matcher.Builtins("/home/user/project"), 0)
	if !strings.Contains(result, "vscode://file//home/user/project/main.go:42:5") {
		t.Fatalf("expected vscode URL, got: %q", result)
	}
}

func TestLinkifyLine_SkipsNonFiles(t *testing.T) {
	line := `time:42 is not a file reference`
	result, _ := LinkifyLine(line, "file://{file}:{line}", "/home/user", "", matcher.Builtins("/home/user"), 0)
	if result != line {
		t.Fatalf("expected no change for non-file, got: %q", result)
	}
}

func TestBuildPositionMap(t *testing.T) {
	original := "hello \033[31mworld\033[0m end"
	posMap := BuildPositionMap(original)

	if posMap[0] != 0 {
		t.Fatalf("expected posMap[0]=0, got %d", posMap[0])
	}
	if posMap[6] != 11 {
		t.Fatalf("expected posMap[6]=11, got %d", posMap[6])
	}
}

// ============================================================
// Real-world test output from various frameworks
// ============================================================

// --- Go ---

func TestReal_GoTestVerbose(t *testing.T) {
	lines := []struct {
		input string
		want  string
	}{
		{`    server_test.go:42: expected 200, got 500`, "server_test.go:42"},
		{`    handler_test.go:128: assertion failed`, "handler_test.go:128"},
		{`--- FAIL: TestCreateUser (0.00s)`, ""},
		{`FAIL	github.com/user/repo/pkg 0.003s`, ""},
		{`panic: runtime error: index out of range [5] with length 3`, ""},
		{`	/Users/mike/project/main.go:55 +0x1a4`, "main.go:55"},
		{`goroutine 1 [running]:`, ""},
	}
	runCases(t, lines, "/Users/mike/project")
}

func TestReal_GoCompileError(t *testing.T) {
	lines := []struct {
		input string
		want  string
	}{
		{`./main.go:12:5: undefined: foo`, "main.go:12"},
		{`./internal/handler.go:44:10: cannot use x (variable of type string) as int`, "handler.go:44"},
	}
	runCases(t, lines, "/Users/mike/project")
}

func TestReal_GoVet(t *testing.T) {
	lines := []struct {
		input string
		want  string
	}{
		{`# github.com/user/repo/pkg`, ""},
		{`pkg/handler.go:23:2: printf: fmt.Sprintf format %d has arg of wrong type`, "handler.go:23"},
	}
	runCases(t, lines, "/Users/mike/project")
}

// --- Jest (JavaScript/TypeScript) ---

func TestReal_Jest(t *testing.T) {
	lines := []struct {
		input string
		want  string
	}{
		{`  ● Auth › should validate token`, ""},
		{`    expect(received).toBe(expected)`, ""},
		{`      at Object.<anonymous> (src/auth/__tests__/auth.test.ts:42:5)`, "auth.test.ts:42"},
		{`      at Object.<anonymous> (/Users/mike/app/src/utils.ts:18:11)`, "utils.ts:18"},
		{`  FAIL src/auth/__tests__/auth.test.ts`, "auth.test.ts"},
		{`Test Suites: 1 failed, 3 passed, 4 total`, ""},
	}
	runCases(t, lines, "/Users/mike/app")
}

// --- Vitest ---

func TestReal_Vitest(t *testing.T) {
	lines := []struct {
		input string
		want  string
	}{
		{` ❯ src/components/__tests__/Button.test.tsx:15:3`, "Button.test.tsx:15"},
		{`    14|   expect(screen.getByRole('button')).toBeInTheDocument()`, ""},
		{`  → 15|   expect(result).toBe(true)`, ""},
		{`    16| })`, ""},
		{` FAIL  src/components/__tests__/Button.test.tsx > Button > renders`, "Button.test.tsx"},
	}
	runCases(t, lines, "/Users/mike/app")
}

// --- Mocha ---

func TestReal_Mocha(t *testing.T) {
	lines := []struct {
		input string
		want  string
	}{
		{`  1) should return 200`, ""},
		{`    AssertionError: expected 404 to equal 200`, ""},
		{`      at Context.<anonymous> (test/api.test.js:25:14)`, "api.test.js:25"},
		{`      at process.processImmediate (node:internal/timers:478:21)`, ""},
		{`  1 failing`, ""},
	}
	runCases(t, lines, "/Users/mike/app")
}

// --- TypeScript Compiler (tsc) ---

func TestReal_TSC(t *testing.T) {
	lines := []struct {
		input string
		want  string
	}{
		{`src/index.ts:10:5 - error TS2322: Type 'string' is not assignable to type 'number'.`, "index.ts:10"},
		{`src/utils/helpers.ts:44:12 - error TS2345: Argument of type 'null' is not assignable`, "helpers.ts:44"},
		{`Found 2 errors in 2 files.`, ""},
	}
	runCases(t, lines, "/Users/mike/app")
}

// --- ESLint ---

func TestReal_ESLint(t *testing.T) {
	lines := []struct {
		input string
		want  string
	}{
		{`/Users/mike/app/src/index.ts`, "index.ts"},
		{`  10:5   error  'foo' is not defined  no-undef`, ""},
		{`  22:1   warning  Unexpected console statement  no-console`, ""},
	}
	runCases(t, lines, "/Users/mike/app")
}

// --- Pytest ---

func TestReal_Pytest(t *testing.T) {
	lines := []struct {
		input string
		want  string
	}{
		{`FAILED tests/test_auth.py::test_login - AssertionError`, "test_auth.py"},
		{`tests/test_auth.py:42: AssertionError`, "test_auth.py:42"},
		{`E       assert 401 == 200`, ""},
		{`    /Users/mike/project/src/auth.py:18: in login`, "auth.py:18"},
	}
	runCases(t, lines, "/Users/mike/project")
}

// --- Rust (cargo test) ---

func TestReal_Cargo(t *testing.T) {
	lines := []struct {
		input string
		want  string
	}{
		{`error[E0308]: mismatched types`, ""},
		{`  --> src/main.rs:42:5`, "main.rs:42"},
		{`   |`, ""},
		{`42 |     let x: i32 = "hello";`, ""},
		{`   |                  ^^^^^^^ expected i32, found &str`, ""},
		{`thread 'tests::test_parse' panicked at src/parser.rs:100:9`, "parser.rs:100"},
		{`note: run with RUST_BACKTRACE=1`, ""},
	}
	runCases(t, lines, "/Users/mike/project")
}

// --- Python traceback ---

func TestReal_PythonTraceback(t *testing.T) {
	lines := []struct {
		input string
		want  string
	}{
		{`Traceback (most recent call last):`, ""},
		{`  File "/Users/mike/project/main.py", line 42, in <module>`, "main.py"},
		{`  File "src/utils.py", line 18, in helper`, "utils.py"},
		{`TypeError: unsupported operand type(s)`, ""},
	}
	runCases(t, lines, "/Users/mike/project")
}

// --- Go panic stack trace ---

func TestReal_GoPanic(t *testing.T) {
	lines := []struct {
		input string
		want  string
	}{
		{`goroutine 1 [running]:`, ""},
		{`main.main()`, ""},
		{`	/Users/mike/project/main.go:42 +0x68`, "main.go:42"},
		{`	/Users/mike/project/internal/server.go:108 +0x1a4`, "server.go:108"},
	}
	runCases(t, lines, "/Users/mike/project")
}

// --- False positives that should NOT match ---

func TestReal_FalsePositives(t *testing.T) {
	lines := []struct {
		input string
		want  string
	}{
		{`listening on http://localhost:3000`, ""},
		{`connected to postgres://user:pass@host:5432/db`, ""},
		{`Duration: 42s`, ""},
		{`Step 3/10 : RUN npm install`, ""},
		{`2024-01-15T10:30:42Z INFO starting server`, ""},
		{`sha256:abc123def456`, ""},
		{`v1.2.3`, ""},
		{`node:internal/modules/cjs/loader:1078`, ""},
	}
	runCases(t, lines, "/Users/mike/project")
}

// --- Rust arrow format ---

func TestReal_RustArrow(t *testing.T) {
	line := `  --> src/main.rs:42:5`
	result, _ := LinkifyLine(line, "file://{file}:{line}", "/Users/mike/project", "", matcher.Builtins("/Users/mike/project"), 0)
	if !strings.Contains(result, OscOpen) {
		t.Fatalf("expected OSC8 link for Rust arrow, got: %q", result)
	}
}

// ============================================================
// Helpers
// ============================================================

func runCases(t *testing.T, cases []struct {
	input string
	want  string
}, cwd string) {
	t.Helper()
	matchers := matcher.Builtins(cwd)
	for _, tc := range cases {
		result, _ := LinkifyLine(tc.input, "file://{file}:{line}", cwd, "", matchers, 0)
		if tc.want == "" {
			if strings.Contains(result, OscOpen) {
				t.Errorf("false positive linkification:\n  input: %q\n  got:   %q", tc.input, result)
			}
		} else {
			if !strings.Contains(result, OscOpen) {
				t.Errorf("expected linkification for %q but got none:\n  result: %q", tc.want, result)
			}
			if !strings.Contains(result, tc.want) {
				t.Errorf("expected URL to contain %q:\n  input:  %q\n  result: %q", tc.want, tc.input, result)
			}
		}
	}
}
