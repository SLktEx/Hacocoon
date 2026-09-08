package controlapi

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
	awsplugin "github.com/SLktEx/Hacocoon/modules/capability/aws"
	"io"
	"net"
	"time"
)

const MethodAWSDownload = "aws.s3.download"

type awsDownloadService interface {
	Download(context.Context, awsplugin.GetSpec, io.Writer) (core.CapabilityResult, error)
}
type awsDownloadFrame struct {
	Data   []byte                 `json:"data,omitempty"`
	Result *core.CapabilityResult `json:"result,omitempty"`
	Error  *responseStatus        `json:"error,omitempty"`
}
type awsDownloadWriter struct {
	ctx     context.Context
	conn    net.Conn
	encoder *json.Encoder
}

func (w awsDownloadWriter) Write(data []byte) (int, error) {
	total := 0
	for len(data) > 0 {
		if err := w.ctx.Err(); err != nil {
			return total, err
		}
		n := len(data)
		if n > 64<<10 {
			n = 64 << 10
		}
		if err := w.conn.SetWriteDeadline(time.Now().Add(30 * time.Second)); err != nil {
			return total, err
		}
		if err := w.encoder.Encode(awsDownloadFrame{Data: data[:n]}); err != nil {
			return total, err
		}
		total += n
		data = data[n:]
	}
	return total, nil
}
func registerAWSDownload(server *control.Server, service awsDownloadService) error {
	return server.RegisterStream(MethodAWSDownload, func(_ context.Context, payload json.RawMessage) (control.Stream, error) {
		var spec awsplugin.GetSpec
		d := json.NewDecoder(bytes.NewReader(payload))
		d.DisallowUnknownFields()
		if d.Decode(&spec) != nil || d.Decode(new(any)) != io.EOF {
			return nil, control.ErrInvalidArgument
		}
		return func(ctx context.Context, conn net.Conn) error {
			encoder := json.NewEncoder(conn)
			result, err := service.Download(ctx, spec, awsDownloadWriter{ctx, conn, encoder})
			if deadlineErr := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); deadlineErr != nil {
				return deadlineErr
			}
			return encoder.Encode(awsDownloadFrame{Result: &result, Error: statusFromError(err)})
		}, nil
	})
}
func (c *Client) DownloadS3(ctx context.Context, spec awsplugin.GetSpec, sink io.Writer) (core.CapabilityResult, error) {
	if sink == nil {
		return core.CapabilityResult{}, core.ErrInvalidArgument
	}
	conn, err := c.wire.OpenStream(ctx, MethodAWSDownload, spec)
	if err != nil {
		return core.CapabilityResult{}, err
	}
	defer conn.Close()
	hash := sha256.New()
	var count int64
	scan := bufio.NewScanner(conn)
	scan.Buffer(make([]byte, 4096), 128<<10)
	for scan.Scan() {
		var frame awsDownloadFrame
		d := json.NewDecoder(bytes.NewReader(scan.Bytes()))
		d.DisallowUnknownFields()
		if d.Decode(&frame) != nil || d.Decode(new(any)) != io.EOF {
			return core.CapabilityResult{}, control.ErrProtocol
		}
		if frame.Result != nil {
			if len(frame.Data) != 0 {
				return core.CapabilityResult{}, control.ErrProtocol
			}
			if err := responseError(frame.Error); err != nil {
				return *frame.Result, err
			}
			if err := awsplugin.VerifyDownload(*frame.Result, count, hex.EncodeToString(hash.Sum(nil))); err != nil {
				return *frame.Result, err
			}
			return *frame.Result, nil
		}
		if frame.Error != nil || len(frame.Data) == 0 || len(frame.Data) > 64<<10 {
			return core.CapabilityResult{}, control.ErrProtocol
		}
		n, err := sink.Write(frame.Data)
		if err != nil {
			return core.CapabilityResult{}, err
		}
		if n != len(frame.Data) {
			return core.CapabilityResult{}, io.ErrShortWrite
		}
		hash.Write(frame.Data)
		count += int64(n)
	}
	return core.CapabilityResult{}, control.ErrProtocol
}
