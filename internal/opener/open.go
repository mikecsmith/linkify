package opener

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mikecsmith/linkify/internal/logutil"
	"github.com/mikecsmith/linkify/internal/process"
	"github.com/mikecsmith/linkify/internal/provider"
)

// Open handles a lfy:// URL by finding the right terminal pane and opening the file.
// editorPath overrides nvim discovery if non-empty.
func Open(rawURL string, editorPath string) error {
	logutil.Log("handling URL: %s", rawURL)

	u, err := url.Parse(rawURL)
	if err != nil {
		logutil.Log("bad URL: %v", err)
		return fmt.Errorf("bad URL: %w", err)
	}

	params := u.Query()
	file := params.Get("file")
	line := params.Get("line")
	col := params.Get("col")
	sourcePID := params.Get("pid")
	if line == "" {
		line = "1"
	}
	if col == "" {
		col = "1"
	}

	if file == "" {
		logutil.Log("no file parameter")
		return fmt.Errorf("no file parameter in URL")
	}

	if resolved, err := filepath.EvalSymlinks(file); err == nil {
		file = resolved
	}

	nvim := findNvim(editorPath)

	// Use source PID to detect terminal and find sibling panes
	if sourcePID != "" {
		pid := 0
		_, _ = fmt.Sscanf(sourcePID, "%d", &pid)

		if pid > 0 {
			cache := process.NewCache()
			prov := provider.Detect(pid, cache)
			if panes, err := prov.FindPanesByPID(pid); err == nil {
				if openInPane(prov, panes, nvim, file, line, col, pid) {
					return nil
				}
			} else {
				logutil.Log("FindPanesByPID(%d): %v", pid, err)
			}
		}
	}

	// Fallback: find most recently active nvim socket
	if sock := findNewestNvimSocket(nvim); sock != "" {
		logutil.Log("fallback: found nvim socket %s", sock)
		sendToNvim(nvim, sock, file, line, col)
		return nil
	}

	// Last resort: launch nvim directly
	logutil.Log("no running nvim, launching new instance")
	cmd := exec.Command(nvim, "+"+line, file)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// openInPane tries to open a file in an existing nvim instance, or delegates
// to the provider to launch nvim in the best available pane/tab.
// Returns true if the file was handled, false to fall through to other methods.
func openInPane(prov provider.Provider, panes []provider.Pane, nvim, file, line, col string, sourcePID int) bool {
	if !isNumeric(line) {
		line = "1"
	}

	// First: check for panes with a known nvim server address
	for i := range panes {
		p := &panes[i]
		if p.NvimServer != "" && isSocketAlive(nvim, p.NvimServer) {
			logutil.Log("found nvim_server in pane %s: %s", p.ID, p.NvimServer)
			sendToNvim(nvim, p.NvimServer, file, line, col)
			_ = prov.FocusPane(p.ID)
			return true
		}
	}

	// Second: look for nvim via process detection + socket scan
	var bestNvim *provider.Pane
	for i := range panes {
		p := &panes[i]
		if paneHasNvim(p) && (bestNvim == nil || p.Area() > bestNvim.Area()) {
			bestNvim = p
		}
	}

	if bestNvim != nil {
		server := getNvimServerAddr(nvim, paneNvimPID(bestNvim))
		if server != "" && isSocketAlive(nvim, server) {
			logutil.Log("sending to nvim in pane %s (%dx%d)", bestNvim.ID, bestNvim.Columns, bestNvim.Lines)
			sendToNvim(nvim, server, file, line, col)
			_ = prov.FocusPane(bestNvim.ID)
			return true
		}
		logutil.Log("nvim in pane %s has dead socket", bestNvim.ID)
	}

	// No live nvim — let the provider decide where to launch.
	cwd := findProjectRoot(file)
	err := prov.LaunchInPane(panes, sourcePID, cwd, []string{nvim, "+" + line, file})
	if err == nil {
		return true
	}

	logutil.Log("LaunchInPane failed: %v", err)
	return false
}

// --- helpers ---

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func isNvimCmd(cmdline []string) bool {
	if len(cmdline) == 0 {
		return false
	}
	base := filepath.Base(cmdline[0])
	return base == "nvim" || base == "vim"
}

func paneHasNvim(p *provider.Pane) bool {
	for _, proc := range p.Processes {
		if isNvimCmd(proc.Cmdline) {
			return true
		}
	}
	return false
}

func paneNvimPID(p *provider.Pane) int {
	for _, proc := range p.Processes {
		if isNvimCmd(proc.Cmdline) {
			return proc.PID
		}
	}
	return 0
}

// findProjectRoot walks up from the file's directory looking for common
// project root markers. Falls back to the file's directory.
func findProjectRoot(file string) string {
	dir := filepath.Dir(file)
	markers := []string{
		// VCS
		".git", ".hg", ".svn",
		// Go
		"go.mod",
		// JavaScript / TypeScript
		"package.json", "deno.json", "deno.jsonc",
		// Rust
		"Cargo.toml",
		// Python
		"pyproject.toml", "setup.py", "setup.cfg",
		// Java / JVM
		"pom.xml", "build.gradle", "build.gradle.kts",
		// C / C++
		"CMakeLists.txt", "meson.build",
		// Ruby
		"Gemfile",
		// PHP
		"composer.json",
		// .NET
		"*.sln", "*.csproj",
		// Elixir
		"mix.exs",
		// Zig
		"build.zig",
		// Swift
		"Package.swift",
		// Scala
		"build.sbt",
		// General
		"Makefile", ".editorconfig",
	}

	current := dir
	for {
		for _, m := range markers {
			if strings.ContainsAny(m, "*?") {
				if matches, _ := filepath.Glob(filepath.Join(current, m)); len(matches) > 0 {
					return current
				}
			} else if _, err := os.Stat(filepath.Join(current, m)); err == nil {
				return current
			}
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return dir
}

func findNvim(override string) string {
	if override != "" {
		return override
	}
	// Check well-known locations — macOS app context has minimal PATH
	candidates := []string{
		filepath.Join(os.Getenv("HOME"), ".local", "bin", "nvim"),
		"/opt/homebrew/bin/nvim",
		"/usr/local/bin/nvim",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if p, err := exec.LookPath("nvim"); err == nil {
		return p
	}
	return "nvim"
}

func sendToNvim(nvim, server, file, line, col string) {
	if !isNumeric(line) {
		line = "1"
	}
	if !isNumeric(col) {
		col = "1"
	}
	// Use --remote-expr to safely open the file and navigate.
	// execute() runs ex commands; we chain them with \n separators
	// inside a single execute() call since | is not valid in expressions.
	expr := fmt.Sprintf(
		`execute("edit " . fnameescape('%s') . "\n call cursor(%s,%s) \n normal! zz")`,
		vimEscapeString(file), line, col,
	)
	out, err := exec.Command(nvim, "--server", server, "--remote-expr", expr).CombinedOutput()
	if err != nil {
		logutil.Log("remote-expr failed: %v: %s", err, out)
	}
}

// vimEscapeString escapes a string for use inside a vim single-quoted string literal.
// In vim, the only special character in single-quoted strings is ' itself,
// which is escaped by ending the string, adding an escaped quote, and reopening: 'foo''bar' → foo'bar
func vimEscapeString(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

func isSocketAlive(nvim, path string) bool {
	out, err := exec.Command(nvim, "--server", path, "--remote-expr", "1").CombinedOutput()
	if err != nil {
		logutil.Log("socket %s is dead: %v: %s", path, err, out)
		return false
	}
	return true
}

// getNvimServerAddr finds the nvim RPC socket for a given nvim PID.
// nvim forks after launch, so the PID seen by the terminal (parent) differs
// from the PID that owns the socket (child). We query each socket and check
// whether the nvim process is a descendant of the expected PID.
func getNvimServerAddr(nvim string, nvimPID int) string {
	if nvimPID == 0 {
		return ""
	}

	tmpDir := os.TempDir()

	// Collect all nvim sockets
	var sockets []string
	entries, _ := os.ReadDir(tmpDir)
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "nvim.") {
			continue
		}
		_ = filepath.WalkDir(filepath.Join(tmpDir, e.Name()), func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			if info.Mode()&os.ModeSocket != 0 {
				sockets = append(sockets, path)
			}
			return nil
		})
	}

	// Query each socket — accept if the nvim PID is the expected PID
	// or a child of it (nvim forks, so the socket PID is a child of
	// the PID the terminal reports).
	for _, sock := range sockets {
		out, err := exec.Command(nvim, "--server", sock, "--remote-expr", "getpid()").Output()
		if err != nil {
			continue
		}
		sockPID := 0
		_, _ = fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &sockPID)
		if sockPID == 0 {
			continue
		}
		if sockPID == nvimPID || process.ParentPID(sockPID) == nvimPID {
			logutil.Log("found nvim socket %s (nvim pid %d, expected %d)", sock, sockPID, nvimPID)
			return sock
		}
	}

	return ""
}

func findNewestNvimSocket(nvim string) string {
	tmpDirs := []string{os.TempDir(), "/tmp"}

	var best string
	var bestTime time.Time

	for _, tmpDir := range tmpDirs {
		entries, err := os.ReadDir(tmpDir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() || !strings.HasPrefix(e.Name(), "nvim.") {
				continue
			}
			_ = filepath.WalkDir(filepath.Join(tmpDir, e.Name()), func(path string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}
				info, err := d.Info()
				if err != nil {
					return nil
				}
				if info.Mode()&os.ModeSocket == 0 {
					return nil
				}
				if info.ModTime().After(bestTime) && isSocketAlive(nvim, path) {
					best = path
					bestTime = info.ModTime()
				}
				return nil
			})
		}
	}
	return best
}
