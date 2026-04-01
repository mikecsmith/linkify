package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

// WeztermProvider implements Provider for WezTerm.
type WeztermProvider struct {
	bin string // cached binary path
}

func (w *WeztermProvider) Name() string { return "wezterm" }

func (w *WeztermProvider) command(args ...string) *exec.Cmd {
	if w.bin == "" {
		w.bin = findWeztermBin()
	}
	return exec.Command(w.bin, args...)
}

// findWeztermBin locates the wezterm binary. The macOS URL handler app
// runs with a minimal PATH, so we check well-known locations first.
func findWeztermBin() string {
	candidates := []string{
		"/Applications/WezTerm.app/Contents/MacOS/wezterm",
		filepath.Join(os.Getenv("HOME"), ".local/bin/wezterm"),
		"/opt/homebrew/bin/wezterm",
		"/usr/local/bin/wezterm",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if p, err := exec.LookPath("wezterm"); err == nil {
		return p
	}
	return "wezterm"
}

// weztermPaneInfo is the JSON structure from `wezterm cli list --format json`.
type weztermPaneInfo struct {
	WindowID int         `json:"window_id"`
	TabID    int         `json:"tab_id"`
	PaneID   int         `json:"pane_id"`
	Title    string      `json:"title"`
	Cwd      string      `json:"cwd"`
	Size     weztermSize `json:"size"`
}

type weztermSize struct {
	Rows int `json:"rows"`
	Cols int `json:"cols"`
}

func (w *WeztermProvider) FindPanesByPID(pid int) ([]Pane, error) {
	out, err := w.command("cli", "list", "--format", "json").Output()
	if err != nil {
		return nil, fmt.Errorf("wezterm cli list: %w", err)
	}

	var panes []weztermPaneInfo
	if err := json.Unmarshal(out, &panes); err != nil {
		return nil, fmt.Errorf("wezterm JSON: %w", err)
	}

	if len(panes) == 0 {
		return nil, fmt.Errorf("no wezterm panes found")
	}

	// Group panes by tab
	tabPanes := make(map[int][]weztermPaneInfo)
	for _, p := range panes {
		tabPanes[p.TabID] = append(tabPanes[p.TabID], p)
	}

	// WezTerm doesn't expose PIDs in cli list output.
	// Return panes from the first tab as best effort.
	var activeTabID int
	for _, p := range panes {
		activeTabID = p.TabID
		break
	}

	var result []Pane
	for _, p := range tabPanes[activeTabID] {
		result = append(result, Pane{
			ID:      strconv.Itoa(p.PaneID),
			PID:     0, // WezTerm doesn't expose this
			Columns: p.Size.Cols,
			Lines:   p.Size.Rows,
			// Title often contains the command name (e.g. "nvim .")
			Processes: weztermTitleToProcesses(p.Title),
		})
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no panes in active wezterm tab")
	}
	return result, nil
}

func (w *WeztermProvider) FocusPane(id string) error {
	return w.command("cli", "activate-pane", "--pane-id", id).Run()
}

func (w *WeztermProvider) LaunchInPane(panes []Pane, sourcePID int, cwd string, args []string) error {
	// WezTerm can spawn directly in a new tab — no need for SendText.
	LogOpen("wezterm: opening new tab (cwd: %s)", cwd)
	return w.launchInNewTab(cwd, args)
}

func (w *WeztermProvider) launchInNewTab(cwd string, args []string) error {
	full := append([]string{"cli", "spawn", "--cwd", cwd, "--"}, args...)
	return w.command(full...).Run()
}

func weztermTitleToProcesses(title string) []PaneProcess {
	if title == "" {
		return nil
	}
	return []PaneProcess{
		{PID: 0, Cmdline: []string{title}},
	}
}
