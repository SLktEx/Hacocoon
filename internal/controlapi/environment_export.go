package controlapi

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"regexp"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/environmenttransfer"
)

const MethodEnvironmentExport = "environment.export"

// A product safety budget, independent of Incus compression or client file paths.
const EnvironmentExportLimit int64 = environmenttransfer.DefaultPayloadLimit
const environmentExportWireLimit = EnvironmentExportLimit + (512 << 10)

var exportSourcePattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,55}[a-z0-9])?$`)
var exportSnapshotPattern = regexp.MustCompile(`^snap-[a-f0-9]{32}$`)

type EnvironmentExportRequest struct {
	Source string `json:"source"`
}

func (r EnvironmentExportRequest) Validate() error {
	if !exportSourcePattern.MatchString(r.Source) {
		return core.ErrInvalidArgument
	}
	return nil
}

type EnvironmentExportResult struct {
	Bytes             int64  `json:"bytes"`
	SHA256            string `json:"sha256"`
	TemporarySnapshot string `json:"temporary_snapshot,omitempty"`
}
type environmentExportFrame struct {
	Data   []byte                   `json:"data,omitempty"`
	Result *EnvironmentExportResult `json:"result,omitempty"`
	Error  *responseStatus          `json:"error,omitempty"`
}

// The wire uses canonical JSON objects. Reject duplicate/unknown fields and
// conflicting alternatives before interpreting byte counts or completion.
func decodeExportJSON(data []byte, target any) error {
	if err := json.Unmarshal(data, target); err != nil {
		return control.ErrProtocol
	}
	canonical, err := json.Marshal(target)
	if err != nil || !bytes.Equal(bytes.TrimSpace(data), canonical) {
		return control.ErrProtocol
	}
	return nil
}

// ExportEnvironment streams into a private, unpublished caller-owned sink. A nil
// error proves the explicit terminal receipt and EOF, not EOF alone. The caller
// must discard partial output on every error and publish only after file sync.
func (c *Client) ExportEnvironment(ctx context.Context, source string, sink io.Writer) (result EnvironmentExportResult, err error) {
	if !exportSourcePattern.MatchString(source) || sink == nil {
		return result, core.ErrInvalidArgument
	}
	conn, err := c.wire.OpenStream(ctx, MethodEnvironmentExport, EnvironmentExportRequest{Source: source})
	if err != nil {
		return result, err
	}
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 4096), 128<<10)
	hash := sha256.New()
	var count int64
	for scanner.Scan() {
		var frame environmentExportFrame
		if err := decodeExportJSON(scanner.Bytes(), &frame); err != nil {
			return result, err
		}
		if frame.Result != nil {
			result = *frame.Result
			if len(frame.Data) != 0 || (result.TemporarySnapshot != "" && !exportSnapshotPattern.MatchString(result.TemporarySnapshot)) {
				return result, control.ErrProtocol
			}
			if frame.Error != nil {
				return result, responseError(frame.Error)
			}
			if result.TemporarySnapshot != "" || result.Bytes != count || count == 0 || result.SHA256 != hex.EncodeToString(hash.Sum(nil)) {
				return result, control.ErrProtocol
			}
			if scanner.Scan() {
				return result, control.ErrProtocol
			}
			if err := scanner.Err(); err != nil {
				return result, err
			}
			if err := ctx.Err(); err != nil {
				return result, err
			}
			return result, nil
		}
		if frame.Error != nil || len(frame.Data) == 0 || len(frame.Data) > 64<<10 || int64(len(frame.Data)) > environmentExportWireLimit-count {
			return result, control.ErrProtocol
		}
		if err := ctx.Err(); err != nil {
			return result, err
		}
		n, err := sink.Write(frame.Data)
		if err != nil {
			return result, err
		}
		if n != len(frame.Data) {
			return result, io.ErrShortWrite
		}
		hash.Write(frame.Data)
		count += int64(n)
	}
	if err := scanner.Err(); err != nil {
		return result, err
	}
	return result, io.ErrUnexpectedEOF
}
