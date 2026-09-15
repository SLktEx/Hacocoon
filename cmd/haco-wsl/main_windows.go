//go:build windows && (amd64 || arm64)

// haco-wsl is an internal Windows installation/continuation helper, not another user-facing CLI.
package main

import "github.com/SLktEx/Hacocoon/internal/cli/wsl"

func main() { wslcli.Main() }
