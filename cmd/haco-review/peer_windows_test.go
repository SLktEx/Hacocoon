package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/desktopreview"
)

func TestNativePrivateReviewChild(t *testing.T) {
	if os.Getenv("HACO_REVIEW_TEST_CHILD") != "1" {
		return
	}
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		os.Exit(2)
	}
	var m desktopreview.Message
	if json.Unmarshal(scanner.Bytes(), &m) != nil {
		os.Exit(2)
	}
	reply, _ := json.Marshal(desktopreview.Reply{Version: 1, Sequence: m.Sequence, Type: "pending"})
	fmt.Println(string(reply))
	// Wait with a blocked read: canceling the parent must close and reap this
	// exact child, not reconnect or retry its second message.
	for scanner.Scan() {
	}
	os.Exit(0)
}
func TestNativePrivatePeerCancellationClosesAndReapsChild(t *testing.T) {
	own, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	plan := desktopreview.Invocation{File: own, Args: []string{"-test.run=^TestNativePrivateReviewChild$"}, Env: []string{"HACO_REVIEW_TEST_CHILD=1", "SystemRoot=" + os.Getenv("SystemRoot")}}
	p, err := startReviewPeer(ctx, plan, own)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	reply, err := p.Exchange(ctx, desktopreview.Message{Action: "list"})
	if err != nil || reply.Type != "pending" {
		t.Fatal(reply, err)
	}
	waiting, stop := context.WithTimeout(ctx, 100*time.Millisecond)
	defer stop()
	if _, err := p.Exchange(waiting, desktopreview.Message{Action: "select", RequestID: strings.Repeat("a", 32)}); err == nil {
		t.Fatal("missing reply was accepted")
	}
	closed := make(chan struct{})
	go func() { p.Close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(3 * time.Second):
		t.Fatal("child cleanup did not finish")
	}
	if _, err := p.Exchange(ctx, desktopreview.Message{Action: "list"}); err == nil {
		t.Fatal("canceled peer was reused")
	}
}
