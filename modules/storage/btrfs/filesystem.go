package btrfs

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/host"
)

const defaultCompressionOption = "compress=zstd:3"
const defaultMountOptions = defaultCompressionOption + ",noatime,nodiscard"

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
	fsType, probeErr := b.runner.Run(ctx, "blkid", "-o", "value", "-s", "TYPE", device)
	filesystemType := strings.TrimSpace(fsType.Stdout)
	switch {
	case probeErr == nil && filesystemType == "btrfs":
		// Existing managed filesystem.
	case probeErr == nil && filesystemType != "":
		return fmt.Errorf("refuse to format %s: existing filesystem type is %q", device, filesystemType)
	case probeErr != nil && fsType.ExitCode != 2:
		return fmt.Errorf("probe filesystem on %s before formatting: %w", device, probeErr)
	case probeErr == nil && filesystemType == "":
		return fmt.Errorf("refuse to format %s: blkid returned no type without the expected not-found status", device)
	default:
		// blkid exit 2 means no identifiable filesystem/signature. Only this
		// explicit state is allowed to reach destructive formatting.
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
	mounted, err := b.runner.Run(ctx, "findmnt", "-rn", "--mountpoint", mountpoint)
	if err != nil {
		if mounted.ExitCode == 1 {
			return nil
		}
		return fmt.Errorf("inspect mountpoint %s before unmount: %w", mountpoint, err)
	}
	if strings.TrimSpace(mounted.Stdout) == "" {
		return nil
	}
	_, err = b.runner.Run(ctx, "umount", mountpoint)
	return err
}

func (b *Btrfs) Mount(ctx context.Context, device, mountpoint string) error {
	mounted, err := b.runner.Run(ctx, "findmnt", "-rn", "-o", "SOURCE", "--mountpoint", mountpoint)
	if err == nil && strings.TrimSpace(mounted.Stdout) != "" {
		source := strings.TrimSpace(mounted.Stdout)
		if source != device {
			return fmt.Errorf("mountpoint %s is already backed by %s, expected %s", mountpoint, source, device)
		}

		options, optionsErr := b.runner.Run(ctx, "findmnt", "-rn", "-o", "OPTIONS", "--mountpoint", mountpoint)
		if optionsErr != nil {
			return fmt.Errorf("inspect mount options for %s: %w", mountpoint, optionsErr)
		}
		if hasExpectedMountOptions(options.Stdout) {
			return nil
		}

		_, err = b.runner.Run(ctx, "mount", device, mountpoint, "-o", "remount,"+defaultMountOptions)
		if err != nil {
			return fmt.Errorf("remount %s with %s: %w", mountpoint, defaultMountOptions, err)
		}
		return nil
	}
	if err != nil && mounted.ExitCode != 1 {
		return fmt.Errorf("inspect mountpoint %s before mount: %w", mountpoint, err)
	}
	_, err = b.runner.Run(ctx, "mount", device, mountpoint, "-o", defaultMountOptions)
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

func hasExpectedMountOptions(options string) bool {
	hasCompression := false
	hasNoatime := false
	hasNodiscard := false
	for _, option := range strings.Split(strings.TrimSpace(options), ",") {
		option = strings.TrimSpace(option)
		switch {
		case option == "compress-force" || strings.HasPrefix(option, "compress-force="):
			return false
		case option == "compress=zstd" || option == defaultCompressionOption:
			hasCompression = true
		case option == "noatime":
			hasNoatime = true
		case option == "nodiscard":
			hasNodiscard = true
		case option == "discard" || strings.HasPrefix(option, "discard="):
			return false
		case option == "atime" || option == "strictatime" || option == "relatime":
			return false
		}
	}
	return hasCompression && hasNoatime && hasNodiscard
}

func parseFirstInt(input, pattern string) int64 {
	match := regexp.MustCompile(pattern).FindStringSubmatch(input)
	if len(match) != 2 {
		return 0
	}
	value, _ := strconv.ParseInt(match[1], 10, 64)
	return value
}
