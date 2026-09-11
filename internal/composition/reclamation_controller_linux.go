//go:build linux && (amd64 || arm64)

package composition

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/internal/reclamation"
	"github.com/SLktEx/Hacocoon/modules/runtime/incus"
)

type linuxReclaimTarget interface {
	TrimPool(context.Context) (incus.PoolTrimObservation, error)
	TrimBackingFilesystem(context.Context) (incus.OuterTrimObservation, error)
	Close() error
}

// ReclaimLinux is available only on the controller management endpoint. The
// caller requests both layers for the exact installed WSL; no path/pool input.
func (a *App) ReclaimLinux(ctx context.Context, expected reclamation.WSLTarget) reclamation.LinuxReport {
	return reclaimLinux(ctx, expected, func(ctx context.Context) (reclamation.WSLTarget, error) {
		return readReclaimInstallation(ctx, host.ExecRunner{})
	}, func(ctx context.Context) (linuxReclaimTarget, error) {
		if a == nil || a.Runtime == nil {
			return nil, errors.New("Incus runtime unavailable")
		}
		return a.PrepareStorageReclamation(ctx)
	})
}

// ReclamationTarget lets a trusted client discover this installation without
// asking its user for GUIDs. This observation is not Windows mutation authority.
func (a *App) ReclamationTarget(ctx context.Context) (reclamation.WSLTarget, error) {
	if a == nil || a.Runtime == nil {
		return reclamation.WSLTarget{}, errors.New("Incus runtime unavailable")
	}
	if err := ctx.Err(); err != nil {
		return reclamation.WSLTarget{}, err
	}
	return readReclaimInstallation(ctx, host.ExecRunner{})
}

func readReclaimInstallation(ctx context.Context, runner host.Runner) (reclamation.WSLTarget, error) {
	result, err := runner.Run(ctx, "/usr/bin/env", "-i", "PATH=/usr/sbin:/usr/bin:/sbin:/bin", "/usr/bin/python3", "-I", "/usr/local/libexec/hacocoon-wsl-interop", "--read-registration")
	if err != nil || result.ExitCode != 0 || result.StdoutTruncated || result.StderrTruncated || len(result.Stdout) > 4096 {
		return reclamation.WSLTarget{}, errors.New("managed WSL identity unavailable")
	}
	var record struct {
		SchemaVersion int `json:"schema_version"`
		reclamation.WSLTarget
	}
	decoder := json.NewDecoder(bytes.NewBufferString(result.Stdout))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&record) != nil || record.SchemaVersion != 1 || record.WSLTarget.Validate() != nil {
		return reclamation.WSLTarget{}, errors.New("invalid managed WSL identity")
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return reclamation.WSLTarget{}, errors.New("trailing managed WSL identity")
	}
	return record.WSLTarget, nil
}

func reclaimLinux(ctx context.Context, expected reclamation.WSLTarget,
	read func(context.Context) (reclamation.WSLTarget, error),
	selectPool func(context.Context) (linuxReclaimTarget, error)) (report reclamation.LinuxReport) {
	report = reclamation.NotStarted("identity_unavailable")
	if expected.Validate() != nil {
		return
	}
	if ctx.Err() != nil {
		return reclamation.NotStarted("canceled")
	}
	actual, err := read(ctx)
	if err != nil {
		return
	}
	if actual != expected {
		return reclamation.NotStarted("identity_changed")
	}
	target, err := selectPool(ctx)
	if err != nil || target == nil {
		return reclamation.NotStarted("pool_unavailable")
	}
	defer func() {
		if target.Close() != nil {
			report.CleanupFailed = true
			if report.Failure == "" {
				report.Failure = "cleanup_failed"
			}
		}
	}()
	pool, err := target.TrimPool(ctx)
	report.Pool = reclamation.Stage{Status: "failed", Attempted: pool.Attempted, FilesystemBefore: reclaimFilesystemProjection(pool.FilesystemBefore), FilesystemAfter: reclaimFilesystemProjection(pool.FilesystemAfter)}
	if pool.Before.LogicalBytes > 0 {
		report.Pool.Before = &reclamation.Allocation{LogicalBytes: pool.Before.LogicalBytes, AllocatedBytes: pool.Before.AllocatedBytes}
	}
	if pool.After.LogicalBytes > 0 {
		report.Pool.After = &reclamation.Allocation{LogicalBytes: pool.After.LogicalBytes, AllocatedBytes: pool.After.AllocatedBytes}
	}
	if pool.KernelReportKnown {
		report.Pool.KernelTrimmedBytes = &pool.KernelTrimmedBytes
	}
	if err != nil {
		report.Failure = "pool_trim_failed"
		return
	}
	report.Pool.Status = "complete"
	// Re-read the root-owned installation before authorizing the outer filesystem.
	actual, err = read(ctx)
	if err != nil {
		report.Failure = "identity_unavailable"
		return
	}
	if actual != expected {
		report.Failure = "identity_changed"
		return
	}
	if ctx.Err() != nil {
		report.Failure = "canceled"
		return
	}
	outer, err := target.TrimBackingFilesystem(ctx)
	report.Outer = reclamation.Stage{Status: "failed", Attempted: outer.Attempted, FilesystemBefore: reclaimFilesystemProjection(outer.FilesystemBefore), FilesystemAfter: reclaimFilesystemProjection(outer.FilesystemAfter)}
	if outer.KernelReportKnown {
		report.Outer.KernelTrimmedBytes = &outer.KernelTrimmedBytes
	}
	if err != nil {
		report.Failure = "outer_trim_failed"
		return
	}
	report.Outer.Status = "complete"
	report.Failure = ""
	return
}

func reclaimFilesystemProjection(f *incus.ReclaimFilesystemUsage) *reclamation.FilesystemUsage {
	if f == nil {
		return nil
	}
	return &reclamation.FilesystemUsage{CapacityBytes: f.CapacityBytes, UsedBytes: f.UsedBytes}
}
