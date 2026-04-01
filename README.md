# linkify

Pipe test output through `lfy` to make file:line references clickable in your terminal ([OSC8 hyperlinks](https://gist.github.com/egmontkob/eb114294efbcd5adb1944c9f3cb5feda)). Click to jump straight to the failing line in your editor.

```bash
go test ./... 2>&1 | lfy
pnpm test 2>&1 | lfy
cargo test 2>&1 | lfy
```

Works with `go test`, Jest, Vitest, Mocha, pytest, `cargo test`, `tsc`, and anything that prints `file:line` references. Understands Go package paths and resolves test names to source locations.

## Editor support

### Neovim (first-class)

lfy has built-in support for Neovim. When you click a link, lfy finds the right terminal pane, locates the running nvim instance via its RPC socket, and opens the file at the correct line and column — no new window, no context switch.

```bash
lfy service install   # register lfy:// scheme + start background service
```

This registers the `lfy://` URL scheme by building a lightweight macOS app (or Linux `.desktop` file) and starts a background service to keep it resident. Because the handler stays running, every click is instant.

The opener walks the process tree to detect your terminal (kitty, tmux, wezterm), finds sibling panes, and prefers opening in an existing nvim instance via `--remote-expr`. If no nvim is running, it launches one — the strategy depends on your terminal (see [Terminal providers](#terminal-providers) below).

### GUI editors (zero setup)

Editors with native URL scheme handlers should work out of the box but are untested — just set `url_template` in your config:

```yaml
# VS Code
url_template: "vscode://file/{file}:{line}:{col}"

# Zed
url_template: "zed://file/{file}:{line}:{col}"

# JetBrains (GoLand, WebStorm, IntelliJ, etc.)
url_template: "jetbrains://open?file={file}&line={line}&column={col}"

```

No service needed. The OS routes clicks directly to the editor.

### Not yet supported

**Emacs**, **Helix**, **Kakoune**, and other terminal editors don't have built-in URL scheme handlers and aren't supported by lfy's opener yet. Contributions welcome — the opener architecture is modular (see `internal/opener/`).

**Vim** has a client-server mode (`+clientserver`) that's similar to nvim's RPC, but most vim builds ship without it. If your vim has `+clientserver` it may work, but this isn't tested.

## Install

### Go install

```bash
go install github.com/mikecsmith/linkify/cmd/lfy@latest
lfy service install   # optional: register URL handler for nvim
```

### From source

```bash
git clone https://github.com/mikecsmith/linkify.git
cd linkify
make install
lfy service install   # optional: register URL handler for nvim
```

> **Self-contained binary:** `lfy service install` works from any install method — no source checkout needed. The macOS URL handler (Swift) and Linux `.desktop` file are embedded in the binary. On macOS, the handler is compiled on the fly using `swiftc` (ships with Xcode Command Line Tools).

## Configuration

```bash
~/.config/linkify/config.yaml
```

See [`config.example.yaml`](config.example.yaml) for all options including editor presets, custom matchers, and `editor_path`.

### Custom matchers

Add patterns for tools lfy doesn't know about:

```yaml
matchers:
  - name: webpack
    pattern: 'ERROR in (?P<file>[^\s(]+)\((?P<line>\d+),(?P<col>\d+)\)'
  - name: bazel
    pattern: '(?P<file>[^\s:]+):(?P<line>\d+):(?P<col>\d+): error'
```

Config matchers run before built-in patterns. Use named capture groups: `(?P<file>...)`, `(?P<line>...)`, `(?P<col>...)`.

## CLI

```
lfy                          Pipe filter (default) — read stdin, write linkified output
lfy --dry-run                Show matched URLs as plain text
lfy --url TMPL               Override url_template for this invocation
lfy service install          Register lfy:// URL scheme and start background service
lfy service uninstall        Remove URL handler and stop service
lfy service build            Build the URL handler without installing
lfy service start / stop     Manage the background service (macOS)
lfy service status           Check if the service is running
```

## How it works

1. **Pipe filter** — reads stdin line by line, matches file:line references, wraps them in OSC8 hyperlinks using your configured URL template
2. **Click** — your terminal opens the URL. GUI editors handle their schemes natively. For nvim, the `lfy://` scheme is handled by a registered URL handler that calls `lfy open`
3. **Open** (nvim) — `lfy open` walks the process tree to detect your terminal, finds the right pane, and either sends the file to an existing nvim via `--remote-expr` or launches a new instance. The working directory is set to the project root (detected by walking up from the file to find `.git`, `go.mod`, `package.json`, etc.)

## Terminal providers

When using the `lfy://` scheme with nvim, linkify detects your terminal from the process tree and uses a provider to interact with panes. Each provider has different capabilities:

### Kitty

**Detection:** `kitty` in process tree | **Discovery:** `kitty @ ls` (full pane state including PIDs, processes, prompt status, user vars)

Kitty has the richest integration. The opener checks all panes for:

1. **Known nvim server** — if a pane has `nvim_server` set via kitty's `user_vars`, the file is sent directly to that nvim instance via `--remote-expr`, and the pane is focused
2. **Process detection** — scans panes for nvim processes and resolves their RPC sockets
3. **Launch strategy** — if no nvim is running, picks the largest non-source pane:
   - Pane is at a shell prompt → launches nvim as an **overlay** on that pane
   - Pane is busy → opens a **new tab**
   - Only one pane (source) → overlays on the source pane if at prompt, otherwise new tab

Environment variables and working directory are inherited from the source pane (`--copy-env`, `--cwd`).

**Requirements:** `allow_remote_control` and `listen_on` in your kitty config.

### tmux

**Detection:** `tmux` in process tree (takes priority over the outer terminal) | **Discovery:** `tmux list-panes -a` (PIDs + ancestor matching)

tmux matches the source PID to a pane via process ancestry, then:

1. Checks for existing nvim instances (process detection + socket scan)
2. If no nvim is found, opens a **new window** with the correct working directory (`tmux new-window -c <cwd>`)

**Requirements:** tmux 3.4+ for OSC8 hyperlink support. Add to your `~/.tmux.conf`:

```tmux
set -ga terminal-features ",*:hyperlinks"
set -g allow-passthrough on
```

### WezTerm

**Detection:** `wezterm-gui` in process tree | **Discovery:** `wezterm cli list` (title-based process detection)

WezTerm has limited process visibility (no PIDs in `cli list`), so nvim detection relies on pane titles. When no nvim is found:

1. Opens a **new tab** via `wezterm cli spawn`

### Default (fallback)

When no terminal is detected, or the provider can't find panes:

1. Scans `$TMPDIR` for the most recently active nvim socket and sends the file via `--remote-expr`
2. If no nvim socket is found, launches nvim directly in the current terminal

## OSC8 terminal support

| Terminal        | OSC8 links            |
| --------------- | --------------------- |
| kitty           | Yes                   |
| tmux 3.4+       | Yes (requires config) |
| wezterm         | Yes                   |
| iTerm2          | Untested              |
| Ghostty         | Untested              |
| Alacritty 0.15+ | Untested              |
| macOS Terminal  | No                    |

## Development

```bash
make test                                          # run all tests
go test ./internal/linkify -run TestGolden -update  # regenerate golden files
make install                                       # build + install binary
lfy service install                                # register URL handler
```

To add a test case for a new tool, drop a `.input` file in `internal/linkify/testdata/`, run with `-update`, review the `.expected` file, and commit both.

## License

MIT
