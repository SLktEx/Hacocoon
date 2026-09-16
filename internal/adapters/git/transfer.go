package gitadapter

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
)

// Pack bytes are separate from metadata. Memory use is independent of the
// transfer size; callers retain operation deadlines and ownership until EOF.
const (
	maxTransferBytes    int64  = 16 << 30
	maxTransferFrame           = 64 << 10
	maxTransferMetadata        = 2 << 20
	progressFrame       uint32 = 1 << 31
)

// transferWriter never buffers a complete pack or bypasses Write via ReadFrom.
// A failed write poisons the stream: its caller cannot finish it as successful.
type transferWriter struct {
	mu       sync.Mutex
	target   io.Writer
	bytes    int64
	failure  error
	finished bool
}

func (w *transferWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.failure != nil {
		return 0, w.failure
	}
	if w.finished {
		return 0, fmt.Errorf("git transfer is already complete")
	}
	if int64(len(data)) > maxTransferBytes-w.bytes {
		w.failure = fmt.Errorf("git pack exceeds transfer limit")
		return 0, w.failure
	}
	written := 0
	for len(data) > 0 {
		n := min(len(data), maxTransferFrame)
		var prefix [4]byte
		binary.BigEndian.PutUint32(prefix[:], uint32(n))
		if err := writeExact(w.target, prefix[:]); err != nil {
			w.failure = err
			return written, err
		}
		if err := writeExact(w.target, data[:n]); err != nil {
			w.failure = err
			return written, err
		}
		written += n
		w.bytes += int64(n)
		data = data[n:]
	}
	return written, nil
}

func (w *transferWriter) finish() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.failure != nil {
		return w.failure
	}
	if w.finished {
		return fmt.Errorf("git transfer is already complete")
	}
	w.finished = true
	w.failure = writeExact(w.target, []byte{0, 0, 0, 0})
	return w.failure
}

// transferReader exposes one bounded frame at a time, without trusting a peer's
// allocation size. Only an explicit zero frame is clean EOF; truncation fails.
type transferReader struct {
	progress  io.Writer
	source    io.Reader
	remaining uint32
	bytes     int64
	done      bool
	failure   error
}

func (r *transferReader) Read(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	if r.failure != nil {
		return 0, r.failure
	}
	if r.done {
		return 0, io.EOF
	}
	for r.remaining == 0 {
		var prefix [4]byte
		if _, err := io.ReadFull(r.source, prefix[:]); err != nil {
			r.failure = fmt.Errorf("incomplete Git transfer frame: %w", errors.Join(io.ErrUnexpectedEOF, err))
			return 0, r.failure
		}
		r.remaining = binary.BigEndian.Uint32(prefix[:])
		if r.remaining&progressFrame != 0 {
			n := r.remaining &^ progressFrame
			if r.progress == nil || n == 0 || n > 4096 {
				r.failure = fmt.Errorf("invalid Git progress frame")
				return 0, r.failure
			}
			line := make([]byte, n)
			if _, err := io.ReadFull(r.source, line); err != nil {
				r.failure = io.ErrUnexpectedEOF
				return 0, r.failure
			}
			if SafeGitLine(string(line)) != string(line) {
				r.failure = fmt.Errorf("invalid Git progress diagnostic")
				return 0, r.failure
			}
			if _, err := io.WriteString(r.progress, string(line)+"\n"); err != nil {
				r.failure = err
				return 0, err
			}
			r.remaining = 0
			continue
		}
		if r.remaining == 0 {
			r.done = true
			return 0, io.EOF
		}
		if r.remaining > maxTransferFrame || int64(r.remaining) > maxTransferBytes-r.bytes {
			r.failure = fmt.Errorf("git transfer frame exceeds limit")
			return 0, r.failure
		}
	}
	n, err := r.source.Read(data[:min(len(data), int(r.remaining))])
	r.remaining -= uint32(n)
	r.bytes += int64(n)
	if err != nil {
		r.failure = fmt.Errorf("incomplete Git transfer data: %w", errors.Join(io.ErrUnexpectedEOF, err))
		return n, r.failure
	}
	return n, nil
}

func writeExact(w io.Writer, data []byte) error {
	n, err := w.Write(data)
	if err == nil && n != len(data) {
		return io.ErrShortWrite
	}
	return err
}

func writeTransferMetadata(w io.Writer, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("invalid Git transfer metadata")
	}
	if len(data) > maxTransferMetadata {
		return fmt.Errorf("git transfer metadata exceeds limit")
	}
	var prefix [4]byte
	binary.BigEndian.PutUint32(prefix[:], uint32(len(data)))
	if err := writeExact(w, prefix[:]); err != nil {
		return err
	}
	return writeExact(w, data)
}

