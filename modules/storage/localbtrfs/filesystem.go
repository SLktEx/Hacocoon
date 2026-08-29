package localbtrfs

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/host"
)

type FilesystemState struct {
	Healthy      bool
	LogicalBytes int64
	UsedBytes    int64
}

type Filesystem interface {
	Probe(context.Context) error
	Ensure(context.Context, string, string) error
	Inspect(context.Context, string) (FilesystemState, error)
	Grow(context.Context, string) error
	MinimumSize(context.Context, string) (int64, error)
	Compact(context.Context, string) error
	Shrink(context.Context, string, int64) error
	Unmount(context.Context, string) error
	Mount(context.Context, string, string) error
	Verify(context.Context, string, int64) error
}

type Btrfs struct {
	runner host.Runner
}

func NewBtrfs(runner host.Runner) *Btrfs { return &Btrfs{runner: runner} }

func (b *Btrfs) Probe(ctx context.Context) error {
	_, err := b.runner.Run(ctx, "btrfs", "version")
	return err
}

func (b *Btrfs) Ensure(ctx context.Context, device, mountpoint string) error {
	if err := os.MkdirAll(mountpoint, 0o700); err != nil {
		return err
	}
	fsType, _ := b.runner.Run(ctx, "blkid", "-o", "value", "-s", "TYPE", device)
	if strings.TrimSpace(fsType.Stdout) != "btrfs" {
		if _, err := b.runner.Run(ctx, "mkfs.btrfs", "-f", device); err != nil {
			return fmt.Errorf("format btrfs: %w", err)
		}
	}
	return b.Mount(ctx, device, mountpoint)
}

func (b *Btrfs) Inspect(ctx context.Context, mountpoint string) (FilesystemState, error) {
	result, err := b.runner.Run(ctx, "btrfs", "filesystem", "usage", "-b", mountpoint)
	if err != nil {
		return FilesystemState{}, err
	}
	logical := parseFirstInt(result.Stdout, `(?m)^\s*Device size:\s*([0-9]+)`)
	used := parseFirstInt(result.Stdout, `(?m)^\s*Used:\s*([0-9]+)`)
	return FilesystemState{Healthy: logical > 0, LogicalBytes: logical, UsedBytes: used}, nil
}

func (b *Btrfs) Grow(ctx context.Context, mountpoint string) error {
	_, err := b.runner.Run(ctx, "btrfs", "filesystem", "resize", "max", mountpoint)
	return err
}

func (b *Btrfs) MinimumSize(ctx context.Context, mountpoint string) (int64, error) {
	result, err := b.runner.Run(ctx, "btrfs", "inspect-internal", "min-dev-size", mountpoint)
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(result.Stdout)
	for i := len(fields) - 1; i >= 0; i-- {
		value, parseErr := strconv.ParseInt(strings.TrimSpace(fields[i]), 10, 64)
		if parseErr == nil {
			return value, nil
		}
	}
	return 0, fmt.Errorf("cannot parse btrfs minimum size: %q", strings.TrimSpace(result.Stdout))
}

func (b *Btrfs) Compact(ctx context.Context, mountpoint string) error {
	filters := []string{"usage=25", "usage=50", "usage=75"}
	for _, filter := range filters {
		_, err := b.runner.Run(ctx, "btrfs", "balance", "start", "-d"+filter, "-m"+filter, mountpoint)
		if err != nil {
			return fmt.Errorf("targeted btrfs balance %s: %w", filter, err)
		}
	}
	_, _ = b.runner.Run(ctx, "fstrim", mountpoint)
	return nil
}

func (b *Btrfs) Shrink(ctx context.Context, mountpoint string, target int64) error {
	_, err := b.runner.Run(ctx, "btrfs", "filesystem", "resize", strconv.FormatInt(target, 10), mountpoint)
	return err
}

func (b *Btrfs) Unmount(ctx context.Context, mountpoint string) error {
	_, err := b.runner.Run(ctx, "umount", mountpoint)
	return err
}

func (b *Btrfs) Mount(ctx context.Context, device, mountpoint string) error {
	mounted, _ := b.runner.Run(ctx, "findmnt", "-rn", mountpoint)
	if strings.TrimSpace(mounted.Stdout) != "" {
		return nil
	}
	_, err := b.runner.Run(ctx, "mount", device, mountpoint)
	return err
}

func (b *Btrfs) Verify(ctx context.Context, mountpoint string, expected int64) error {
	state, err := b.Inspect(ctx, mountpoint)
	if err != nil {
		return err
	}
	if !state.Healthy {
		return fmt.Errorf("btrfs verification failed: unhealthy filesystem")
	}
	if expected > 0 && state.LogicalBytes > expected+(64<<20) {
		return fmt.Errorf("btrfs verification failed: logical size %d exceeds target %d", state.LogicalBytes, expected)
	}
	return nil
}

func parseFirstInt(input, pattern string) int64 {
	match := regexp.MustCompile(pattern).FindStringSubmatch(input)
	if len(match) != 2 {
		return 0
	}
	value, _ := strconv.ParseInt(match[1], 10, 64)
	return value
}
