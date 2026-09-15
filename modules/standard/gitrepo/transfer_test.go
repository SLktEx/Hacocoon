package gitrepo

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"strings"
	"testing"
)

const legacyPackLimit = 32 << 20

type zeroPack struct{}

func (zeroPack) Read(p []byte) (int, error) { clear(p); return len(p), nil }

func TestTransferStreamsBeyondOldPackLimitWithBoundedFrames(t *testing.T) {
	const size = 40 << 20
	reader, writer := io.Pipe()
	defer func() { _ = reader.Close() }()
	done := make(chan error, 1)
	go func() {
		stream := &transferWriter{target: writer}
		n, err := io.Copy(stream, io.LimitReader(zeroPack{}, size))
		if err == nil {
			err = finishResponse(stream, Response{PackBytes: n})
		}
		_ = writer.CloseWithError(err)
		done <- err
	}()
	actual := sha256.New()
	result, err := ReadResponse(reader, actual)
	_ = reader.CloseWithError(err)
	if producer := <-done; producer != nil {
		t.Fatal(producer)
	}
	if err != nil || result.PackBytes != size {
		t.Fatal(result, err)
	}
	expected := sha256.New()
	if _, err := io.Copy(expected, io.LimitReader(zeroPack{}, size)); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual.Sum(nil), expected.Sum(nil)) {
		t.Fatal("stream content changed")
	}
}

func TestRequestPackRequiresTerminatorAndTransportEOF(t *testing.T) {
	body, err := RequestBody(Request{Operation: "push", Repository: "demo", Pack: strings.NewReader("pack")})
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"valid", "truncated", "trailing"} {
		t.Run(mode, func(t *testing.T) {
			value := append([]byte(nil), data...)
			switch mode {
			case "truncated":
				value = value[:len(value)-1]
			case "trailing":
				value = append(value, 1)
			}
			req, err := ReadRequest(bytes.NewReader(value))
			if err != nil {
				t.Fatal(err)
			}
			pack, err := io.ReadAll(req.Pack)
			if string(pack) != "pack" || (err == nil) != (mode == "valid") {
				t.Fatal(string(pack), err)
			}
		})
	}
}

func TestTransferRejectsOversizeFramesBeforeReadingData(t *testing.T) {
	var prefix [4]byte
	binary.BigEndian.PutUint32(prefix[:], maxTransferFrame+1)
	r := &transferReader{source: bytes.NewReader(prefix[:])}
	if _, err := io.Copy(io.Discard, r); err == nil {
		t.Fatal("oversize frame accepted")
	}
	binary.BigEndian.PutUint32(prefix[:], 2)
	r = &transferReader{source: bytes.NewReader(prefix[:]), bytes: maxTransferBytes - 1}
	if _, err := io.Copy(io.Discard, r); err == nil {
		t.Fatal("cumulative bound bypassed")
	}
}

type shortTransferWriter struct{}

func (shortTransferWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }
func TestTransferWriteFailureCannotFinishSuccessfully(t *testing.T) {
	w := &transferWriter{target: shortTransferWriter{}}
	if _, err := w.Write([]byte("data")); !errors.Is(err, io.ErrShortWrite) {
		t.Fatal(err)
	}
	if err := w.finish(); !errors.Is(err, io.ErrShortWrite) {
		t.Fatal(err)
	}
	w = &transferWriter{target: io.Discard, bytes: maxTransferBytes}
	if _, err := w.Write([]byte{1}); err == nil {
		t.Fatal("limit bypassed")
	}
	if err := w.finish(); err == nil {
		t.Fatal("failed transfer finished successfully")
	}
}

func TestTransferResponseRequiresMatchingFinalReceipt(t *testing.T) {
	for _, mode := range []string{"valid", "missing", "wrong count", "operation failed", "trailing", "unexpected pack"} {
		t.Run(mode, func(t *testing.T) {
			var wire bytes.Buffer
			w := &transferWriter{target: &wire}
			if _, err := w.Write([]byte("pack")); err != nil {
				t.Fatal(err)
			}
			result := Response{PackBytes: 4}
			if mode == "wrong count" {
				result.PackBytes = 3
			}
			if mode == "operation failed" {
				result.Error = "failed after bytes"
			}
			if mode == "missing" {
				if err := w.finish(); err != nil {
					t.Fatal(err)
				}
			} else if err := finishResponse(w, result); err != nil {
				t.Fatal(err)
			}
			if mode == "trailing" {
				wire.WriteByte(1)
			}
			sink := io.Discard
			if mode == "unexpected pack" {
				sink = nil
			}
			_, err := ReadResponse(&wire, sink)
			if (err == nil) != (mode == "valid") {
				t.Fatal(err)
			}
		})
	}
}

func TestTransferMetadataRejectsUnknownGuestAuthorityAndMalformedDocuments(t *testing.T) {
	for _, data := range []string{`null`, `{} {}`, `{"metadata":{"operation":"list","repository":"demo","remote":"file:///private"},"has_pack":false}`} {
		var wire bytes.Buffer
		var prefix [4]byte
		binary.BigEndian.PutUint32(prefix[:], uint32(len(data)))
		wire.Write(prefix[:])
		wire.WriteString(data)
		wire.Write([]byte{0, 0, 0, 0})
		if _, err := ReadRequest(&wire); err == nil {
			t.Fatal("invalid metadata accepted", data)
		}
	}
}
