//go:build !linux

package egress

import "github.com/SLktEx/Hacocoon/internal/core"

func lockFile(string) (func(), error) { return nil, core.ErrUnsupported }
