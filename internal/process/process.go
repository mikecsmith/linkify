package process

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type processInfo struct {
	name string
	ppid int
}

// Cache memoizes process info lookups for the lifetime of a single operation.
type Cache struct {
	entries map[int]processInfo
}

// NewCache creates a new process info cache.
func NewCache() *Cache {
	return &Cache{entries: make(map[int]processInfo)}
}

// Info returns the process name (basename) and parent PID for a given PID,
// using the cache to avoid repeated ps invocations.
func (c *Cache) Info(pid int) (name string, ppid int) {
	if e, ok := c.entries[pid]; ok {
		return e.name, e.ppid
	}
	name, ppid = lookup(pid)
	c.entries[pid] = processInfo{name, ppid}
	return name, ppid
}

// ParentPID returns the PPID for a given PID.
func (c *Cache) ParentPID(pid int) int {
	_, ppid := c.Info(pid)
	return ppid
}

// IsDescendantOf checks if child is a descendant process of ancestor.
func (c *Cache) IsDescendantOf(child, ancestor int) bool {
	current := child
	for i := 0; i < 10; i++ {
		parent := c.ParentPID(current)
		if parent == 0 || parent == 1 {
			return false
		}
		if parent == ancestor {
			return true
		}
		current = parent
	}
	return false
}

// AncestorSet returns the set of all ancestor PIDs for the given PID,
// walking up to 20 levels. Useful for O(1) "is ancestor" checks.
func (c *Cache) AncestorSet(pid int) map[int]bool {
	set := make(map[int]bool)
	current := pid
	for i := 0; i < 20; i++ {
		parent := c.ParentPID(current)
		if parent == 0 || parent == 1 {
			break
		}
		set[parent] = true
		current = parent
	}
	return set
}

// --- package-level convenience functions (uncached, backward-compatible) ---

// Info returns the process name (basename) and parent PID for a given PID.
func Info(pid int) (name string, ppid int) {
	return lookup(pid)
}

// ParentPID returns the PPID for a given PID.
func ParentPID(pid int) int {
	_, ppid := Info(pid)
	return ppid
}

// IsDescendantOf checks if child is a descendant process of ancestor.
func IsDescendantOf(child, ancestor int) bool {
	current := child
	for i := 0; i < 10; i++ {
		parent := ParentPID(current)
		if parent == 0 || parent == 1 {
			return false
		}
		if parent == ancestor {
			return true
		}
		current = parent
	}
	return false
}

// lookup performs the actual ps invocation.
func lookup(pid int) (name string, ppid int) {
	out, err := exec.Command("ps", "-c", "-o", "comm=,ppid=", "-p", fmt.Sprintf("%d", pid)).Output()
	if err != nil {
		return "", 0
	}

	line := strings.TrimSpace(string(out))
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return "", 0
	}

	ppidStr := parts[len(parts)-1]
	fullPath := strings.Join(parts[:len(parts)-1], " ")

	// Use basename as safety net
	name = filepath.Base(fullPath)
	// Strip leading dash from login shells (e.g. "-zsh" → "zsh")
	name = strings.TrimPrefix(name, "-")

	_, _ = fmt.Sscanf(ppidStr, "%d", &ppid)
	return name, ppid
}
