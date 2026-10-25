//go:build windows && (amd64 || arm64)

// haco-reclaim starts a fresh, explicitly confirmed Windows-native reclamation.
package main

import "github.com/SLktEx/Hacocoon/internal/cli/reclaimnative"

func main() { reclaimnative.Main() }
