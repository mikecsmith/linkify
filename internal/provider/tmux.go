package provider

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/mikecsmith/linkify/internal/process"
)

// TmuxProvider implements Provider for tmux.
type TmuxProvider struct {
	cache *process.Cache
	bin   string // cached binary path
}

func (t *TmuxProvider) Name() string { return "tmux" }

func (t *TmuxProvider) tmux(args ...string) *exec.Cmd {
	if t.bin == "" {
		if p, err := exec.LookPath("tmux"); err == nil {
			t.bin = p
		} else {
			t.bin = "tmux"
		}
	}
	return exec.Command(t.bin, args...)
}

func (t *TmuxProvider) FindPanesByPID(pid int) ([]Pane, error) {
	const sep = "|||"
	out, err := t.tmux("list-panes", "-a", "-F",
		"#{pane_id}"+sep+"#{pane_pid}"+sep+"#{pane_width}"+sep+"#{pane_height}"+sep+"#{pane_current_command}"+sep+"#{window_id}",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("tmux list-panes: %w", err)
	}

	ppid := t.cache.ParentPID(pid)
	ancestors := t.cache.AncestorSet(pid)
	LogOpen("tmux: looking for PID %d (ppid=%d)", pid, ppid)
	var targetWindowID string

	type tmuxPane struct {
		pane     Pane
		windowID string
	}
	var allPanes []tmuxPane

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.SplitN(line, sep, 6)
		if len(fields) < 6 {
			continue
		}

		paneID := fields[0]
		panePID, _ := strconv.Atoi(fields[1])
		cols, _ := strconv.Atoi(fields[2])
		lines, _ := strconv.Atoi(fields[3])
		cmd := fields[4]
		windowID := fields[5]

		p := tmuxPane{
			pane: Pane{
				ID:      paneID,
				PID:     panePID,
				Columns: cols,
				Lines:   lines,
				Processes: []PaneProcess{
					{PID: panePID, Cmdline: []string{cmd}},
				},
			},
			windowID: windowID,
		}
		allPanes = append(allPanes, p)

		if panePID == pid || panePID == ppid || ancestors[panePID] {
			targetWindowID = windowID
		}
	}

	if targetWindowID == "" {
		return nil, fmt.Errorf("PID %d not found in any tmux window", pid)
	}

	var result []Pane
	for _, p := range allPanes {
		if p.windowID == targetWindowID {
			result = append(result, p.pane)
		}
	}
	return result, nil
}

func (t *TmuxProvider) FocusPane(id string) error {
	return t.tmux("select-pane", "-t", id).Run()
}

func (t *TmuxProvider) LaunchInPane(panes []Pane, sourcePID int, cwd string, args []string) error {
	// tmux new-window spawns the process directly — no SendText needed.
	LogOpen("tmux: opening new window (cwd: %s)", cwd)
	return t.launchInNewWindow(cwd, args)
}

func (t *TmuxProvider) launchInNewWindow(cwd string, args []string) error {
	full := append([]string{"new-window", "-c", cwd, "--"}, args...)
	return t.tmux(full...).Run()
}
