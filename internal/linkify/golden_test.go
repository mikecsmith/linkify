package linkify

import (
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mikecsmith/linkify/internal/matcher"
)

var update = flag.Bool("update", false, "update .expected golden files")

func TestGolden(t *testing.T) {
	inputs, err := filepath.Glob("testdata/*.input")
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) == 0 {
		t.Fatal("no testdata/*.input files found")
	}

	for _, inputPath := range inputs {
		name := strings.TrimSuffix(filepath.Base(inputPath), ".input")
		expectedPath := strings.TrimSuffix(inputPath, ".input") + ".expected"

		t.Run(name, func(t *testing.T) {
			inputData, err := os.ReadFile(inputPath)
			if err != nil {
				t.Fatal(err)
			}

			// Use a fixed cwd so golden files are portable
			cwd := "/test/project"
			urlTemplate := "file://{file}:{line}:{col}"
			matchers := matcher.Builtins(cwd)

			var output strings.Builder
			for _, line := range strings.Split(string(inputData), "\n") {
				result := LinkifyLineDryRun(line, urlTemplate, cwd, "0", matchers)
				output.WriteString(result)
				output.WriteString("\n")
			}

			got := output.String()

			if *update {
				if err := os.WriteFile(expectedPath, []byte(got), 0644); err != nil {
					t.Fatal(err)
				}
				t.Logf("updated %s", expectedPath)
				return
			}

			expectedData, err := os.ReadFile(expectedPath)
			if err != nil {
				t.Fatalf("missing golden file %s (run with -update to create)", expectedPath)
			}

			expected := string(expectedData)
			if got != expected {
				t.Errorf("output mismatch for %s\n\n--- expected ---\n%s\n--- got ---\n%s\n--- diff ---\n%s",
					name, expected, got, diff(expected, got))
			}
		})
	}
}

func diff(expected, got string) string {
	expectedLines := strings.Split(expected, "\n")
	gotLines := strings.Split(got, "\n")

	var b strings.Builder
	maxLen := len(expectedLines)
	if len(gotLines) > maxLen {
		maxLen = len(gotLines)
	}

	for i := 0; i < maxLen; i++ {
		var e, g string
		if i < len(expectedLines) {
			e = expectedLines[i]
		}
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if e != g {
			b.WriteString("  line ")
			b.WriteString(strconv.Itoa(i + 1))
			b.WriteString(":\n")
			b.WriteString("    expected: ")
			b.WriteString(e)
			b.WriteString("\n")
			b.WriteString("    got:      ")
			b.WriteString(g)
			b.WriteString("\n")
		}
	}
	return b.String()
}
