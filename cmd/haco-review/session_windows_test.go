package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"runtime"
	"testing"
	"time"
)

func sessionTestName(t *testing.T) string {
	t.Helper()
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	return `Local\Hacocoon.SessionTest-` + hex.EncodeToString(nonce[:])
}

type sessionObservation struct {
	owned bool
	err   error
}

func observeSession(name string, wait time.Duration) <-chan sessionObservation {
	result := make(chan sessionObservation, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		s, err := openReviewSession(name, wait)
		if err != nil {
			result <- sessionObservation{err: err}
			return
		}
		owned := s.owned
		s.close()
		result <- sessionObservation{owned: owned}
	}()
	return result
}
func TestReviewSessionRequiresPublishedReadiness(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	name := sessionTestName(t)
	owner, err := openReviewSession(name, time.Second)
	if err != nil || !owner.owned {
		t.Fatal("could not own session", err)
	}
	defer owner.close()
	waiting := <-observeSession(name, 40*time.Millisecond)
	if !errors.Is(waiting.err, context.DeadlineExceeded) {
		t.Fatal("starting owner mistaken for ready COM", waiting)
	}
	if err := owner.publish(); err != nil {
		t.Fatal(err)
	}
	ready := <-observeSession(name, time.Second)
	if ready.err != nil || ready.owned {
		t.Fatal("ready owner not reused", ready)
	}
	// Closing a non-owner never withdraws the actual owner's publication.
	again := <-observeSession(name, time.Second)
	if again.err != nil || again.owned {
		t.Fatal("reader changed ownership/readiness", again)
	}
}
func TestReviewSessionWaitsForClosingOwnerThenTakesOwnership(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	name := sessionTestName(t)
	owner, err := openReviewSession(name, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.close()
	if err := owner.publish(); err != nil {
		t.Fatal(err)
	}
	if err := owner.withdraw(); err != nil {
		t.Fatal(err)
	}
	next := observeSession(name, time.Second)
	select {
	case got := <-next:
		t.Fatal("closing owner dispatched before cleanup", got)
	case <-time.After(40 * time.Millisecond):
	}
	owner.close()
	got := <-next
	if got.err != nil || !got.owned {
		t.Fatal("closed owner was not replaced", got)
	}
}
func TestReviewSessionUnpublishedFailureCanBeReplaced(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	name := sessionTestName(t)
	owner, err := openReviewSession(name, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.close()
	next := observeSession(name, time.Second)
	owner.close()
	got := <-next
	if got.err != nil || !got.owned {
		t.Fatal("failed startup left an unusable session", got)
	}
}
