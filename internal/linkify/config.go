package linkify

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"

	"github.com/mikecsmith/linkify/internal/matcher"
)

// Config defines the linkify configuration loaded from YAML.
type Config struct {
	URLTemplate     string              `yaml:"url_template"`
	EditorPath      string              `yaml:"editor_path"`
	ExtraExtensions []string            `yaml:"extra_extensions"`
	Matchers        []matcher.ConfigDef `yaml:"matchers"`
}

// DefaultConfig is used when no config file is found.
var DefaultConfig = Config{
	URLTemplate: "lfy://open?file={file}&line={line}&col={col}&pid={pid}",
}

// LoadConfig reads ~/.config/linkify/config.yaml and returns the config,
// falling back to DefaultConfig on any error.
func LoadConfig() Config {
	cfg := DefaultConfig

	home, err := os.UserHomeDir()
	if err != nil {
		return cfg
	}

	configPath := filepath.Join(home, ".config", "linkify", "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return cfg
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "linkify: invalid config %s: %v\n", configPath, err)
		return DefaultConfig
	}

	for _, ext := range cfg.ExtraExtensions {
		RegisterExtension(ext)
	}

	return cfg
}
