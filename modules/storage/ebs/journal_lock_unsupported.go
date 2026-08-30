//go:build !linux

package ebs

import (
	"os"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func tryExclusiveFileLock(*os.File) (bool, error) { return false, core.ErrUnsupported }
func unlockExclusiveFile(*os.File) error          { return nil }
