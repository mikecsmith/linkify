package linkify

import (
	"path/filepath"
	"strings"
)

// knownExtensions is the set of file extensions recognised as source files.
// Use RegisterExtension to add entries at runtime.
var knownExtensions = map[string]bool{
	".go": true, ".ts": true, ".tsx": true, ".js": true, ".jsx": true,
	".py": true, ".rs": true, ".lua": true, ".rb": true, ".java": true,
	".kt": true, ".c": true, ".cpp": true, ".cc": true, ".h": true,
	".hpp": true, ".cs": true, ".swift": true, ".zig": true, ".ex": true,
	".exs": true, ".erl": true, ".hs": true, ".ml": true, ".mli": true,
	".vue": true, ".svelte": true, ".yaml": true, ".yml": true,
	".json": true, ".toml": true, ".xml": true, ".html": true,
	".css": true, ".scss": true, ".sass": true, ".less": true,
	".sh": true, ".bash": true, ".zsh": true, ".fish": true,
	".sql": true, ".graphql": true, ".gql": true, ".proto": true,
	".tf": true, ".hcl": true, ".nix": true, ".vim": true,
	".el": true, ".clj": true, ".cljs": true, ".dart": true,
	".r": true, ".R": true, ".jl": true, ".php": true, ".pl": true,
	".pm": true, ".t": true, ".m": true, ".mm": true,
	".mod": true, ".sum": true, ".txt": true, ".md": true,
	".mk": true, ".cmake": true,
}

// RegisterExtension adds a file extension to the known set.
func RegisterExtension(ext string) {
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	knownExtensions[ext] = true
}

// LooksLikeFile returns true if the path looks like a source file.
func LooksLikeFile(path string) bool {
	ext := filepath.Ext(path)
	if ext == "" {
		base := filepath.Base(path)
		return base == "Makefile" || base == "Dockerfile" || base == "Taskfile"
	}
	return knownExtensions[ext]
}
