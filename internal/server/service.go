package server

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

const serviceLabel = "dev.roseland.linkify"

// Build compiles the URL handler to the install location.
func Build() error {
	switch runtime.GOOS {
	case "darwin":
		return buildApp()
	case "linux":
		return nil // nothing to compile on Linux
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// Install builds the URL handler (if needed), registers the lfy:// scheme,
// and installs a service to keep it running.
func Install() error {
	switch runtime.GOOS {
	case "darwin":
		return installDarwin()
	case "linux":
		return installLinux()
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// Uninstall stops the service and removes the URL handler.
func Uninstall() error {
	switch runtime.GOOS {
	case "darwin":
		return uninstallDarwin()
	case "linux":
		return uninstallLinux()
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// Start starts the installed service.
func Start() error {
	switch runtime.GOOS {
	case "darwin":
		return launchdctl("bootstrap", guiDomain(), launchdPlistPath())
	case "linux":
		return fmt.Errorf("linux uses on-demand launching via .desktop file — no service needed")
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// Stop stops the running service.
func Stop() error {
	switch runtime.GOOS {
	case "darwin":
		return launchdctl("bootout", guiDomain(), launchdPlistPath())
	case "linux":
		return fmt.Errorf("linux uses on-demand launching via .desktop file — no service needed")
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// Status prints whether the service is running.
func Status() {
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("launchctl", "print", guiDomain()+"/"+serviceLabel).CombinedOutput()
		if err != nil {
			fmt.Println("stopped")
			return
		}
		_ = out
		fmt.Println("running")
	case "linux":
		fmt.Println("Linux uses on-demand launching via .desktop file")
	}
}

// --- macOS ---

func appDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "linkify")
}

func appBundlePath() string {
	return filepath.Join(appDir(), "Linkify.app")
}

func appBinaryPath() string {
	return filepath.Join(appBundlePath(), "Contents", "MacOS", "linkify-url-handler")
}

func launchdPlistPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", serviceLabel+".plist")
}

func guiDomain() string {
	return fmt.Sprintf("gui/%d", os.Getuid())
}

func installDarwin() error {
	if needsBuild() {
		if err := buildApp(); err != nil {
			return fmt.Errorf("build URL handler: %w", err)
		}
	} else {
		fmt.Println("URL handler is up to date, skipping build")
	}

	// Register URL scheme
	lsregister := "/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister"
	if err := exec.Command(lsregister, "-R", appBundlePath()).Run(); err != nil {
		return fmt.Errorf("lsregister: %w", err)
	}

	// Capture the current PATH at install time so the URL handler can find
	// kitty, wezterm, tmux, nvim etc. macOS launchd agents get a minimal
	// PATH (/usr/bin:/bin:/usr/sbin:/sbin) by default.
	installPath := os.Getenv("PATH")

	// Create launchd plist to keep the app running
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>EnvironmentVariables</key>
	<dict>
		<key>PATH</key>
		<string>%s</string>
	</dict>
</dict>
</plist>`, serviceLabel, appBinaryPath(), installPath)

	path := launchdPlistPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	// Stop existing service if running (ignore errors on clean install)
	_ = exec.Command("launchctl", "bootout", guiDomain(), path).Run()
	time.Sleep(500 * time.Millisecond)

	if err := os.WriteFile(path, []byte(plist), 0644); err != nil {
		return err
	}

	if err := launchdctl("bootstrap", guiDomain(), path); err != nil {
		return fmt.Errorf("launchctl bootstrap: %w", err)
	}

	fmt.Println("Installed lfy service")
	fmt.Printf("  app:   %s\n", appBundlePath())
	fmt.Printf("  plist: %s\n", path)
	fmt.Println("  lfy:// URL scheme registered")
	fmt.Println("  Service running — clicks are instant")
	return nil
}

func needsBuild() bool {
	appBin, err := os.Stat(appBinaryPath())
	if err != nil {
		return true // doesn't exist
	}
	self, err := os.Executable()
	if err != nil {
		return true
	}
	selfInfo, err := os.Stat(self)
	if err != nil {
		return true
	}
	// Rebuild if lfy binary is newer than the handler
	return selfInfo.ModTime().After(appBin.ModTime())
}

func buildApp() error {
	bundleDir := appBundlePath()
	macosDir := filepath.Join(bundleDir, "Contents", "MacOS")
	if err := os.MkdirAll(macosDir, 0755); err != nil {
		return err
	}

	// Write embedded Info.plist
	if err := os.WriteFile(filepath.Join(bundleDir, "Contents", "Info.plist"), InfoPlist, 0644); err != nil {
		return err
	}

	// Write embedded Swift source to a temp file and compile
	tmpDir, err := os.MkdirTemp("", "lfy-build-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	swiftFile := filepath.Join(tmpDir, "main.swift")
	if err := os.WriteFile(swiftFile, SwiftSource, 0644); err != nil {
		return err
	}

	cmd := exec.Command("swiftc", "-O", "-o", filepath.Join(macosDir, "linkify-url-handler"), swiftFile)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func uninstallDarwin() error {
	path := launchdPlistPath()
	_ = launchdctl("bootout", guiDomain(), path)
	_ = os.Remove(path)

	// Unregister and remove .app
	lsregister := "/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister"
	_ = exec.Command(lsregister, "-u", appBundlePath()).Run()
	_ = os.RemoveAll(appBundlePath())
	_ = os.Remove(appDir()) // rmdir if empty

	fmt.Println("Uninstalled lfy service and URL handler")
	return nil
}

func launchdctl(args ...string) error {
	cmd := exec.Command("launchctl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// --- Linux ---

func installLinux() error {
	home, _ := os.UserHomeDir()
	desktopDir := filepath.Join(home, ".local", "share", "applications")
	if err := os.MkdirAll(desktopDir, 0755); err != nil {
		return err
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return err
	}

	desktop := fmt.Sprintf(`[Desktop Entry]
Name=Linkify
Exec=%s open %%u
Type=Application
NoDisplay=true
MimeType=x-scheme-handler/lfy;
`, resolved)

	path := filepath.Join(desktopDir, "linkify.desktop")
	if err := os.WriteFile(path, []byte(desktop), 0644); err != nil {
		return err
	}

	// Register as default handler for lfy:// scheme
	_ = exec.Command("xdg-mime", "default", "linkify.desktop", "x-scheme-handler/lfy").Run()

	fmt.Println("Installed lfy:// URL handler")
	fmt.Printf("  desktop: %s\n", path)
	fmt.Println("  Linux uses on-demand launching — no background service needed")
	return nil
}

func uninstallLinux() error {
	home, _ := os.UserHomeDir()
	_ = os.Remove(filepath.Join(home, ".local", "share", "applications", "linkify.desktop"))
	fmt.Println("Uninstalled lfy URL handler")
	return nil
}
