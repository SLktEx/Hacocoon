//go:build linux

package incus

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/lxc/incus/v6/shared/api"
)

// WithImportedRootfs owns a temporary native image for a synchronous consumer.
// The consumer must create an independent instance with current explicit config;
// it must not retain the transport image as a Base or another saved component.
func (r *Runtime) WithImportedRootfs(ctx context.Context, source io.ReadSeeker, root string, limit int64, consume func(string) error) error {
	connect, err := localImageExportConnect(r.project)
	if err != nil {
		return err
	}
	return r.withImportedRootfs(ctx, source, root, limit, consume, connect)
}

func (r *Runtime) withImportedRootfs(ctx context.Context, source io.ReadSeeker, root string, limit int64, consume func(string) error, connect imageExportConnect) (resultErr error) {
	if source == nil || consume == nil || connect == nil {
		return core.ErrInvalidArgument
	}
	server, err := connect(ctx)
	if err != nil {
		return err
	}
	defer server.Disconnect()
	info, err := server.GetConnectionInfo()
	if err != nil {
		return err
	}
	if info.Project != r.project || !filepath.IsAbs(info.SocketPath) {
		return core.ErrUnsupported
	}
	journal, err := newImageTransferJournal(root, r.project, "archive", info.SocketPath, "import")
	if err != nil {
		return err
	}
	defer journal.close()
	attempted, complete := false, false
	fingerprint := ""
	defer func() {
		if !attempted {
			resultErr = errors.Join(resultErr, journal.finish())
			return
		}
		if !complete {
			return
		} // The native task may still create an image.
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), r.cleanupTimeout)
		defer cancel()
		cleanupServer, cleanupErr := connect(cleanup)
		if cleanupErr == nil {
			cleanupErr = removeOwnedTransferImage(cleanup, cleanupServer, fingerprint, importImageOwnerKey, journal.owner)
			cleanupServer.Disconnect()
		}
		if cleanupErr == nil {
			cleanupErr = journal.finish()
		}
		if cleanupErr != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("rootfs import cleanup unconfirmed; receipt %s: %w", journal.path(), core.ErrRecoveryRequired), cleanupErr)
		}
	}()
	archive, err := prepareRootfsImport(ctx, source, root, limit, journal.owner)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, archive.Close()) }()
	fingerprint = archive.Digest()
	if err := journal.record(map[string]string{"fingerprint": fingerprint}); err != nil {
		return err
	}
	// Never adopt or relabel an existing image, even with a matching digest.
	if _, _, err := server.GetImage(fingerprint); !api.StatusErrorCheck(err, 404) {
		if err == nil {
			return core.ErrAlreadyExists
		}
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	attempted = true
	// SDK 6.0.5 CreateImage's upload request lacks its connection context.
	// RawOperation streams the same native endpoint with that context retained.
	op, _, err := server.RawOperation("POST", "/images", archive.Reader(), "")
	if err != nil {
		return journal.unconfirmed(err)
	}
	if op == nil {
		return journal.unconfirmed(core.ErrIncompatibleState)
	}
	if err := journal.record(map[string]string{"operation": op.Get().ID}); err != nil {
		return journal.unconfirmed(err)
	}
	if err := waitExportOperation(ctx, op); err != nil {
		return journal.unconfirmed(err)
	}
	observed, _ := op.Get().Metadata["fingerprint"].(string)
	if observed != fingerprint {
		return journal.unconfirmed(core.ErrIncompatibleState)
	}
	complete = true
	if err := verifyOwnedTransferImage(server, fingerprint, importImageOwnerKey, journal.owner); err != nil {
		return err
	}
	return consume(fingerprint)
}
