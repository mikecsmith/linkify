package provider

import "fmt"

// DefaultProvider is the fallback when no terminal-specific provider is available.
// It can only find nvim via socket discovery — no pane awareness.
type DefaultProvider struct{}

func (d *DefaultProvider) Name() string { return "default" }

func (d *DefaultProvider) FindPanesByPID(pid int) ([]Pane, error) {
	return nil, fmt.Errorf("default provider has no pane awareness")
}

func (d *DefaultProvider) FocusPane(id string) error {
	return nil // no-op
}

func (d *DefaultProvider) LaunchInPane(panes []Pane, sourcePID int, cwd string, args []string) error {
	return fmt.Errorf("default provider has no pane awareness")
}
