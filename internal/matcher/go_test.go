package matcher

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGoMatcher_PackageResolution(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module github.com/example/myapp\n\ngo 1.21\n")
	os.MkdirAll(filepath.Join(dir, "internal", "tui"), 0755)
	writeFile(t, filepath.Join(dir, "internal", "tui", "view_test.go"), `package tui

import "testing"

func TestPopupInputBlackbox(t *testing.T) {
	t.Run("ShowInput_activates", func(t *testing.T) {
	})
	t.Run("input_escape_cancels", func(t *testing.T) {
	})
}
`)

	gm := NewGoMatcher(dir)
	matchers := []Matcher{gm}

	matches := RunAll(matchers, "ok  \tgithub.com/example/myapp/internal/tui\t9.259s")
	if len(matches) == 0 {
		t.Fatal("expected match for package line")
	}
	expectedDir := filepath.Join(dir, "internal", "tui")
	if matches[0].File != expectedDir {
		t.Fatalf("expected file=%q, got %q", expectedDir, matches[0].File)
	}

	matches = RunAll(matchers, "--- PASS: TestPopupInputBlackbox (0.00s)")
	if len(matches) == 0 {
		t.Fatal("expected match for test name")
	}
	expectedFile := filepath.Join(dir, "internal", "tui", "view_test.go")
	if matches[0].File != expectedFile {
		t.Fatalf("expected file=%q, got %q", expectedFile, matches[0].File)
	}
	if matches[0].Line != "5" {
		t.Fatalf("expected line=5, got %q", matches[0].Line)
	}

	matches = RunAll(matchers, "    --- PASS: TestPopupInputBlackbox/ShowInput_activates (0.00s)")
	if len(matches) == 0 {
		t.Fatal("expected match for subtest")
	}
	if matches[0].Line != "6" {
		t.Fatalf("expected line=6, got %q", matches[0].Line)
	}

	matches = RunAll(matchers, "=== RUN   TestPopupInputBlackbox/input_escape_cancels")
	if len(matches) == 0 {
		t.Fatal("expected match for RUN subtest")
	}
	if matches[0].Line != "8" {
		t.Fatalf("expected line=8, got %q", matches[0].Line)
	}
}

func TestGoMatcher_NoGoMod(t *testing.T) {
	dir := t.TempDir()
	gm := NewGoMatcher(dir)

	matches := gm.Match("ok  \tgithub.com/example/myapp/internal/tui\t9.259s")
	if len(matches) != 0 {
		t.Fatalf("expected no matches without go.mod, got %d", len(matches))
	}
}

func TestGoMatcher_FAIL(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module github.com/example/app\n\ngo 1.21\n")
	os.MkdirAll(filepath.Join(dir, "pkg"), 0755)
	writeFile(t, filepath.Join(dir, "pkg", "handler_test.go"), `package pkg

import "testing"

func TestHandler(t *testing.T) {
}
`)

	gm := NewGoMatcher(dir)

	matches := gm.Match("FAIL\tgithub.com/example/app/pkg\t0.003s")
	if len(matches) == 0 {
		t.Fatal("expected match for FAIL package line")
	}
	if matches[0].File != filepath.Join(dir, "pkg") {
		t.Fatalf("expected pkg dir, got %q", matches[0].File)
	}
}

func TestGoMatcher_RootPackage(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module github.com/example/app\n\ngo 1.21\n")

	gm := NewGoMatcher(dir)

	matches := gm.Match("ok  \tgithub.com/example/app\t1.234s")
	if len(matches) == 0 {
		t.Fatal("expected match for root package")
	}
	if matches[0].File != dir {
		t.Fatalf("expected root dir %q, got %q", dir, matches[0].File)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writeFile %s: %v", path, err)
	}
}