func readTransferMetadata(r io.Reader, target any) error {
	var prefix [4]byte
	if _, err := io.ReadFull(r, prefix[:]); err != nil {
		return fmt.Errorf("incomplete Git transfer metadata")
	}
	n := binary.BigEndian.Uint32(prefix[:])
	if n == 0 || n > maxTransferMetadata {
		return fmt.Errorf("git transfer metadata exceeds limit")
	}
	data := make([]byte, n)
	if _, err := io.ReadFull(r, data); err != nil {
		return fmt.Errorf("incomplete Git transfer metadata")
	}
	// Null is not a valid request/receipt and must not silently produce zero fields.
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return fmt.Errorf("invalid Git transfer metadata")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(target) != nil || decoder.Decode(new(any)) != io.EOF {
		return fmt.Errorf("invalid Git transfer metadata")
	}
	return nil
}

// countPackWriter retains the transfer bound even when io.Copy selects WriterTo.
type countPackWriter struct {
	target io.Writer
	bytes  int64
}

func (w *countPackWriter) Write(data []byte) (int, error) {
	if w.target == nil {
		return 0, fmt.Errorf("unexpected Git pack")
	}
	if int64(len(data)) > maxTransferBytes-w.bytes {
		return 0, fmt.Errorf("git pack exceeds transfer limit")
	}
	n, err := w.target.Write(data)
	w.bytes += int64(n)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	return n, err
}

func runPack(cmd *exec.Cmd, input io.Reader, output io.Writer) (int64, error) {
	counter := &countPackWriter{target: output}
	cmd.Stdin = input
	cmd.Stdout = counter
	err := cmd.Run()
	diagnostic, _ := cmd.Stderr.(*GitDiagnostic)
	if diagnostic != nil {
		diagnostic.Flush()
	}
	if err != nil {
		if diagnostic != nil && diagnostic.Failure() != "" {
			return counter.bytes, fmt.Errorf("git command failed: %s", diagnostic.Failure())
		}
		return counter.bytes, fmt.Errorf("git command failed")
	}
	return counter.bytes, nil
}

// Request metadata comes first so authority can be checked before consuming a
// guest pack. Pack EOF requires the frame terminator AND the transport's EOF.
type requestEnvelope[T any] struct {
	Metadata T    `json:"metadata"`
	HasPack  bool `json:"has_pack"`
}
type requestPack struct {
	*transferReader
	endChecked bool
}

func (r *requestPack) Read(data []byte) (int, error) {
	n, err := r.transferReader.Read(data)
	if err == io.EOF && !r.endChecked {
		r.endChecked = true
		var extra [1]byte
		if _, end := io.ReadFull(r.source, extra[:]); end != io.EOF {
			r.failure = fmt.Errorf("trailing Git request data")
			return n, r.failure
		}
	}
	return n, err
}

func readRequest[T any](r io.Reader) (T, io.Reader, error) {
	var envelope requestEnvelope[T]
	if err := readTransferMetadata(r, &envelope); err != nil {
		return envelope.Metadata, nil, err
	}
	pack := &requestPack{transferReader: &transferReader{source: r}}
	if envelope.HasPack {
		return envelope.Metadata, pack, nil
	}
	var probe [1]byte
	if n, err := pack.Read(probe[:]); n != 0 || err != io.EOF {
		return envelope.Metadata, nil, fmt.Errorf("unexpected Git request pack")
	}
	return envelope.Metadata, nil, nil
}
func ReadRequest(r io.Reader) (Request, error) {
	req, pack, err := readRequest[Request](r)
	req.Pack = pack
	return req, err
}
func ReadAgentRequest(r io.Reader) (AgentRequest, error) {
	req, pack, err := readRequest[AgentRequest](r)
	req.Pack = pack
	return req, err
}
func finishResponse(stream *transferWriter, result Response) error {
	if err := stream.finish(); err != nil {
		return err
	}
	// Failures can follow partial bytes; the receipt is mandatory before success.
	return writeTransferMetadata(stream.target, result)
}
func ReadResponse(r io.Reader, output io.Writer) (Response, error) {
	return ReadResponseProgress(r, output, nil)
}

