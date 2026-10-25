package wslreclaim

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/SLktEx/Hacocoon/internal/storage/reclamation"
)

func nativeTargetFixture() reclamation.WSLTarget {
	return reclamation.WSLTarget{RegistrationID: "{12345678-1234-1234-1234-123456789abc}", InstallationID: "abcdef12-3456-7890-abcd-123456789abc"}
}

func TestNativeSelectionRequiresOneExplicitCandidate(t *testing.T) {
	candidates := []nativeRegistrationName{{"one", "Hacocoon"}, {"two", "Other"}}
	for _, name := range []string{"Hacocoon", "hacocoon", "HACOCOON"} {
		if got, err := selectNativeRegistration(name, candidates); err != nil || got != "one" {
			t.Fatal(got, err)
		}
	}
	for _, name := range []string{"", "default", "Missing", "--shutdown", "Hacocoon\n", " Hacocoon", "../Hacocoon", strings.Repeat("x", 257)} {
		if got, err := selectNativeRegistration(name, candidates); err == nil || got != "" {
			t.Fatal("accepted invalid or absent selection", name, got, err)
		}
	}
	for _, candidates := range [][]nativeRegistrationName{
		{{"one", "Hacocoon"}, {"two", "hacocoon"}},
		{{"one", "Hacocoon"}, {"one", "Hacocoon"}},
		{{"", "Hacocoon"}},
		make([]nativeRegistrationName, maxNativeRegistrations+1),
	} {
		if _, err := selectNativeRegistration("Hacocoon", candidates); err == nil {
			t.Fatal("accepted ambiguous, invalid or excessive enumeration")
		}
	}
}

func TestNativeDistributionNameEncodingIsBoundedAndLiteral(t *testing.T) {
	encode := func(s string) []byte {
		var raw []byte
		for _, unit := range append(utf16.Encode([]rune(s)), 0) {
			raw = append(raw, byte(unit), byte(unit>>8))
		}
		return raw
	}
	for _, name := range []string{"Hacocoon", "Other distribution", "日本語", strings.Repeat("x", 256)} {
		if got, err := decodeNativeDistributionName(encode(name)); err != nil || got != name {
			t.Fatal(got, err)
		}
	}
	for _, raw := range [][]byte{nil, {0}, {0, 0}, {1, 0, 2}, {1, 0, 2, 0}, encode("Hacocoon\x00Other"), encode(strings.Repeat("x", 257))} {
		if _, err := decodeNativeDistributionName(raw); err == nil {
			t.Fatal("accepted malformed or oversized name", raw)
		}
	}
}

func TestNativePreparationPreflightRefusalsNeverStartDiscovery(t *testing.T) {
	// The registry decoder remains the canonical PR772 decoder. Here each
	// possible refusal is injected at its production ordering boundary without
	// opening a real registry, disk, WSL or subprocess.
	for _, state := range []string{"pending", "malformed", "oversized", "wrong type", "unknown schema", "read error"} {
		t.Run(state, func(t *testing.T) {
			retained := []byte(state)
			before := append([]byte(nil), retained...)
			cause := errors.New(state)
			forbidden := func(context.Context) (reclamation.WSLTarget, error) {
				t.Fatal("refused preflight started WSL discovery")
				return reclamation.WSLTarget{}, nil
			}
			err := prepareNativeDiscovery(context.Background(), nativeTargetFixture(), func() error { return cause }, forbidden, forbidden,
				func() error { t.Fatal("refused preflight wrote intent"); return nil })
			var failure *targetFailure
			if !errors.Is(err, cause) || !errors.As(err, &failure) || failure.stage != "intent" || !bytes.Equal(before, retained) {
				t.Fatal("refusal lost stage/cause or changed evidence", err)
			}
		})
	}
}

func TestNativePreparationOrdersOwnedDiscoveryBeforePersistenceAndRelease(t *testing.T) {
	var order []string
	target := nativeTargetFixture()
	read := func(name string) func(context.Context) (reclamation.WSLTarget, error) {
		return func(ctx context.Context) (reclamation.WSLTarget, error) {
			order = append(order, name)
			err := finishNativeDiscovery(ctx, nil, func() error { order = append(order, name+" absent"); return nil })
			return target, err
		}
	}
	err := prepareNativeDiscovery(context.Background(), target,
		func() error { order = append(order, "preflight"); return nil }, read("identity"), read("controller"),
		func() error { order = append(order, "persist version 2"); return nil })
	err = closePreparationReservation(err, func() error { order = append(order, "release"); return nil })
	want := []string{"preflight", "identity", "identity absent", "controller", "controller absent", "persist version 2", "release"}
	if err != nil || !reflect.DeepEqual(order, want) {
		t.Fatal(order, err)
	}
}

