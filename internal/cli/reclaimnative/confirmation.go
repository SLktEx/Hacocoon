package reclaimnative

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/cli/ui"
)

type confirmationKey struct {
	character uint16
	repeat    uint16
	down      bool
}

// poll is a nonblocking console event read, not a read of cooked characters.
// The bounded answer and event-at-a-time context checks need no background
// reader, console-mode changes or cancellation of arbitrary I/O.
func confirmEvents(ctx context.Context, out io.Writer, language cliui.Language, poll func() (confirmationKey, bool, error)) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if _, err := fmt.Fprint(out, language.Text("reclaim.text.confirm")); err != nil {
		return false, err
	}
	var answer []byte
	for {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		key, available, err := poll()
		if err != nil {
			return false, err
		}
		if !available {
			timer := time.NewTimer(10 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return false, ctx.Err()
			case <-timer.C:
			}
			continue
		}
		if !key.down || key.character == 0 {
			continue
		}
		switch key.character {
		case 3:
			return false, context.Canceled
		case 27:
			return false, nil
		case '\r':
			if ctx.Err() != nil {
				return false, ctx.Err()
			}
			if _, err := fmt.Fprintln(out); err != nil {
				return false, err
			}
			text := strings.TrimSpace(string(answer))
			return strings.EqualFold(text, "y") || strings.EqualFold(text, "yes"), nil
		case '\b':
			if len(answer) > 0 {
				answer = answer[:len(answer)-1]
				if _, err := io.WriteString(out, "\b \b"); err != nil {
					return false, err
				}
			}
		default:
			if key.character < 32 || key.character > 126 || key.repeat == 0 || int(key.repeat) > 127-len(answer) {
				return false, errors.New("invalid or oversized confirmation")
			}
			text := strings.Repeat(string(rune(key.character)), int(key.repeat))
			answer = append(answer, text...)
			if _, err := io.WriteString(out, text); err != nil {
				return false, err
			}
		}
	}
}