// ReadResponseProgress separates bounded human diagnostics from pack bytes.
// Progress never substitutes for the final receipt or counts as transferred data.
func ReadResponseProgress(r io.Reader, output, progress io.Writer) (Response, error) {
	stream := &transferReader{source: r, progress: progress}
	count := &countPackWriter{target: output}
	if _, err := io.Copy(count, stream); err != nil {
		return Response{}, err
	}
	var result Response
	if err := readTransferMetadata(r, &result); err != nil {
		return Response{}, err
	}
	var extra [1]byte
	if _, err := io.ReadFull(r, extra[:]); err != io.EOF {
		return Response{}, fmt.Errorf("trailing Git response data")
	}
	if result.Error != "" {
		return Response{}, fmt.Errorf("git operation did not complete cleanly; inspect the remote and trusted Host before retrying")
	}
	if result.PackBytes != count.bytes {
		return Response{}, fmt.Errorf("git transfer receipt does not match pack")
	}
	return result, nil
}

// framedPackReader lets HTTP/exec pull a pack with backpressure without an
// independent encoder goroutine. It retains at most one frame plus its prefix.
type framedPackReader struct {
	source   io.Reader
	buffer   []byte
	pending  []byte
	bytes    int64
	terminal error
	done     bool
}

func (r *framedPackReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if len(r.pending) == 0 {
		if r.terminal != nil {
			return 0, r.terminal
		}
		if r.done {
			return 0, io.EOF
		}
		if r.buffer == nil {
			r.buffer = make([]byte, maxTransferFrame+4)
		}
		n, err := 0, io.EOF
		if r.source != nil {
			n, err = r.source.Read(r.buffer[4:])
		}
		if int64(n) > maxTransferBytes-r.bytes {
			r.terminal = fmt.Errorf("git pack exceeds transfer limit")
			return 0, r.terminal
		}
		r.bytes += int64(n)
		if err != nil && err != io.EOF {
			r.terminal = err
		}
		if n == 0 {
			if err != nil && err != io.EOF {
				return 0, err
			}
			if err == nil {
				return 0, nil
			}
			r.done = true
		}
		// n>0 with EOF emits its data first; the next read emits the zero terminator.
		if err == io.EOF {
			r.source = nil
		}
		binary.BigEndian.PutUint32(r.buffer[:4], uint32(n))
		r.pending = r.buffer[:4+n]
	}
	n := copy(p, r.pending)
	r.pending = r.pending[n:]
	return n, nil
}
func requestBody(metadata any, pack io.Reader) (io.Reader, error) {
	var header bytes.Buffer
	if err := writeTransferMetadata(&header, requestEnvelope[any]{Metadata: metadata, HasPack: pack != nil}); err != nil {
		return nil, err
	}
	return io.MultiReader(bytes.NewReader(header.Bytes()), &framedPackReader{source: pack}), nil
}
func RequestBody(req Request) (io.Reader, error)           { return requestBody(req, req.Pack) }
func AgentRequestBody(req AgentRequest) (io.Reader, error) { return requestBody(req, req.Pack) }

// ServeExchange owns the complete framed request/response lifetime. The caller
// retains authority and source locks while exchange consumes and produces bytes.
func ServeExchange(ctx context.Context, input io.Reader, output io.Writer, exchange Exchange) error {
	request, err := ReadRequest(input)
	stream := &transferWriter{target: output}
	request.PackOutput = stream
	var response Response
	if err == nil {
		response, err = exchange(WithProgress(ctx, NewGitDiagnostic(streamProgress{stream})), request)
	}
	if err != nil {
		response = Response{Error: "operation did not complete cleanly; inspect the remote and trusted Host before retrying"}
	}
	if sendErr := finishResponse(stream, response); err == nil {
		err = sendErr
	}
	return err
}

// streamProgress shares serialization with pack writes so subprocess stderr can
// arrive concurrently without corrupting frames. Only filtered lines are sent.
type streamProgress struct{ stream *transferWriter }

func (p streamProgress) Write(data []byte) (int, error) {
	line := strings.TrimSuffix(string(data), "\n")
	if line == "" || len(line) > 4096 || SafeGitLine(line) != line {
		return 0, fmt.Errorf("invalid Git progress")
	}
	w := p.stream
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.failure != nil {
		return 0, w.failure
	}
	if w.finished {
		return 0, fmt.Errorf("Git response already complete")
	}
	var prefix [4]byte
	binary.BigEndian.PutUint32(prefix[:], progressFrame|uint32(len(line)))
	w.failure = writeExact(w.target, prefix[:])
	if w.failure == nil {
		w.failure = writeExact(w.target, []byte(line))
	}
	if w.failure != nil {
		return 0, w.failure
	}
	return len(data), nil
}
