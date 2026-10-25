package reclaimnative

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/cli/ui"
	"io"
	"testing"
	"time"
)

func TestConfirmationNoEventsIsCancellable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	confirmed, err := confirmEvents(ctx, io.Discard, cliui.English, func() (confirmationKey, bool, error) { return confirmationKey{}, false, nil })
	if confirmed || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(confirmed, err)
	}
}
func TestConfirmationEventsAreBoundedAndExplicit(t *testing.T) {
	cases := []struct {
		name    string
		keys    []confirmationKey
		yes     bool
		failure bool
	}{
		{"yes", []confirmationKey{{'y', 1, true}, {'e', 1, true}, {'s', 1, true}, {'\r', 1, true}}, true, false},
		{"keyup-not-confirmation", []confirmationKey{{'y', 1, false}, {'\r', 1, true}}, false, false},
		{"repeat", []confirmationKey{{'y', 2, true}, {'\r', 1, true}}, false, false},
		{"backspace", []confirmationKey{{'y', 1, true}, {'x', 1, true}, {'\b', 1, true}, {'\r', 1, true}}, true, false},
		{"control-c", []confirmationKey{{3, 1, true}}, false, true},
		{"escape", []confirmationKey{{27, 1, true}}, false, false},
		{"oversized", []confirmationKey{{'y', 128, true}}, false, true},
		{"nonascii", []confirmationKey{{0x3042, 1, true}}, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			index := 0
			yes, err := confirmEvents(context.Background(), io.Discard, cliui.English, func() (confirmationKey, bool, error) {
				if index == len(tc.keys) {
					return confirmationKey{}, false, io.EOF
				}
				key := tc.keys[index]
				index++
				return key, true, nil
			})
			if yes != tc.yes || (err != nil) != tc.failure {
				t.Fatal(yes, err)
			}
		})
	}
}
