package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/reclaimclient"
	"github.com/SLktEx/Hacocoon/internal/reclamation"
)

func runReclaim(args []string) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 8*time.Minute)
	defer cancel()
	return reclaimCommand(ctx, args, os.Stdin, os.Stdout, os.Stderr, func(ctx context.Context) (reclamation.WSLTarget, error) {
		client, err := controlapi.NewDefaultClient()
		if err != nil {
			return reclamation.WSLTarget{}, err
		}
		ready, cancel := context.WithTimeout(ctx, controllerStartupTimeout)
		defer cancel()
		if err := waitForController(ready, func(ctx context.Context) error { _, err := client.Ping(ctx); return err }); err != nil {
			return reclamation.WSLTarget{}, err
		}
		return client.ReclamationTarget(ctx)
	}, reclaimclient.InvokeWindows)
}

func reclaimCommand(ctx context.Context, args []string, in io.Reader, out, diagnostic io.Writer,
	target func(context.Context) (reclamation.WSLTarget, error),
	invoke func(context.Context, reclamation.WSLTarget, string) ([]byte, error),
) int {
	mode := "start"
	yes := false
	for _, arg := range args {
		switch arg {
		case "--help", "-h":
			fmt.Fprintln(out, "Usage: haco reclaim [--yes | --status]\nReclaim unused disk space in this managed WSL. Running sessions disconnect.\nUse --status after reopening Hacocoon to inspect the saved result.")
			return 0
		case "--status":
			if mode != "start" || yes {
				fmt.Fprintln(diagnostic, "Usage: haco reclaim [--yes | --status]")
				return 2
			}
			mode = "status"
		case "--yes":
			if yes || mode != "start" {
				fmt.Fprintln(diagnostic, "Usage: haco reclaim [--yes | --status]")
				return 2
			}
			yes = true
		default:
			fmt.Fprintln(diagnostic, "Usage: haco reclaim [--yes | --status]")
			return 2
		}
	}
	selected, err := target(ctx)
	if err != nil || selected.Validate() != nil {
		fmt.Fprintln(diagnostic, "Managed Windows installation unavailable. Run from its trusted Hacocoon Host.")
		return 1
	}
	if mode == "start" {
		if _, err := fmt.Fprintln(out, "Hacocoon will stop and restart. Save active work first; files and disk capacity are retained."); err != nil {
			return 1
		}
		if !yes {
			if _, err := fmt.Fprint(out, "Continue? [y/N] "); err != nil {
				return 1
			}
			answers := make(chan bool, 1)
			go func() {
				answer, err := bufio.NewReader(io.LimitReader(in, 128)).ReadString('\n')
				answers <- err == nil && (strings.EqualFold(strings.TrimSpace(answer), "y") || strings.EqualFold(strings.TrimSpace(answer), "yes"))
			}()
			confirmed := false
			select {
			case confirmed = <-answers:
			case <-ctx.Done():
			}
			if !confirmed {
				fmt.Fprintln(out, "Canceled.")
				return 0
			}
		}
	}
	if ctx.Err() != nil {
		fmt.Fprintln(diagnostic, "Reclamation canceled before Windows invocation.")
		return 1
	}
	raw, err := invoke(ctx, selected, mode)
	if err != nil {
		fmt.Fprintln(diagnostic, "Windows reclamation could not be confirmed. Inspect haco reclaim --status; do not assume a failed dispatch means no work started.")
		return 1
	}
	if mode == "status" {
		return writeReclamationStatus(out, diagnostic, raw)
	}
	var started struct {
		Operation string `json:"operation"`
		PID       int    `json:"worker_pid"`
	}
	if decodeReclaimOutput(raw, &started) != nil || !validReclaimOperation(started.Operation) || (started.PID < 1 || uint64(started.PID) > 0xffffffff) {
		fmt.Fprintln(diagnostic, "Dispatch result unavailable. Inspect haco reclaim --status; the operation may already be running.")
		return 1
	}
	fmt.Fprintf(out, "Operation: %s\n", started.Operation)
	fmt.Fprintln(out, "Worker dispatched; reclamation is not yet confirmed. Reopen Hacocoon after restart and run haco reclaim --status.")
	return 0
}

func validReclaimOperation(id string) bool {
	return (reclamation.WSLTarget{RegistrationID: strings.ToLower(id), InstallationID: "11111111-1111-4111-8111-111111111111"}).Validate() == nil
}
func decodeReclaimOutput(raw []byte, target any) error {
	if len(raw) == 0 || len(raw) > 16384 {
		return errors.New("invalid Windows result size")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return errors.New("trailing Windows result")
	}
	return nil
}

// Read-only wire projection. These observations never select a native resource.
type windowsReclaimStatus struct {
	LinuxStarted bool                     `json:"linux_started,omitempty"`
	Linux        *reclamation.LinuxReport `json:"linux,omitempty"`
	Operation    string                   `json:"operation"`
	State        string                   `json:"state"`
	Observation  *struct {
		StopAttempted, StopRequested bool
		Compaction                   struct {
			Before, After struct{ LogicalBytes, AllocatedBytes uint64 }
			Virtual       struct {
				Capacity   uint64
				Identifier [16]byte
			}
			Attempted, Completed bool
			OpenAttempts         int
		}
		ResumeAttempted, Resumed bool
	} `json:"observation,omitempty"`
}

