package core

import (
	"errors"
	"strings"
	"testing"
)

func TestProcessRequestPreservesArgumentsWithinExecutionBounds(t *testing.T) {
	for _, request := range []ProcessRequest{
		{Argv: []string{"printf", "%s", "", "-n", "日本語", "a\nb", "$(touch unwanted)"}},
		{WorkingDirectory: "/workspace/project with spaces", Argv: []string{"tool"}, TTY: true},
		{WorkingDirectory: "/", Argv: []string{strings.Repeat("x", 32<<10)}},
		{Argv: append([]string{"tool"}, make([]string, 255)...)},
	} {
		before := strings.Join(request.Argv, "\x00")
		if err := ValidateProcessRequest(request); err != nil {
			t.Fatal("supported process request rejected", err)
		}
		if strings.Join(request.Argv, "\x00") != before {
			t.Fatal("validation rewrote literal arguments")
		}
	}
	for _, request := range []ProcessRequest{
		{}, {Argv: []string{""}}, {Argv: append([]string{"tool"}, make([]string, 256)...)},
		{Argv: []string{strings.Repeat("x", 32<<10), "overflow"}},
		{Argv: []string{"tool", "nul\x00argument"}},
		{Argv: []string{"tool"}, WorkingDirectory: "relative"},
		{Argv: []string{"tool"}, WorkingDirectory: "/workspace\x00other"},
		{Argv: []string{"tool"}, WorkingDirectory: "/workspace\r\ncommand"},
	} {
		if err := ValidateProcessRequest(request); !errors.Is(err, ErrInvalidArgument) {
			t.Fatal("unrepresentable or unbounded execution request accepted", err)
		}
	}
}

func TestForwardAddressRequiresLiteralUnscopedLoopback(t *testing.T) {
	for _, test := range []struct {
		address string
		port    int
		valid   bool
	}{
		{"127.0.0.1", 1, true}, {"127.255.255.254", 65535, true}, {"::1", 8080, true},
		{"localhost", 80, false}, {"0.0.0.0", 80, false}, {"::", 80, false},
		{"192.0.2.1", 80, false}, {"::ffff:127.0.0.1", 80, false}, {"::1%lo", 80, false},
		{"127.0.0.1", 0, false}, {"127.0.0.1", 65536, false}, {"127.0.0.1", -1, false},
	} {
		if got := ValidForwardAddress(test.address, test.port); got != test.valid {
			t.Fatalf("address %q port %d: valid=%t", test.address, test.port, got)
		}
	}
}
