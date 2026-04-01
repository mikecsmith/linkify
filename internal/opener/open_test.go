package opener

import (
	"fmt"
	"testing"

	"github.com/mikecsmith/linkify/internal/provider"
)

// --- Unit tests for helper functions ---

func TestIsNumeric(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"", false},
		{"0", true},
		{"1", true},
		{"42", true},
		{"100", true},
		{"abc", false},
		{"12a", false},
		{"-1", false},
		{"1.5", false},
		{" 1", false},
		{"1 ", false},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%q", tc.input), func(t *testing.T) {
			got := isNumeric(tc.input)
			if got != tc.want {
				t.Errorf("isNumeric(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestVimEscapeString(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"it's", "it''s"},
		{"'quoted'", "''quoted''"},
		{"no quotes", "no quotes"},
		{"a'b'c", "a''b''c"},
		{"", ""},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := vimEscapeString(tc.input)
			if got != tc.want {
				t.Errorf("vimEscapeString(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestIsNvimCmd(t *testing.T) {
	cases := []struct {
		name    string
		cmdline []string
		want    bool
	}{
		{"empty", nil, false},
		{"nvim", []string{"nvim"}, true},
		{"vim", []string{"vim"}, true},
		{"nvim with path", []string{"/opt/homebrew/bin/nvim"}, true},
		{"nvim with args", []string{"/usr/local/bin/nvim", "+42", "file.go"}, true},
		{"zsh", []string{"zsh"}, false},
		{"bash", []string{"/bin/bash"}, false},
		{"go", []string{"go", "test"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isNvimCmd(tc.cmdline)
			if got != tc.want {
				t.Errorf("isNvimCmd(%v) = %v, want %v", tc.cmdline, got, tc.want)
			}
		})
	}
}

func TestPaneHasNvim(t *testing.T) {
	t.Run("pane with nvim", func(t *testing.T) {
		p := &provider.Pane{
			Processes: []provider.PaneProcess{
				{PID: 100, Cmdline: []string{"/opt/homebrew/bin/nvim", "main.go"}},
			},
		}
		if !paneHasNvim(p) {
			t.Error("expected paneHasNvim to return true")
		}
	})

	t.Run("pane with shell only", func(t *testing.T) {
		p := &provider.Pane{
			Processes: []provider.PaneProcess{
				{PID: 100, Cmdline: []string{"/bin/zsh"}},
			},
		}
		if paneHasNvim(p) {
			t.Error("expected paneHasNvim to return false")
		}
	})

	t.Run("pane with no processes", func(t *testing.T) {
		p := &provider.Pane{}
		if paneHasNvim(p) {
			t.Error("expected paneHasNvim to return false")
		}
	})
}

func TestPaneNvimPID(t *testing.T) {
	p := &provider.Pane{
		Processes: []provider.PaneProcess{
			{PID: 100, Cmdline: []string{"/bin/zsh"}},
			{PID: 200, Cmdline: []string{"/opt/homebrew/bin/nvim", "main.go"}},
		},
	}
	got := paneNvimPID(p)
	if got != 200 {
		t.Errorf("paneNvimPID = %d, want 200", got)
	}

	empty := &provider.Pane{}
	if paneNvimPID(empty) != 0 {
		t.Error("expected paneNvimPID to return 0 for empty pane")
	}
}

func TestFindNvim(t *testing.T) {
	t.Run("override takes priority", func(t *testing.T) {
		got := findNvim("/custom/path/nvim")
		if got != "/custom/path/nvim" {
			t.Errorf("findNvim with override = %q, want /custom/path/nvim", got)
		}
	})

	t.Run("empty override does lookup", func(t *testing.T) {
		got := findNvim("")
		if got == "" {
			t.Error("findNvim with empty override should return something")
		}
	})
}

// --- mockProvider for openInPane tests ---

type mockProvider struct {
	name        string
	focused     []string
	launchErr   error
	launchCalls []mockLaunchCall
}

type mockLaunchCall struct {
	panes     []provider.Pane
	sourcePID int
	args      []string
}

func (m *mockProvider) Name() string { return m.name }

func (m *mockProvider) FindPanesByPID(pid int) ([]provider.Pane, error) {
	return nil, fmt.Errorf("not implemented in mock")
}

func (m *mockProvider) FocusPane(id string) error {
	m.focused = append(m.focused, id)
	return nil
}

func (m *mockProvider) LaunchInPane(panes []provider.Pane, sourcePID int, cwd string, args []string) error {
	m.launchCalls = append(m.launchCalls, mockLaunchCall{panes, sourcePID, args})
	return m.launchErr
}

// --- openInPane pane selection tests ---
//
// These tests verify the pane selection logic without requiring real nvim
// or terminal instances. The sendToNvim and isSocketAlive calls will fail
// (no real sockets), so these tests exercise the fallback to LaunchInPane.

func TestOpenInPane_NoNvim_DelegatesToProvider(t *testing.T) {
	panes := []provider.Pane{
		{ID: "1", PID: 100, Columns: 80, Lines: 24},
		{ID: "2", PID: 200, Columns: 160, Lines: 48},
	}
	mock := &mockProvider{name: "test", launchErr: nil}

	result := openInPane(mock, panes, "nvim", "/tmp/test.go", "42", "1", 100)

	if !result {
		t.Fatal("expected openInPane to return true when LaunchInPane succeeds")
	}
	if len(mock.launchCalls) != 1 {
		t.Fatalf("expected 1 LaunchInPane call, got %d", len(mock.launchCalls))
	}
	call := mock.launchCalls[0]
	if call.sourcePID != 100 {
		t.Errorf("LaunchInPane sourcePID = %d, want 100", call.sourcePID)
	}
	if len(call.args) != 3 || call.args[0] != "nvim" || call.args[1] != "+42" {
		t.Errorf("LaunchInPane args = %v, want [nvim +42 /tmp/test.go]", call.args)
	}
}

func TestOpenInPane_ProviderFails_ReturnsFalse(t *testing.T) {
	panes := []provider.Pane{
		{ID: "1", PID: 100, Columns: 80, Lines: 24},
	}
	mock := &mockProvider{name: "test", launchErr: fmt.Errorf("no pane awareness")}

	result := openInPane(mock, panes, "nvim", "/tmp/test.go", "10", "1", 100)

	if result {
		t.Fatal("expected openInPane to return false when LaunchInPane fails")
	}
}

func TestOpenInPane_InvalidLine_DefaultsToOne(t *testing.T) {
	panes := []provider.Pane{
		{ID: "1", PID: 100, Columns: 80, Lines: 24},
	}
	mock := &mockProvider{name: "test", launchErr: nil}

	openInPane(mock, panes, "nvim", "/tmp/test.go", "abc", "1", 100)

	if len(mock.launchCalls) != 1 {
		t.Fatalf("expected 1 LaunchInPane call, got %d", len(mock.launchCalls))
	}
	args := mock.launchCalls[0].args
	if args[1] != "+1" {
		t.Errorf("expected line to default to +1, got %s", args[1])
	}
}

func TestOpenInPane_NvimServerDead_FallsThrough(t *testing.T) {
	// Pane has NvimServer set but the socket won't be alive (fake path).
	// Should fall through to LaunchInPane.
	panes := []provider.Pane{
		{
			ID: "1", PID: 100, Columns: 160, Lines: 48,
			NvimServer: "/tmp/nonexistent-nvim-socket",
		},
	}
	mock := &mockProvider{name: "test", launchErr: nil}

	result := openInPane(mock, panes, "nvim", "/tmp/test.go", "10", "1", 100)

	if !result {
		t.Fatal("expected openInPane to return true via LaunchInPane fallback")
	}
	if len(mock.launchCalls) != 1 {
		t.Fatalf("expected LaunchInPane to be called, got %d calls", len(mock.launchCalls))
	}
}

func TestOpenInPane_NvimProcess_DeadSocket_FallsThrough(t *testing.T) {
	// Pane reports nvim as foreground process but socket scan won't find
	// a valid socket (fake PID). Should fall through to LaunchInPane.
	panes := []provider.Pane{
		{
			ID: "1", PID: 100, Columns: 160, Lines: 48,
			Processes: []provider.PaneProcess{
				{PID: 99999, Cmdline: []string{"/opt/homebrew/bin/nvim", "main.go"}},
			},
		},
	}
	mock := &mockProvider{name: "test", launchErr: nil}

	result := openInPane(mock, panes, "nvim", "/tmp/test.go", "10", "1", 100)

	if !result {
		t.Fatal("expected openInPane to return true via LaunchInPane fallback")
	}
	if len(mock.launchCalls) != 1 {
		t.Fatal("expected LaunchInPane to be called after dead nvim socket")
	}
}
