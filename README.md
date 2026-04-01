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

lfy has built-in support for Neovim. When you click a link, lfy finds the right terminal pane, locates the running nvim instance via its RPC socket, and opens the file at the correct line — no new window, no context switch.

```bash
lfy service install   # register lfy:// scheme + start background service
```

This registers the `lfy://` URL scheme by building a lightweight macOS app (or Linux `.desktop` file) and starts a background service to keep it resident. Because the handler stays running, every click is instant. The opener walks the process tree to detect your terminal (kitty, tmux, wezterm), finds sibling panes, and prefers opening in an existing nvim instance. If no nvim is running, it launches one in the largest available pane.

### GUI editors (zero setup)

Editors with native URL scheme handlers work out of the box — just set `url_template` in your config:

```yaml
# VS Code
url_template: "vscode://file/{file}:{line}:{col}"

# Zed
url_template: "zed://file/{file}:{line}:{col}"

# JetBrains (GoLand, WebStorm, IntelliJ, etc.)
url_template: "jetbrains://open?file={file}&line={line}&column={col}"

# Sublime Text
url_template: "subl://open?url=file://{file}&line={line}&column={col}"
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
3. **Open** (nvim) — `lfy open` walks the process tree to detect your terminal, finds the right pane, and opens the file in an existing nvim via RPC or launches one in the largest pane

## Terminal providers

When using the `lfy://` scheme with nvim, linkify detects your terminal from the process tree and uses a provider to interact with panes.

| Provider | Detection | Pane discovery | How it opens files |
|---|---|---|---|
| **kitty** | `kitty` in process tree | `kitty @ ls` — full pane/PID/process info | `kitty @ send-text` or nvim `--remote-send` |
| **tmux** | `tmux` in process tree (takes priority) | `tmux list-panes -a` — PID + process walking | `tmux send-keys` or nvim `--remote-send` |
| **wezterm** | `wezterm-gui` in process tree | `wezterm cli list` — title-based nvim detection | `wezterm cli send-text` or nvim `--remote-send` |
| **default** | Fallback | No pane awareness | Finds newest nvim socket in `$TMPDIR`, or launches new instance |

Kitty requires `allow_remote_control` and `listen_on` in your kitty config. Tmux takes priority over the outer terminal when running inside a multiplexer.

## OSC8 terminal support

| Terminal | OSC8 links |
|---|---|
| kitty | Yes |
| tmux 3.4+ | Yes |
| wezterm | Yes |
| iTerm2 | Yes |
| Ghostty | Yes |
| Alacritty 0.15+ | Yes |
| macOS Terminal | No |

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
