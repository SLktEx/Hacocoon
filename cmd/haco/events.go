package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/SLktEx/Hacocoon/internal/composition"
	eventsapp "github.com/SLktEx/Hacocoon/internal/events"
)

func eventsCommand(ctx context.Context, app *composition.App, args []string) error {
	return eventsCommandTo(ctx, app, args, os.Stdout)
}

func eventsCommandTo(ctx context.Context, app *composition.App, args []string, out io.Writer) error {
	jsonOutput, sinceOffset, err := parseEventsArgs(args)
	if err != nil {
		return err
	}

	var encoder *json.Encoder
	if jsonOutput {
		encoder = json.NewEncoder(out)
	}
	_, err = app.Events.Stream(ctx, sinceOffset, func(event eventsapp.Event) error {
		if encoder != nil {
			return encoder.Encode(event)
		}
		_, err := fmt.Fprintf(out, "%s\t%s\t%s\t%s\t%s\n", event.Time.UTC().Format("2006-01-02T15:04:05Z07:00"), event.Type, event.Capability, event.Action, event.Decision)
		return err
	})
	return err
}
