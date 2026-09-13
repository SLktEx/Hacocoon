package desktopreview

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"sync"
)

// Peer is the bounded other end of Session's private child-process protocol.
// Process lifetime/cancellation and stderr limits belong to the native adapter.
// A failed exchange poisons this peer: a possibly submitted decision is never
// resent on another child or inferred to have succeeded.
type Peer struct {
	in       *bufio.Reader
	out      io.Writer
	sequence uint64
	failed   bool
	mu       sync.Mutex
}

func NewPeer(input io.Reader, output io.Writer) *Peer {
	return &Peer{in: bufio.NewReaderSize(input, 256<<10), out: output}
}

func (p *Peer) Exchange(message Message) (Reply, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.failed {
		return Reply{}, ErrInvalid
	}
	p.failed = true
	p.sequence++
	message.Version, message.Sequence = 1, p.sequence
	data, err := json.Marshal(message)
	if err != nil || len(data) >= 4096 {
		return Reply{}, ErrInvalid
	}
	data = append(data, '\n')
	n, err := p.out.Write(data)
	if err != nil {
		return Reply{}, err
	}
	if n != len(data) {
		return Reply{}, io.ErrShortWrite
	}
	line, err := p.in.ReadSlice('\n')
	if err != nil || len(line) > 256<<10 {
		return Reply{}, ErrInvalid
	}
	var reply Reply
	decoder := json.NewDecoder(bytes.NewReader(line))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&reply) != nil || decoder.Decode(new(any)) != io.EOF || reply.Version != 1 || reply.Sequence != p.sequence {
		return Reply{}, ErrInvalid
	}
	valid := false
	switch reply.Type {
	case "error":
		valid = reply.Error != "" && reply.View == nil && reply.Result == nil && len(reply.Pending) == 0
	case "pending":
		valid = message.Action == "list" && reply.View == nil && reply.Result == nil && reply.Error == "" && len(reply.Pending) <= 128
		seen := map[string]bool{}
		for _, item := range reply.Pending {
			if !requestPattern.MatchString(item.RequestID) || seen[item.RequestID] {
				valid = false
			}
			seen[item.RequestID] = true
		}
	case "selected":
		valid = message.Action == "select" && reply.View != nil && reply.View.Request.RequestID == message.RequestID && ValidateView(*reply.View) == nil && reply.Result == nil && reply.Error == "" && len(reply.Pending) == 0
	case "result":
		valid = message.Action == "decide" && reply.View == nil && len(reply.Pending) == 0 && (reply.Result != nil && reply.Result.RequestID == message.RequestID || reply.Error == "outcome_unconfirmed")
	}
	if !valid {
		return Reply{}, ErrInvalid
	}
	p.failed = false
	return reply, nil
}
