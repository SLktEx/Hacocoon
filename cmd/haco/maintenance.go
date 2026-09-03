package main

import (
	"fmt"
	"os"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// maintenance compact is intercepted by the installed Windows launcher before
// the Linux haco binary is entered. Keep a pre-runtime fallback here so native
// Ubuntu and direct WSL invocations fail clearly instead of depending on Incus
// or controller availability.
func init() {
	handled, err := handleMaintenanceArgs(os.Args[1:])
	if !handled {
		return
	}
	if err != nil {
		fail(err)
	}
	os.Exit(0)
}

func handleMaintenanceArgs(args []string) (bool, error) {
	if len(args) == 0 || args[0] != "maintenance" {
		return false, nil
	}
	if len(args) != 2 || args[1] != "compact" {
		return true, fmt.Errorf("usage: haco maintenance compact: %w", core.ErrInvalidArgument)
	}
	return true, fmt.Errorf("haco maintenance compact must run from Windows PowerShell or cmd.exe through the installed Hacocoon Windows launcher so the dedicated WSL VHD can be fully offline: %w", core.ErrUnsupported)
}
