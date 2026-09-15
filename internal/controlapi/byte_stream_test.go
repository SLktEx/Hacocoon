//go:build linux

package controlapi

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestByteStreamPreparationTimeoutCancelsTarget(t *testing.T) {
	done := make(chan struct{})
	client, _ := streamTestServer(t, streamTestService{dial: func(ctx context.Context, _ core.StreamTarget) (net.Conn, error) {
		defer close(done)
		<-ctx.Done()
		return nil, ctx.Err()
	}})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, err := client.openByteStream(ctx, MethodEnvironmentStream, streamTestTarget(), 100*time.Millisecond)
	if conn != nil || err == nil {
		t.Fatalf("late preparation accepted: conn=%v err=%v", conn, err)
	}
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("preparation retained target after cancellation")
	}
}

func TestByteStreamPreparationDeadlineDoesNotLimitActiveSession(t *testing.T) {
	target, peer := net.Pipe()
	defer func() { _ = peer.Close() }()
	client, _ := streamTestServer(t, streamTestService{dial: func(context.Context, core.StreamTarget) (net.Conn, error) { return target, nil }})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	const preparation = 250 * time.Millisecond
	conn, err := client.openByteStream(ctx, MethodEnvironmentStream, streamTestTarget(), preparation)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	// An application may legitimately send its first bytes after preparation's
	// budget has elapsed. Only the caller/session lifetime may terminate it.
	<-time.After(2 * preparation)
	done := make(chan error, 1)
	go func() { _, e := peer.Write([]byte("late application bytes")); done <- e }()
	b := make([]byte, len("late application bytes"))
	if _, err = io.ReadFull(conn, b); err != nil || string(b) != "late application bytes" {
		t.Fatalf("active session ended: %q %v", b, err)
	}
	if err = <-done; err != nil {
		t.Fatal(err)
	}
	if err = conn.Close(); err != nil {
		t.Fatal(err)
	}
	_ = peer.SetReadDeadline(time.Now().Add(time.Second))
	_, err = peer.Read(b)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("target retained after close: %v", err)
	}
}
