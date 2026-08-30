package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestEgressCommandIsRegistered(t *testing.T) {
	err := dispatch(context.Background(), nil, []string{"egress", "nope"})
	if !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("error = %v, want ErrInvalidArgument", err)
	}
	if !strings.Contains(err.Error(), "usage: haco egress serve") {
		t.Fatalf("error = %v, command was not routed to egress handler", err)
	}
}
