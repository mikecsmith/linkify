package provider

import (
	"github.com/mikecsmith/linkify/internal/process"
)

// Pane represents a terminal pane/window with its properties.
type Pane struct {
	ID      string // terminal-specific identifier
	PID     int    // shell PID
	Columns int
	Lines   int
	// Foreground processes running in this pane
	Processes []PaneProcess
	// Terminal-specific rich data (populated when available)
	AtPrompt   bool   // shell is at a prompt (kitty)
	NvimServer string // nvim RPC socket path (kitty user_vars)
}

// PaneProcess is a process running in a terminal pane.
type PaneProcess struct {
	PID     int
	Cmdline []string
}

// Area returns the pane's area in cells.
func (p Pane) Area() int {
	return p.Columns * p.Lines
}

// Provider abstracts terminal-specific operations.
// Implementations: kitty, wezterm, tmux, default (fallback).
type Provider interface {
	// Name returns the provider name for logging.
	Name() string

	// FindPanesByPID finds the group of sibling panes (tab/window)
	// that contains the given PID. Returns all panes in that group.
	FindPanesByPID(pid int) ([]Pane, error)

	// FocusPane brings the given pane to the foreground.
	FocusPane(id string) error

	// LaunchInPane picks the best pane from the given set and launches
	// the command there. The sourcePID identifies the pane where the
	// link was clicked (used as fallback target). cwd is the working
	// directory for the launched process. Terminal-specific
	// implementations decide the strategy (new tab, send text, etc.).
	LaunchInPane(panes []Pane, sourcePID int, cwd string, args []string) error
}

func findPaneByPID(panes []Pane, pid int) *Pane {
	for i := range panes {
		p := &panes[i]
		if p.PID == pid {
			return p
		}
		for _, proc := range p.Processes {
			if proc.PID == pid {
				return p
			}
		}
	}
	return nil
}

// Detect walks the process tree from the source PID upward
// to find which terminal emulator owns it. Falls back to default.
func Detect(pid int, cache *process.Cache) Provider {
	if pid > 0 {
		terminal := findTerminalInProcessTree(pid, cache)
		LogOpen("process tree for PID %d → terminal: %q", pid, terminal)

		switch terminal {
		case "kitty":
			LogOpen("using terminal provider: kitty")
			return &KittyProvider{cache: cache}
		case "tmux":
			LogOpen("using terminal provider: tmux")
			return &TmuxProvider{cache: cache}
		case "wezterm-gui":
			LogOpen("using terminal provider: wezterm")
			return &WeztermProvider{}
		}
	}

	LogOpen("using default provider (terminal not detected or provider unavailable)")
	return &DefaultProvider{}
}

// findTerminalInProcessTree walks parent PIDs looking for a known terminal.
func findTerminalInProcessTree(pid int, cache *process.Cache) string {
	// First match wins — tmux appears before kitty/wezterm in the tree,
	// so it takes priority when running inside a multiplexer.
	knownTerminals := map[string]string{
		"tmux: server": "tmux",
		"tmux: client": "tmux",
		"tmux":         "tmux",
		"kitty":        "kitty",
		"wezterm-gui":  "wezterm-gui",
		"iTerm2":       "iterm2",
		"Terminal":     "apple-terminal",
		"Alacritty":    "alacritty",
		"ghostty":      "ghostty",
	}

	current := pid
	for i := 0; i < 20; i++ { // cap depth to avoid infinite loops
		name, ppid := cache.Info(current)
		if name == "" || current <= 1 {
			break
		}
		if terminal, ok := knownTerminals[name]; ok {
			return terminal
		}
		current = ppid
	}
	return ""
}
