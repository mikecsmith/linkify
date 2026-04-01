package server

import _ "embed"

// Embedded sources for the macOS URL handler.
// These are compiled into the binary so lfy service install works
// from go install, Homebrew, or GoReleaser — no source checkout needed.

//go:embed macos/main.swift
var SwiftSource []byte

//go:embed macos/Info.plist
var InfoPlist []byte
