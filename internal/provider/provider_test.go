package provider

import (
	"encoding/json"
	"testing"

	"github.com/mikecsmith/linkify/internal/process"
)

func TestFindPaneByPID(t *testing.T) {
	panes := []Pane{
		{ID: "1", PID: 100},
		{ID: "2", PID: 200, Processes: []PaneProcess{{PID: 201, Cmdline: []string{"nvim"}}}},
		{ID: "3", PID: 300},
	}

	t.Run("match by shell PID", func(t *testing.T) {
		p := findPaneByPID(panes, 100)
		if p == nil || p.ID != "1" {
			t.Errorf("expected pane 1, got %v", p)
		}
	})

	t.Run("match by foreground process PID", func(t *testing.T) {
		p := findPaneByPID(panes, 201)
		if p == nil || p.ID != "2" {
			t.Errorf("expected pane 2, got %v", p)
		}
	})

	t.Run("no match", func(t *testing.T) {
		p := findPaneByPID(panes, 999)
		if p != nil {
			t.Errorf("expected nil, got pane %s", p.ID)
		}
	})
}

func TestPaneArea(t *testing.T) {
	p := Pane{Columns: 160, Lines: 48}
	if p.Area() != 7680 {
		t.Errorf("Area() = %d, want 7680", p.Area())
	}

	zero := Pane{Columns: 0, Lines: 0}
	if zero.Area() != 0 {
		t.Errorf("Area() = %d, want 0", zero.Area())
	}
}

func TestKittyTabToPanes(t *testing.T) {
	tab := kittyTab{
		ID: 1,
		Windows: []kittyWindow{
			{
				ID: 38, PID: 100, Columns: 188, Lines: 42,
				AtPrompt: false,
				ForegroundProcesses: []kittyForegroundProcess{
					{PID: 200, Cmdline: []string{"/opt/homebrew/bin/nvim", "main.go"}},
				},
				UserVars: kittyUserVars{NvimServer: "/tmp/nvim.sock"},
			},
			{
				ID: 61, PID: 300, Columns: 80, Lines: 24,
				AtPrompt: true,
				ForegroundProcesses: []kittyForegroundProcess{
					{PID: 301, Cmdline: []string{"/bin/zsh"}},
				},
			},
		},
	}

	panes := kittyTabToPanes(tab)

	if len(panes) != 2 {
		t.Fatalf("expected 2 panes, got %d", len(panes))
	}

	p0 := panes[0]
	if p0.ID != "38" {
		t.Errorf("pane 0 ID = %s, want 38", p0.ID)
	}
	if p0.NvimServer != "/tmp/nvim.sock" {
		t.Errorf("pane 0 NvimServer = %q, want /tmp/nvim.sock", p0.NvimServer)
	}
	if p0.AtPrompt {
		t.Error("pane 0 should not be at prompt")
	}
	if p0.Columns != 188 || p0.Lines != 42 {
		t.Errorf("pane 0 size = %dx%d, want 188x42", p0.Columns, p0.Lines)
	}
	if len(p0.Processes) != 1 || p0.Processes[0].PID != 200 {
		t.Errorf("pane 0 processes = %v, want [{200 [nvim main.go]}]", p0.Processes)
	}

	p1 := panes[1]
	if p1.ID != "61" {
		t.Errorf("pane 1 ID = %s, want 61", p1.ID)
	}
	if !p1.AtPrompt {
		t.Error("pane 1 should be at prompt")
	}
	if p1.NvimServer != "" {
		t.Errorf("pane 1 NvimServer = %q, want empty", p1.NvimServer)
	}
}

func TestKittyJSONParsing(t *testing.T) {
	// Realistic kitty @ ls JSON excerpt
	raw := `[{
		"tabs": [{
			"id": 1,
			"windows": [{
				"id": 38,
				"pid": 18022,
				"columns": 188,
				"lines": 42,
				"at_prompt": false,
				"foreground_processes": [
					{"pid": 200, "cmdline": ["/opt/homebrew/bin/nvim", "."]}
				],
				"user_vars": {"nvim_server": "/tmp/nvim.mike/abc/nvim.123.0"}
			}, {
				"id": 61,
				"pid": 18023,
				"columns": 80,
				"lines": 24,
				"at_prompt": true,
				"foreground_processes": [
					{"pid": 18023, "cmdline": ["/bin/zsh"]}
				],
				"user_vars": {}
			}]
		}]
	}]`

	var state []kittyOSWindow
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		t.Fatalf("JSON parse error: %v", err)
	}

	if len(state) != 1 || len(state[0].Tabs) != 1 {
		t.Fatalf("unexpected structure: %d os windows", len(state))
	}

	tab := state[0].Tabs[0]
	if len(tab.Windows) != 2 {
		t.Fatalf("expected 2 windows, got %d", len(tab.Windows))
	}

	w0 := tab.Windows[0]
	if w0.ID != 38 || w0.PID != 18022 || w0.AtPrompt {
		t.Errorf("window 0 parsed incorrectly: id=%d pid=%d at_prompt=%v", w0.ID, w0.PID, w0.AtPrompt)
	}
	if w0.UserVars.NvimServer != "/tmp/nvim.mike/abc/nvim.123.0" {
		t.Errorf("window 0 nvim_server = %q", w0.UserVars.NvimServer)
	}

	w1 := tab.Windows[1]
	if w1.ID != 61 || !w1.AtPrompt || w1.UserVars.NvimServer != "" {
		t.Errorf("window 1 parsed incorrectly: id=%d at_prompt=%v nvim_server=%q", w1.ID, w1.AtPrompt, w1.UserVars.NvimServer)
	}

	// Verify kittyTabToPanes propagates fields correctly
	panes := kittyTabToPanes(tab)
	if panes[0].NvimServer != "/tmp/nvim.mike/abc/nvim.123.0" {
		t.Error("NvimServer not propagated to Pane")
	}
	if !panes[1].AtPrompt {
		t.Error("AtPrompt not propagated to Pane")
	}
}

func TestKittyTabContainsPID(t *testing.T) {
	tab := kittyTab{
		Windows: []kittyWindow{
			{
				ID: 38, PID: 100,
				ForegroundProcesses: []kittyForegroundProcess{
					{PID: 200, Cmdline: []string{"nvim"}},
				},
			},
			{
				ID: 61, PID: 300,
				ForegroundProcesses: []kittyForegroundProcess{
					{PID: 301, Cmdline: []string{"zsh"}},
				},
			},
		},
	}

	// Use a real (empty) cache — ParentPID of non-existent PIDs returns 0,
	// so only direct PID matches will work.
	cache := process.NewCache()

	t.Run("match shell PID", func(t *testing.T) {
		if !kittyTabContainsPID(tab, 100, cache) {
			t.Error("expected tab to contain PID 100")
		}
	})

	t.Run("match foreground PID", func(t *testing.T) {
		if !kittyTabContainsPID(tab, 200, cache) {
			t.Error("expected tab to contain PID 200")
		}
	})

	t.Run("no match", func(t *testing.T) {
		if kittyTabContainsPID(tab, 999, cache) {
			t.Error("expected tab to not contain PID 999")
		}
	})
}
