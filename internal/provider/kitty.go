package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/mikecsmith/linkify/internal/process"
)

// KittyProvider implements Provider for the Kitty terminal.
type KittyProvider struct {
	sock  string // cached socket path
	bin   string // cached kitty binary path
	cache *process.Cache
}

func (k *KittyProvider) Name() string { return "kitty" }

func (k *KittyProvider) cmd(args ...string) *exec.Cmd {
	if k.sock == "" {
		k.sock = findKittySock()
	}
	if k.bin == "" {
		k.bin = findKittyBin()
	}
	full := append([]string{"@", "--to", k.sock}, args...)
	return exec.Command(k.bin, full...)
}

// findKittyBin locates the kitty binary. The macOS URL handler app runs
// with a minimal PATH, so we check well-known locations first.
func findKittyBin() string {
	candidates := []string{
		"/Applications/kitty.app/Contents/MacOS/kitty",
		filepath.Join(os.Getenv("HOME"), ".local/kitty.app/bin/kitty"),
		"/opt/homebrew/bin/kitty",
		"/usr/local/bin/kitty",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if p, err := exec.LookPath("kitty"); err == nil {
		return p
	}
	return "kitty"
}

func (k *KittyProvider) FindPanesByPID(pid int) ([]Pane, error) {
	out, err := k.cmd("ls").Output()
	if err != nil {
		return nil, fmt.Errorf("kitty @ ls: %w", err)
	}

	var state []kittyOSWindow
	if err := json.Unmarshal(out, &state); err != nil {
		return nil, fmt.Errorf("kitty JSON: %w", err)
	}

	// Find the tab containing this PID, return all panes in that tab
	for _, osWin := range state {
		for _, tab := range osWin.Tabs {
			if kittyTabContainsPID(tab, pid, k.cache) {
				return kittyTabToPanes(tab), nil
			}
		}
	}

	return nil, fmt.Errorf("PID %d not found in any kitty tab", pid)
}

func (k *KittyProvider) FocusPane(id string) error {
	return k.cmd("focus-window", "--match", "id:"+id).Run()
}

func (k *KittyProvider) LaunchInPane(panes []Pane, sourcePID int, cwd string, args []string) error {
	sourcePane := findPaneByPID(panes, sourcePID)

	// Find the largest non-source pane.
	var target *Pane
	for i := range panes {
		p := &panes[i]
		if sourcePane != nil && p.ID == sourcePane.ID {
			continue
		}
		if target == nil || p.Area() > target.Area() {
			target = p
		}
	}

	if target != nil {
		LogOpen("kitty: target pane %s (%dx%d) at_prompt=%v",
			target.ID, target.Columns, target.Lines, target.AtPrompt)

		// Pane is at a shell prompt — overlay nvim on top of it
		if target.AtPrompt {
			LogOpen("kitty: pane %s at prompt, launching overlay", target.ID)
			return k.launchOverlay(target.ID, cwd, args)
		}

		// Pane has something else running — open new tab
		LogOpen("kitty: pane %s busy, opening new tab", target.ID)
		return k.launchInNewTab(cwd, args)
	}

	// Only one pane (the source) — overlay or new tab
	if sourcePane != nil && sourcePane.AtPrompt {
		LogOpen("kitty: only source pane, launching overlay on %s", sourcePane.ID)
		return k.launchOverlay(sourcePane.ID, cwd, args)
	}

	LogOpen("kitty: opening new tab")
	return k.launchInNewTab(cwd, args)
}

func (k *KittyProvider) launchOverlay(paneID, cwd string, args []string) error {
	// Focus the target pane first — kitty overlays land on the active window,
	// and --match on launch only selects the tab, not the window.
	if err := k.cmd("focus-window", "--match", "id:"+paneID).Run(); err != nil {
		LogOpen("kitty: focus-window %s failed: %v", paneID, err)
	}

	full := append([]string{"launch", "--type=overlay", "--copy-env", "--cwd=" + cwd}, args...)
	cmd := k.cmd(full...)
	LogOpen("kitty: running: %v", cmd.Args)
	out, err := cmd.CombinedOutput()
	if err != nil {
		LogOpen("kitty: launch overlay failed: %v: %s", err, out)
	}
	return err
}

func (k *KittyProvider) launchInNewTab(cwd string, args []string) error {
	full := append([]string{"launch", "--type=tab", "--copy-env", "--cwd=" + cwd}, args...)
	cmd := k.cmd(full...)
	LogOpen("kitty: running: %v", cmd.Args)
	out, err := cmd.CombinedOutput()
	if err != nil {
		LogOpen("kitty: launch tab failed: %v: %s", err, out)
	}
	return err
}

// --- kitty JSON structures ---

type kittyOSWindow struct {
	Tabs []kittyTab `json:"tabs"`
}

type kittyTab struct {
	ID      int           `json:"id"`
	Windows []kittyWindow `json:"windows"`
}

type kittyWindow struct {
	ID                  int                      `json:"id"`
	PID                 int                      `json:"pid"`
	Columns             int                      `json:"columns"`
	Lines               int                      `json:"lines"`
	AtPrompt            bool                     `json:"at_prompt"`
	ForegroundProcesses []kittyForegroundProcess `json:"foreground_processes"`
	UserVars            kittyUserVars            `json:"user_vars"`
}

type kittyForegroundProcess struct {
	PID     int      `json:"pid"`
	Cmdline []string `json:"cmdline"`
}

type kittyUserVars struct {
	NvimServer string `json:"nvim_server"`
}

// --- helpers ---

func kittyTabContainsPID(tab kittyTab, pid int, cache *process.Cache) bool {
	ppid := cache.ParentPID(pid)

	for _, win := range tab.Windows {
		if win.PID == pid || win.PID == ppid {
			return true
		}
		for _, fp := range win.ForegroundProcesses {
			if fp.PID == pid || fp.PID == ppid {
				return true
			}
		}
	}
	return false
}

func kittyTabToPanes(tab kittyTab) []Pane {
	panes := make([]Pane, 0, len(tab.Windows))
	for _, win := range tab.Windows {
		p := Pane{
			ID:         fmt.Sprintf("%d", win.ID),
			PID:        win.PID,
			Columns:    win.Columns,
			Lines:      win.Lines,
			AtPrompt:   win.AtPrompt,
			NvimServer: win.UserVars.NvimServer,
		}
		for _, fp := range win.ForegroundProcesses {
			p.Processes = append(p.Processes, PaneProcess(fp))
		}
		panes = append(panes, p)
	}
	return panes
}

func findKittySock() string {
	if v := os.Getenv("KITTY_LISTEN_ON"); v != "" {
		return v
	}
	matches, _ := filepath.Glob("/tmp/mykitty-*")
	if len(matches) == 0 {
		return ""
	}
	best := matches[0]
	for _, m := range matches[1:] {
		if info, err := os.Stat(m); err == nil {
			if bestInfo, err2 := os.Stat(best); err2 == nil && info.ModTime().After(bestInfo.ModTime()) {
				best = m
			}
		}
	}
	return "unix:" + best
}