func writeReclamationStatus(out, diagnostic io.Writer, raw []byte) int {
	var result windowsReclaimStatus
	if decodeReclaimOutput(raw, &result) != nil || !validReclaimOperation(result.Operation) || (result.State != "pending" && result.State != "failed" && result.State != "complete") || (result.Linux != nil && result.Linux.Validate() != nil) {
		fmt.Fprintln(diagnostic, "Saved reclamation result is invalid; retain it for inspection.")
		return 1
	}
	if result.Linux != nil && !result.LinuxStarted {
		fmt.Fprintln(diagnostic, "Saved Linux result has no recorded attempt.")
		return 1
	}
	if result.State == "pending" && result.Observation != nil {
		fmt.Fprintln(diagnostic, "Pending Windows result has unconfirmed observations.")
		return 1
	}
	if result.Observation != nil {
		o := result.Observation
		if (o.StopRequested && !o.StopAttempted) || (o.Resumed && !o.ResumeAttempted) || (o.Compaction.Completed && !o.Compaction.Attempted) || (o.Compaction.Attempted && !o.StopRequested) || o.Compaction.OpenAttempts < 0 {
			fmt.Fprintln(diagnostic, "Saved Windows observations are inconsistent.")
			return 1
		}
	}
	if result.State == "complete" && (result.Observation == nil || !result.Observation.StopRequested || !result.Observation.Compaction.Completed || !result.Observation.Resumed || result.Observation.Compaction.Virtual.Capacity == 0 || result.Observation.Compaction.Before.LogicalBytes == 0 || result.Observation.Compaction.After.LogicalBytes == 0 || (result.LinuxStarted && (result.Linux == nil || !result.Linux.Complete()))) {
		fmt.Fprintln(diagnostic, "Saved reclamation completion is unproven.")
		return 1
	}
	if result.State == "complete" && result.Linux == nil {
		fmt.Fprintln(out, "Saved Windows-only operation: complete")
	} else {
		fmt.Fprintf(out, "Reclamation: %s\n", result.State)
	}
	if result.Linux == nil {
		fmt.Fprintln(out, "Linux stages: unrecorded / outcome unknown")
	} else {
		for _, entry := range []struct {
			name  string
			stage reclamation.Stage
		}{{"Incus Btrfs", result.Linux.Pool}, {"WSL ext4", result.Linux.Outer}} {
			fmt.Fprintf(out, "%s: %s\n", entry.name, entry.stage.Status)
			if a, b := entry.stage.FilesystemBefore, entry.stage.FilesystemAfter; a != nil && b != nil {
				fmt.Fprintf(out, "  filesystem capacity: %d bytes; used: %d -> %d bytes\n", a.CapacityBytes, a.UsedBytes, b.UsedBytes)
			}
			if a, b := entry.stage.Before, entry.stage.After; a != nil && b != nil {
				fmt.Fprintf(out, "  file capacity: %d bytes; allocated: %d -> %d bytes\n", a.LogicalBytes, a.AllocatedBytes, b.AllocatedBytes)
			}
			if n := entry.stage.KernelTrimmedBytes; n != nil {
				fmt.Fprintf(out, "  kernel discard: %d bytes (not Windows recovered space)\n", *n)
			}
		}
		if result.Linux.Failure != "" {
			fmt.Fprintf(out, "Linux failure: %s\n", result.Linux.Failure)
		}
	}
	if result.Observation == nil {
		fmt.Fprintln(out, "Windows stage: outcome unknown")
	} else {
		o := result.Observation
		fmt.Fprintf(out, "Windows: stop requested=%t, compaction complete=%t, resumed=%t\n", o.StopRequested, o.Compaction.Completed, o.Resumed)
		c := o.Compaction
		if c.Virtual.Capacity > 0 {
			fmt.Fprintf(out, "  virtual capacity: %d bytes\n", c.Virtual.Capacity)
		}
		if c.Before.LogicalBytes > 0 && c.After.LogicalBytes > 0 {
			fmt.Fprintf(out, "  allocated: %d -> %d bytes\n", c.Before.AllocatedBytes, c.After.AllocatedBytes)
			if c.Before.AllocatedBytes >= c.After.AllocatedBytes {
				fmt.Fprintf(out, "  reclaimed: %d bytes\n", c.Before.AllocatedBytes-c.After.AllocatedBytes)
			} else {
				fmt.Fprintf(out, "  allocation increased: %d bytes\n", c.After.AllocatedBytes-c.Before.AllocatedBytes)
			}
		}
	}
	if result.State == "pending" {
		fmt.Fprintln(out, "Completion is unknown. This command does not retry or clear the operation.")
	}
	if result.State == "failed" {
		return 1
	}
	return 0
}