func TestNativePreparationRejectsEitherWrongTargetAndReadFailure(t *testing.T) {
	for _, phase := range []int{0, 1} {
		for _, mutation := range []string{"registration", "installation", "invalid", "read failure"} {
			t.Run(mutation+string(rune('0'+phase)), func(t *testing.T) {
				target := nativeTargetFixture()
				calls := 0
				read := func(context.Context) (reclamation.WSLTarget, error) {
					current := calls
					calls++
					observed := target
					if current == phase {
						switch mutation {
						case "registration":
							observed.RegistrationID = "{99999999-1234-1234-1234-123456789abc}"
						case "installation":
							observed.InstallationID = "99999999-3456-7890-abcd-123456789abc"
						case "invalid":
							observed = reclamation.WSLTarget{}
						case "read failure":
							return observed, errors.New("read failed despite plausible identity")
						}
					}
					return observed, nil
				}
				err := prepareNativeDiscovery(context.Background(), target, func() error { return nil }, read, read,
					func() error { t.Fatal("unproven identity persisted intent"); return nil })
				if err == nil || calls != phase+1 {
					t.Fatal(calls, err)
				}
			})
		}
	}
}

func TestNativeSuccessfulRPCWithCleanupFailureRetainsReservations(t *testing.T) {
	for _, failAt := range []int{0, 1} {
		var calls, cleanup int
		cause := errors.New("native descendant absence unconfirmed")
		target := nativeTargetFixture()
		read := func(ctx context.Context) (reclamation.WSLTarget, error) {
			current := calls
			calls++
			return target, finishNativeDiscovery(ctx, nil, func() error {
				cleanup++
				if current == failAt {
					return cause
				}
				return nil
			})
		}
		err := prepareNativeDiscovery(context.Background(), target, func() error { return nil }, read, read,
			func() error { t.Fatal("cleanup uncertainty wrote intent"); return nil })
		for range 2 {
			err = closePreparationReservation(err, func() error { t.Fatal("cleanup uncertainty released reservation"); return nil })
		}
		if !errors.Is(err, errNativeDiscoveryCleanup) || !errors.Is(err, cause) || calls != failAt+1 || cleanup != calls {
			t.Fatal(calls, cleanup, err)
		}
	}
}

func TestNativeCancellationAndFailedPersistenceNeverRetry(t *testing.T) {
	for _, cancelAt := range []int{-1, 0, 1, 2} {
		ctx, cancel := context.WithCancel(context.Background())
		var calls, persists int
		if cancelAt == -1 {
			cancel()
		}
		read := func(ctx context.Context) (reclamation.WSLTarget, error) {
			calls++
			if calls == cancelAt {
				cancel()
			}
			return nativeTargetFixture(), finishNativeDiscovery(ctx, nil, func() error { return nil })
		}
		persistErr := errors.New("flush/readback failed")
		err := prepareNativeDiscovery(ctx, nativeTargetFixture(), func() error { return nil }, read, read,
			func() error { persists++; return persistErr })
		cancel()
		if cancelAt == 0 {
			if !errors.Is(err, persistErr) || calls != 2 || persists != 1 {
				t.Fatal(calls, persists, err)
			}
		} else if !errors.Is(err, context.Canceled) || persists != 0 {
			t.Fatal(calls, persists, err)
		}
		closed := 0
		err = closePreparationReservation(err, func() error { closed++; return nil })
		if closed != 1 {
			t.Fatal("proven cleanup did not release reservation", err)
		}
	}
}

func TestNativeCleanupPreservesOperationAndReservationCloseErrors(t *testing.T) {
	operation, cleanup, release := errors.New("read failed"), errors.New("close failed"), errors.New("release failed")
	err := finishNativeDiscovery(context.Background(), operation, func() error { return cleanup })
	if !errors.Is(err, operation) || !errors.Is(err, cleanup) || !errors.Is(err, errNativeDiscoveryCleanup) {
		t.Fatal(err)
	}
	err = closePreparationReservation(operation, func() error { return release })
	if !errors.Is(err, operation) || !errors.Is(err, release) {
		t.Fatal(err)
	}
}
