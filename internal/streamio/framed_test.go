package streamio

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"
	"time"
)

func framedPair(t *testing.T) (*FramedConn, *FramedConn) {
	t.Helper()
	ar, bw := io.Pipe()
	br, aw := io.Pipe()
	a, b := NewFramedConn(ar, aw), NewFramedConn(br, bw)
	t.Cleanup(func() { _ = a.Close(); _ = b.Close() })
	return a, b
}
func TestFramedHalfCloseAndBoundedBinaryFrames(t *testing.T) {
	a, b := framedPair(t)
	_ = a.SetDeadline(time.Now().Add(3 * time.Second))
	_ = b.SetDeadline(time.Now().Add(3 * time.Second))
	data := bytes.Repeat([]byte{0, 10, 13, 255}, 256000)
	result := make(chan error, 1)
	go func() {
		received, err := io.ReadAll(b)
		if err == nil && !bytes.Equal(received, data) {
			err = errors.New("request bytes changed")
		}
		if err == nil {
			_, err = b.Write(received)
		}
		if err == nil {
			err = b.CloseWrite()
		}
		result <- err
	}()
	if _, err := a.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := a.CloseWrite(); err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(a)
	if err != nil || !bytes.Equal(data, got) {
		t.Fatal(len(got), err)
	}
	if err = <-result; err != nil {
		t.Fatal(err)
	}
	if _, err = a.Write([]byte("late")); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
}
func TestFramedPeerEOFBeforeRequestEnd(t *testing.T) {
	a, b := framedPair(t)
	_ = a.SetDeadline(time.Now().Add(time.Second))
	_ = b.SetDeadline(time.Now().Add(time.Second))
	closed := make(chan error, 1)
	go func() { closed <- b.CloseWrite() }()
	buf := make([]byte, 1)
	if _, err := a.Read(buf); !errors.Is(err, io.EOF) {
		t.Fatal(err)
	}
	if err := <-closed; err != nil {
		t.Fatal(err)
	}
	sent := make(chan error, 1)
	go func() {
		_, err := a.Write([]byte("x"))
		if err == nil {
			err = a.CloseWrite()
		}
		sent <- err
	}()
	got, err := io.ReadAll(b)
	if err != nil || string(got) != "x" {
		t.Fatal(string(got), err)
	}
	if err = <-sent; err != nil {
		t.Fatal(err)
	}
}
func TestFramedRejectsMalformedAndTruncatedControl(t *testing.T) {
	for _, wire := range [][]byte{{0x12, 0, 0, 0, 0}, {frameData, 0, 1, 0, 0}, {frameData, 0, 0, 0, 0}, {frameEOF, 0, 0, 0, 1}, {frameData, 0, 0, 0, 2, 1}} {
		reader, writer := io.Pipe()
		outReader, outWriter := io.Pipe()
		c := NewFramedConn(reader, outWriter)
		go func() { _, _ = writer.Write(wire); _ = writer.Close() }()
		_ = c.SetReadDeadline(time.Now().Add(time.Second))
		var b [1]byte
		_, err := c.Read(b[:])
		_ = c.Close()
		_ = outReader.Close()
		if err == nil || errors.Is(err, io.EOF) {
			t.Fatal("malformed transport became successful EOF", wire, err)
		}
	}
}
func TestFramedDeadlineClosesBlockedWriteAndJoinsReader(t *testing.T) {
	reader, writer := io.Pipe()
	outReader, outWriter := io.Pipe()
	defer func() { _ = writer.Close() }()
	defer func() { _ = outReader.Close() }()
	c := NewFramedConn(reader, outWriter)
	defer func() { _ = c.Close() }()
	_ = c.SetWriteDeadline(time.Now().Add(30 * time.Millisecond))
	if _, err := c.Write([]byte("blocked")); err == nil {
		t.Fatal("write outlived deadline")
	}
	if !errors.Is(c.failure(), os.ErrDeadlineExceeded) {
		t.Fatal(c.failure())
	}
}
