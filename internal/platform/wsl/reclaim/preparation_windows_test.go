//go:build windows && (amd64 || arm64)

package wslreclaim

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"golang.org/x/sys/windows/registry"
)

func TestPreparationRefusesUncertainRecordBeforeInstallationRead(t *testing.T) {
	for _, tc := range []struct {
		name    string
		change  func(*operationRecord)
		raw     []byte
		missing bool
		refuse  bool
	}{
		{name: "missing", missing: true},
		{name: "pending", refuse: true},
		{name: "Linux started pending", change: func(r *operationRecord) { r.LinuxStarted = true }, refuse: true},
		{name: "failed", change: func(r *operationRecord) { r.State = "failed" }},
		{name: "interrupted", change: func(r *operationRecord) { r.State = "interrupted" }},
		{name: "complete", change: func(r *operationRecord) {
			r.State, r.LinuxStarted, r.Linux = "complete", true, linuxSuccess()
			r.Observation = continuationObservation{StopAttempted: true, StopRequested: true,
				Compaction: compactObservation{Attempted: true, Completed: true}, ResumeAttempted: true, Resumed: true}
		}},
		{name: "malformed", raw: []byte("not a record"), refuse: true},
		{name: "oversized", raw: bytes.Repeat([]byte("x"), 16385), refuse: true},
		{name: "unknown version", change: func(r *operationRecord) { r.Version = 3 }, refuse: true},
		{name: "unknown state", change: func(r *operationRecord) { r.State = "unknown" }, refuse: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, current := linuxRecordFixture(t, 2)
			if tc.change != nil {
				tc.change(&current)
			}
			raw := tc.raw
			if raw == nil {
				var err error
				raw, err = json.Marshal(current)
				if err != nil {
					t.Fatal(err)
				}
			}
			if tc.missing {
				if err := store.key.DeleteValue("Operation"); err != nil {
					t.Fatal(err)
				}
			} else if err := store.key.SetBinaryValue("Operation", raw); err != nil {
				t.Fatal(err)
			}
			beforeNames, err := store.key.ReadValueNames(0)
			if err != nil {
				t.Fatal(err)
			}
			reads := 0
			identity := installationIdentity{SchemaVersion: 1}
			got, err := readReclamationInstallation(context.Background(), store.refusePendingPreparation,
				func(context.Context) (installationIdentity, error) {
					reads++
					return identity, nil
				})
			if tc.refuse {
				if err == nil || reads != 0 || got != (installationIdentity{}) || PreparationFailure(err).Stage != "intent" {
					t.Fatalf("refusal accessed WSL identity or lost intent failure: reads=%d identity=%+v err=%v", reads, got, err)
				}
				if current.State == "pending" && tc.raw == nil && current.Version == 2 && !errors.Is(err, errOperationNeedsReview) {
					t.Fatal("pending refusal lost its review requirement", err)
				}
			} else if err != nil || reads != 1 || got != identity {
				t.Fatal("valid state did not reach the unchanged identity check exactly once", reads, got, err)
			}
			after, _, readErr := store.key.GetBinaryValue("Operation")
			if tc.missing {
				if !errors.Is(readErr, registry.ErrNotExist) {
					t.Fatal("absence created a record", readErr)
				}
			} else if readErr != nil || !bytes.Equal(after, raw) {
				t.Fatal("preflight changed retained bytes", readErr)
			}
			afterNames, err := store.key.ReadValueNames(0)
			if err != nil || !reflect.DeepEqual(beforeNames, afterNames) {
				t.Fatal("preflight changed registry values", beforeNames, afterNames, err)
			}
			// Passing this early check is not permission to replace a terminal
			// result that lacks its explicit review evidence.
			if tc.name == "failed" || tc.name == "interrupted" {
				if _, err := store.beginVersion(current.Registration, current.Disk, 2); err == nil {
					t.Fatal("early pass bypassed canonical review requirement")
				}
			}
		})
	}
}

func TestPreparationReadFailureNeverInvokesInstallationReader(t *testing.T) {
	store, _ := linuxRecordFixture(t, 2)
	if err := store.key.SetStringValue("Operation", "not binary"); err != nil {
		t.Fatal(err)
	}
	if _, err := readReclamationInstallation(context.Background(), store.refusePendingPreparation,
		func(context.Context) (installationIdentity, error) {
			t.Fatal("wrong registry type reached WSL")
			return installationIdentity{}, nil
		}); err == nil || PreparationFailure(err).Stage != "intent" {
		t.Fatal("wrong registry type accepted", err)
	}
	if value, kind, err := store.key.GetStringValue("Operation"); err != nil || kind != registry.SZ || value != "not binary" {
		t.Fatal("wrongly typed evidence changed", value, kind, err)
	}
	// The fixture cleanup remains the sole owner of its real registry handle.
	// A zero handle exercises a read failure without closing/reusing that handle.
	unreadable := &operationStore{}
	_, readErr := unreadable.read()
	if readErr == nil {
		t.Fatal("invalid registry key unexpectedly readable")
	}
	if _, err := readReclamationInstallation(context.Background(), unreadable.refusePendingPreparation,
		func(context.Context) (installationIdentity, error) {
			t.Fatal("registry access error reached WSL")
			return installationIdentity{}, nil
		}); !errors.Is(err, readErr) || PreparationFailure(err).Stage != "intent" {
		t.Fatal("registry read error was lost or treated as absence", err, readErr)
	}
}

func TestPreparationIdentitySequencePreservesErrorsAndCancellation(t *testing.T) {
	cause := errors.New("native read failed")
	for _, preflightFails := range []bool{false, true} {
		var order []string
		_, err := readReclamationInstallation(context.Background(), func() error {
			order = append(order, "preflight")
			if preflightFails {
				return cause
			}
			return nil
		}, func(context.Context) (installationIdentity, error) {
			order = append(order, "identity")
			return installationIdentity{}, cause
		})
		want, stage := []string{"preflight", "identity"}, "installation"
		if preflightFails {
			want, stage = []string{"preflight"}, "intent"
		}
		if !reflect.DeepEqual(order, want) || !errors.Is(err, cause) || PreparationFailure(err).Stage != stage {
			t.Fatal(order, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := readReclamationInstallation(ctx, func() error {
		t.Fatal("canceled preparation read records")
		return nil
	}, func(context.Context) (installationIdentity, error) {
		t.Fatal("canceled preparation reached WSL")
		return installationIdentity{}, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
