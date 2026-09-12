package main

import (
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/core"
	"strconv"
)

func parseEventsArgs(args []string) (bool, int64, error) {
	jsonOutput := false
	var sinceOffset int64
	offsetSeen := false
	for len(args) > 0 {
		switch args[0] {
		case "--json":
			if jsonOutput {
				return false, 0, core.ErrInvalidArgument
			}
			jsonOutput = true
			args = args[1:]
		case "--since-offset":
			if offsetSeen || len(args) < 2 {
				return false, 0, core.ErrInvalidArgument
			}
			offset, err := strconv.ParseInt(args[1], 10, 64)
			if err != nil || offset < 0 {
				return false, 0, fmt.Errorf("invalid events offset %q: %w", args[1], core.ErrInvalidArgument)
			}
			sinceOffset = offset
			offsetSeen = true
			args = args[2:]
		default:
			return false, 0, fmt.Errorf("usage: haco events [--json] [--since-offset <byte-offset>]: %w", core.ErrInvalidArgument)
		}
	}
	return jsonOutput, sinceOffset, nil
}
