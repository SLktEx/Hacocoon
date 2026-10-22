//go:build !linux

package terminalbridge

import "io"

// The interactive product client runs on Linux, including its WSL entry.
// Other platforms retain caller-owned input until they have an owned reader.
func ownInput(input io.Reader) (io.Reader, func(), error) { return input, nil, nil }
