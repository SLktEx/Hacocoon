package controlapi

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"hash"
	"io"
	"net"
	"regexp"
	"time"

	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/env/transfer"
)

const MethodEnvironmentImport = "environment.import"

type EnvironmentImportRequest struct {
	Environment string `json:"environment,omitempty"`
}

func (r EnvironmentImportRequest) Validate() error {
	if r.Environment != "" && !exportSourcePattern.MatchString(r.Environment) {
		return core.ErrInvalidArgument
	}
	return nil
}

type environmentImportEnd struct {
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}
type environmentImportUpload struct {
	Data []byte                `json:"data,omitempty"`
	End  *environmentImportEnd `json:"end,omitempty"`
}
type environmentImportResponse = uploadResponse[environmenttransfer.ImportResult]

type uploadResponse[T any] struct {
	Result T                    `json:"result"`
	Upload environmentImportEnd `json:"upload"`
	Error  *responseStatus      `json:"error,omitempty"`
}

var importPublicName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,56}$`)

func validImportResult(r environmenttransfer.ImportResult) bool {
	if r.Environment != "" && !exportSourcePattern.MatchString(r.Environment) {
		return false
	}
	if r.Workspace != "" && !importPublicName.MatchString(r.Workspace) {
		return false
	}
	if r.OCI != "" && (len(r.OCI) < 5 || r.OCI[:4] != "oci:" || !importPublicName.MatchString(r.OCI[4:])) {
		return false
	}
	if len(r.Offline) > 8 {
		return false
	}
	for _, name := range r.Offline {
		if len(name) > 48 || !importPublicName.MatchString(name) {
			return false
		}
	}
	switch r.State {
	case "", "failed", "cleanup-required", "start-failed", "running":
		return true
	default:
		return false
	}
}

// RegisterEnvironmentImport is for the management endpoint only. The callback
// must verify/stage the entire reader before any native mutation (Importer.Import
// does this). Source paths, budgets and native configuration are not client input.
func RegisterEnvironmentImport(server *control.Server, receive func(context.Context, io.Reader, string) (environmenttransfer.ImportResult, error)) error {
	if receive == nil {
		return core.ErrInvalidArgument
	}
	return registerImportStream(server, MethodEnvironmentImport, func(payload json.RawMessage) (func(context.Context, io.Reader) (environmenttransfer.ImportResult, error), error) {
		var req EnvironmentImportRequest
		if decodeExportJSON(payload, &req) != nil || req.Validate() != nil {
			return nil, control.ErrInvalidArgument
		}
		return func(ctx context.Context, r io.Reader) (environmenttransfer.ImportResult, error) {
			return receive(ctx, r, req.Environment)
		}, nil
	}, func(r environmenttransfer.ImportResult) bool { return r.State == "running" })
}

// registerImportStream shares framing, cancellation and exact upload receipts.
// Each domain validates its own request/result and owns staging and publication.
func registerImportStream[T any](server *control.Server, method string, prepare func(json.RawMessage) (func(context.Context, io.Reader) (T, error), error), successful func(T) bool) error {
	return server.RegisterStream(method, func(_ context.Context, payload json.RawMessage) (control.Stream, error) {
		receive, err := prepare(payload)
		if err != nil {
			return nil, err
		}
		return func(ctx context.Context, conn net.Conn) error {
			ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
			defer cancel()
			stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
			defer stop()
			reader := &environmentImportReader{reader: bufio.NewReaderSize(conn, 128<<10), hash: sha256.New(), conn: conn, limit: environmentExportWireLimit}
			disconnected := make(chan struct{})
			reader.complete = func() {
				// After the explicit upload end, any further byte or disconnect cancels
				// activation. The connection remains open for the terminal operation receipt.
				go func() { defer close(disconnected); _, _ = reader.reader.ReadByte(); cancel() }()
			}
			result, err := receive(ctx, reader)
			if !reader.done {
				err = errors.Join(err, control.ErrProtocol)
			}
			if reader.done {
				defer func() { _ = conn.Close(); <-disconnected }()
			}
			if err == nil && !successful(result) {
				err = core.ErrRecoveryRequired
			}
			if err := conn.SetWriteDeadline(time.Now().Add(30 * time.Second)); err != nil {
				return err
			}
			return json.NewEncoder(conn).Encode(uploadResponse[T]{Result: result, Upload: environmentImportEnd{Bytes: reader.count, SHA256: hex.EncodeToString(reader.hash.Sum(nil))}, Error: statusFromError(err)})
		}, nil
	})
}

type environmentImportReader struct {
	reader       *bufio.Reader
	hash         hash.Hash
	conn         net.Conn
	count, limit int64
	pending      []byte
	done         bool
	complete     func()
}

func (r *environmentImportReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if len(r.pending) > 0 {
		n := copy(p, r.pending)
		r.pending = r.pending[n:]
		return n, nil
	}
	if r.done {
		return 0, io.EOF
	}
	if err := r.conn.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
		return 0, err
	}
	line, err := r.reader.ReadSlice('\n')
	if err != nil {
		if err == io.EOF {
			return 0, io.ErrUnexpectedEOF
		}
		return 0, err
	}
	var frame environmentImportUpload
	if decodeExportJSON(line, &frame) != nil {
		return 0, control.ErrProtocol
	}
	if frame.End != nil {
		if len(frame.Data) != 0 || r.count == 0 || frame.End.Bytes != r.count || frame.End.SHA256 != hex.EncodeToString(r.hash.Sum(nil)) {
			return 0, control.ErrProtocol
		}
		if err := r.conn.SetReadDeadline(time.Time{}); err != nil {
			return 0, err
		}
		r.done = true
		if r.complete != nil {
			r.complete()
		}
		return 0, io.EOF
	}
	if len(frame.Data) == 0 || len(frame.Data) > 64<<10 || int64(len(frame.Data)) > r.limit-r.count {
		return 0, control.ErrProtocol
	}
	r.hash.Write(frame.Data)
	r.count += int64(len(frame.Data))
	r.pending = frame.Data
	return r.Read(p)
}

// ImportEnvironment sends bytes, never a controller-side path. Reader ownership
// stays with the caller, which must ensure its reads can finish or be cancelled.
// An operation failure returns its retained-resource receipt alongside the error.
func (c *Client) ImportEnvironment(ctx context.Context, source io.Reader, name string) (environmenttransfer.ImportResult, error) {
	req := EnvironmentImportRequest{Environment: name}
	if source == nil || req.Validate() != nil {
		return environmenttransfer.ImportResult{}, core.ErrInvalidArgument
	}
	return uploadInput(ctx, c, source, MethodEnvironmentImport, req, validImportResult, func(r environmenttransfer.ImportResult) bool {
		return r.State == "running" && exportSourcePattern.MatchString(r.Environment) && r.Workspace != "" && (name == "" || r.Environment == name)
	})
}

func uploadInput[T any](ctx context.Context, c *Client, source io.Reader, method string, req any, valid func(T) bool, successful func(T) bool) (T, error) {
	var result T
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	conn, err := c.wire.OpenStream(ctx, method, req)
	if err != nil {
		return result, err
	}
	defer conn.Close()
	// Read concurrently so a rejected upload cannot deadlock with its error reply.
	type answer struct {
		result T
		upload environmentImportEnd
		err    error
	}
	answers := make(chan answer, 1)
	go func() {
		a := answer{}
		defer func() { _ = conn.Close(); answers <- a }()
		scanner := bufio.NewScanner(conn)
		scanner.Buffer(make([]byte, 4096), 64<<10)
		if !scanner.Scan() {
			a.err = scanner.Err()
			if a.err == nil {
				a.err = io.ErrUnexpectedEOF
			}
			return
		}
		var response uploadResponse[T]
		if decodeExportJSON(scanner.Bytes(), &response) != nil {
			a.err = control.ErrProtocol
			return
		}
		if !valid(response.Result) {
			a.err = control.ErrProtocol
			return
		}
		a.result = response.Result
		a.upload = response.Upload
		a.err = responseError(response.Error)
		if scanner.Scan() {
			a.err = control.ErrProtocol
			return
		}
		if e := scanner.Err(); e != nil {
			a.err = e
			return
		}
		if a.err == nil && !successful(a.result) {
			a.err = control.ErrProtocol
		}
	}()
	encoder := json.NewEncoder(conn)
	send := func(frame environmentImportUpload) error {
		if err := conn.SetWriteDeadline(time.Now().Add(30 * time.Second)); err != nil {
			return err
		}
		return encoder.Encode(frame)
	}
	hash := sha256.New()
	var count int64
	buffer := make([]byte, 64<<10)
	for {
		n, readErr := source.Read(buffer)
		if n > 0 {
			if int64(n) > environmentExportWireLimit-count {
				err = core.ErrInvalidArgument
				break
			}
			hash.Write(buffer[:n])
			count += int64(n)
			if err = send(environmentImportUpload{Data: buffer[:n]}); err != nil {
				break
			}
		}
		if readErr == io.EOF {
			err = send(environmentImportUpload{End: &environmentImportEnd{Bytes: count, SHA256: hex.EncodeToString(hash.Sum(nil))}})
			break
		}
		if readErr != nil {
			err = readErr
			break
		}
		if err = ctx.Err(); err != nil {
			break
		}
	}
	if err != nil {
		_ = conn.Close()
	}
	a := <-answers
	if a.err == nil && (a.upload.Bytes != count || count == 0 || a.upload.SHA256 != hex.EncodeToString(hash.Sum(nil))) {
		a.err = control.ErrProtocol
	}
	return a.result, errors.Join(err, a.err)
}
