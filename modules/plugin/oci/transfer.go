package oci

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"regexp"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const MaxTransferBytes int64 = 256 << 20

var transferImagePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/:@-]{0,255}$`)

type TransferRequest struct {
	Environment string `json:"environment"`
	Driver      Driver `json:"driver"`
	Image       string `json:"image"`
}
type TransferReport struct {
	Environment   string `json:"environment"`
	Driver        Driver `json:"driver"`
	Image         string `json:"image"`
	ArchiveSHA256 string `json:"archive_sha256"`
	Bytes         int64  `json:"bytes"`
}
type TransferBackend interface {
	SaveImage(context.Context, Driver, string, io.Writer) error
	LoadImage(context.Context, core.Environment, Driver, io.Reader) error
}
type TransferEnvironments interface {
	GetEnvironment(context.Context, string) (core.Environment, error)
}
type TransferService struct {
	Backend      TransferBackend
	Environments TransferEnvironments
}

func (s *TransferService) Distribute(parent context.Context, req TransferRequest) (TransferReport, error) {
	if (req.Driver != DriverDocker && req.Driver != DriverNerdctl) || !transferImagePattern.MatchString(req.Image) {
		return TransferReport{}, core.ErrInvalidArgument
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Minute)
	defer cancel()
	env, err := s.Environments.GetEnvironment(ctx, req.Environment)
	if err != nil {
		return TransferReport{}, err
	}
	file, err := os.CreateTemp("", "haco-oci-*.tar")
	if err != nil {
		return TransferReport{}, err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	sum := sha256.New()
	bounded := &transferWriter{Writer: io.MultiWriter(file, sum), remaining: MaxTransferBytes}
	if err = s.Backend.SaveImage(ctx, req.Driver, req.Image, bounded); err != nil {
		return TransferReport{}, fmt.Errorf("save trusted Host image; check installed runtime and image: %w", err)
	}
	size := MaxTransferBytes - bounded.remaining
	if size == 0 {
		return TransferReport{}, core.ErrIncompatibleState
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return TransferReport{}, err
	}
	// Re-read the authoritative Environment before import; never import into a
	// replacement which appeared while the source was being saved.
	current, err := s.Environments.GetEnvironment(ctx, req.Environment)
	if err != nil || current.CreatedAt != env.CreatedAt || current.RuntimeRef != env.RuntimeRef || current.Workspace != env.Workspace {
		return TransferReport{}, core.ErrCapabilityStale
	}
	if err = s.Backend.LoadImage(ctx, env, req.Driver, file); err != nil {
		return TransferReport{}, fmt.Errorf("load Environment image; check its local runtime and nesting setup: %w", err)
	}
	return TransferReport{Environment: env.Name, Driver: req.Driver, Image: req.Image, ArchiveSHA256: fmt.Sprintf("%x", sum.Sum(nil)), Bytes: size}, nil
}

type transferWriter struct {
	io.Writer
	remaining int64
}

func (w *transferWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > w.remaining {
		return 0, fmt.Errorf("image archive exceeds %d bytes: %w", MaxTransferBytes, core.ErrInvalidArgument)
	}
	n, e := w.Writer.Write(p)
	w.remaining -= int64(n)
	return n, e
}
