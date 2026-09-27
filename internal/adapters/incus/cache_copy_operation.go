package incus

import (
	"context"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/core"
	"net/url"
	"regexp"
	"time"
)

var copyOperationID = regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`)

type copyOperationResponse struct {
	Type       string `json:"type"`
	StatusCode int    `json:"status_code"`
	ErrorCode  int    `json:"error_code"`
	Operation  string `json:"operation"`
	Metadata   struct {
		ID         string `json:"id"`
		StatusCode int    `json:"status_code"`
	} `json:"metadata"`
}

// CopyTracked is deliberately restricted to collection from an ordinary Env.
// The shared copy path independently verifies the stopped consumer and owner.
func (b *PersistentResourceBackend) CopyTracked(ctx context.Context, source, target core.PersistentResource, record func(string) error, completed func() error) error {
	if source.Kind != CacheResourceKind || !core.ValidEnvironmentResourceRef(source.Ref()) || !core.ValidGenerationResource(target.Ref()) || !target.SourceOnly || target.Producer != source.Ref() || record == nil || completed == nil {
		return core.ErrInvalidArgument
	}
	return b.copyWithReceipt(ctx, source, target, record, completed)
}

func (b *PersistentResourceBackend) submitTrackedCopy(ctx context.Context, pool string, data []byte, record func(string) error) error {
	r, err := b.Runtime.runner.Run(ctx, "incus", "query", "--raw", "-X", "POST", "/1.0/storage-pools/"+pool+"/volumes/custom?project="+b.Runtime.project, "--data", string(data))
	var response copyOperationResponse
	if err != nil || r.ExitCode != 0 || r.StdoutTruncated || json.Unmarshal([]byte(r.Stdout), &response) != nil || response.Type != "async" || response.StatusCode != 100 || response.ErrorCode != 0 {
		return core.ErrRecoveryRequired
	}
	u, err := url.Parse(response.Operation)
	if err != nil || u.Scheme != "" || u.Host != "" || u.Fragment != "" || u.RawPath != "" {
		return core.ErrRecoveryRequired
	}
	id := response.Metadata.ID
	if !copyOperationID.MatchString(id) || u.Path != "/1.0/operations/"+id {
		return core.ErrRecoveryRequired
	}
	if err := record(id); err != nil {
		return err
	}
	status, err := b.waitCopyOperation(ctx, id)
	if err != nil {
		return err
	}
	if status != 200 {
		return core.ErrRecoveryRequired
	}
	return nil
}

// Incus volume copies may not support cancellation. Waiting for a terminal
// operation (including failure) is the deletion fence; a lost reply is not.
func (b *PersistentResourceBackend) WaitCopyStopped(ctx context.Context, target core.PersistentResource) error {
	if !target.CopyCleanup || target.CopyCompleted || target.Kind != CacheResourceKind || !target.SourceOnly || !core.ValidGenerationResource(target.Ref()) || target.CopySource != target.Producer || !core.ValidEnvironmentResourceRef(target.Producer) {
		return core.ErrInvalidArgument
	}
	_, err := b.waitCopyOperation(ctx, target.CopyOperation)
	return err
}

func (b *PersistentResourceBackend) waitCopyOperation(ctx context.Context, id string) (int, error) {
	if !copyOperationID.MatchString(id) {
		return 0, core.ErrInvalidArgument
	}
	for {
		r, err := b.Runtime.runner.Run(ctx, "incus", "query", "--raw", "/1.0/operations/"+id+"/wait?timeout=1&project="+b.Runtime.project)
		var response copyOperationResponse
		if err != nil || r.ExitCode != 0 || r.StdoutTruncated || json.Unmarshal([]byte(r.Stdout), &response) != nil || response.Type != "sync" || response.StatusCode != 200 || response.ErrorCode != 0 || response.Metadata.ID != id {
			return 0, core.ErrRecoveryRequired
		}
		switch response.Metadata.StatusCode {
		case 200, 400, 401:
			return response.Metadata.StatusCode, nil
		case 100, 103:
		default:
			return 0, core.ErrRecoveryRequired
		}
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}
