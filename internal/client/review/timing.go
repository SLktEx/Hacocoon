package desktopreview

import "time"

// Startup and predecessor cleanup are different phases. The outer notifier
// covers both without extending normal controller read or decision deadlines.
const (
	StartupTimeout     = 50 * time.Second
	SessionWaitTimeout = 60 * time.Second
	LaunchTimeout      = 120 * time.Second
)
