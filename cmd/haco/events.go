package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/core"
	eventsapp "github.com/SLktEx/Hacocoon/internal/events"
)

func eventsCommand(ctx context.Context, app *composition.App, args []string) error {
	return eventsCommandTo(ctx, app, args, os.Stdout)
}

func eventsCommandTo(ctx context.Context, app *composition.App, args []string, out io.Writer) error {
	jsonOutput := false
	var sinceOffset int64
	offsetSeen := false
	for len(args) > 0 {
		switch args[0] {
		case "--json":
			if jsonOutput {
				return core.ErrInvalidArgument
			}
			jsonOutput = true
			args = args[1:]
		case "--since-offset":
			if offsetSeen || len(args) < 2 {
				return core.ErrInvalidArgument
			}
			offset, err := strconv.ParseInt(args[1], 10, 64)
			if err != nil || offset < 0 {
				return fmt.Errorf("invalid events offset %q: %w", args[1], core.ErrInvalidArgument)
			}
			sinceOffset = offset
			offsetSeen = true
			args = args[2:]
		default:
			return fmt.Errorf("usage: haco events [--json] [--since-offset <byte-offset>]: %w", core.ErrInvalidArgument)
		}
	}

	var encoder *json.Encoder
	if jsonOutput {
		encoder = json.NewEncoder(out)
	}
	_, err := app.Events.Stream(ctx, sinceOffset, func(event eventsapp.Event) error {
		if encoder != nil {
			return encoder.Encode(event)
		}
		_, err := fmt.Fprintf(out, "%s\t%s\t%s\t%s\t%s\n", event.Time.UTC().Format("2006-01-02T15:04:05Z07:00"), event.Type, event.Capability, event.Action, event.Decision)
		return err
	})
	return err
}
