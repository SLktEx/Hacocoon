package desktopreview

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

type shortReviewWrite struct{}

func (shortReviewWrite) Write(p []byte) (int, error) { return len(p) - 1, nil }

func TestReviewPeerRejectsAndPoisonsMalformedReplies(t *testing.T) {
	for _, input := range []string{
		`{"version":1,"sequence":2,"type":"pending"}` + "\n",
		`{"version":1,"sequence":1,"type":"pending","extra":true}` + "\n",
		`{"version":1,"sequence":1,"type":"pending"} {}` + "\n",
		`{"version":1,"sequence":1,"type":"pending"}`,
		`{"version":1,"sequence":1,"type":"result"}` + "\n",
		`{"version":1,"sequence":1,"type":"pending","pending":[{"request_id":"bad"}]}` + "\n",
		strings.Repeat(" ", 256<<10) + "\n",
	} {
		var out bytes.Buffer
		p := NewPeer(strings.NewReader(input), &out)
		if _, err := p.Exchange(Message{Action: "list"}); err == nil {
			t.Fatal("accepted invalid reply")
		}
		sent := out.Len()
		if _, err := p.Exchange(Message{Action: "list"}); err == nil || out.Len() != sent {
			t.Fatal("poisoned peer resent")
		}
	}
	p := NewPeer(strings.NewReader(""), shortReviewWrite{})
	if _, err := p.Exchange(Message{Action: "list"}); err != io.ErrShortWrite {
		t.Fatal(err)
	}
}

func TestReviewPeerSessionRoundTripUsesPrivateSequenceAndToken(t *testing.T) {
	session, backend, id := newReviewFixture()
	input, response := io.Pipe()
	commands, output := io.Pipe()
	done := make(chan error, 1)
	go func() { done <- session.Serve(context.Background(), commands, response); response.Close() }()
	p := NewPeer(input, output)
	defer input.Close()
	selected, err := p.Exchange(Message{Action: "select", RequestID: id, Sequence: 900, Version: 9})
	if err != nil || selected.Sequence != 1 {
		t.Fatal(selected, err)
	}
	approved := true
	result, err := p.Exchange(Message{Action: "decide", RequestID: id, Token: selected.View.Token, Approved: &approved})
	if err != nil || result.Result == nil || len(backend.decisions) != 1 {
		t.Fatal(result, err)
	}
	data, _ := json.Marshal(result)
	if bytes.Contains(data, []byte("secret provider")) {
		t.Fatal("provider output leaked")
	}
	output.Close()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
