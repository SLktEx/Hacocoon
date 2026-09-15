package main

import (
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
	target := func(ctx context.Context) (reclamation.WSLTarget, error) {
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
	}
	for i, arg := range args {
		if arg == "--review" {
			reviewArgs := append([]string{}, args[:i]...)
			reviewArgs = append(reviewArgs, args[i+1:]...)
			return reclaimReviewCommand(ctx, reviewArgs, os.Stdin, os.Stdout, os.Stderr, target, reclaimclient.InvokeWindows, reclaimclient.ReviewWindows)
		}
	}
	return reclaimCommand(ctx, args, os.Stdin, os.Stdout, os.Stderr, target, reclaimclient.InvokeWindows)
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
			_, _ = fmt.Fprintln(out, cliLanguage().Text("reclaim.text.help"))
			return 0
		case "--status":
			if mode != "start" || yes {
				_, _ = fmt.Fprintln(diagnostic, cliLanguage().Text("reclaim.text.usage"))
				return 2
			}
			mode = "status"
		case "--yes":
			if yes || mode != "start" {
				_, _ = fmt.Fprintln(diagnostic, cliLanguage().Text("reclaim.text.usage"))
				return 2
			}
			yes = true
		default:
			_, _ = fmt.Fprintln(diagnostic, cliLanguage().Text("reclaim.text.usage"))
			return 2
		}
	}
	selected, err := target(ctx)
	if err != nil || selected.Validate() != nil {
		_, _ = fmt.Fprintln(diagnostic, cliLanguage().Text("reclaim.text.unavailable"))
		return 1
	}
	if mode == "start" {
		if _, err := fmt.Fprintln(out, cliLanguage().Text("reclaim.text.stop_warning")); err != nil {
			return 1
		}
		if !yes {
			confirmed, err := confirmReclamation(ctx, in, out)
			if err != nil {
				_, _ = fmt.Fprintln(diagnostic, cliLanguage().Text("reclaim.text.confirmation_unavailable"))
				return 1
			}
			if !confirmed {
				_, _ = fmt.Fprintln(out, cliLanguage().Text("reclaim.text.canceled"))
				return 0
			}
		}
	}
	if ctx.Err() != nil {
		_, _ = fmt.Fprintln(diagnostic, cliLanguage().Text("reclaim.text.canceled_before"))
		return 1
	}
	raw, err := invoke(ctx, selected, mode)
	if err != nil {
		writeReclaimInvocationFailure(diagnostic, err)
		_, _ = fmt.Fprintln(diagnostic, cliLanguage().Text("reclaim.text.unconfirmed"))
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
		_, _ = fmt.Fprintln(diagnostic, cliLanguage().Text("reclaim.text.dispatch_unknown"))
		return 1
	}
	_, _ = fmt.Fprintf(out, cliLanguage().Text("reclaim.text.operation"), started.Operation)
	_, _ = fmt.Fprintln(out, cliLanguage().Text("reclaim.text.dispatched"))
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
		Failure                      string
		NativeError                  uint32
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

func parseReclamationStatus(raw []byte) (windowsReclaimStatus, error) {
	var result windowsReclaimStatus
	if err := decodeReclaimOutput(raw, &result); err != nil {
		return result, errors.New(cliLanguage().Text("reclaim.text.invalid_saved"))
	}
	if result.State == "none" {
		if result.Operation != "" || result.LinuxStarted || result.Linux != nil || result.Observation != nil {
			return result, errors.New(cliLanguage().Text("reclaim.text.invalid_absent"))
		}
		return result, nil
	}
	if !validReclaimOperation(result.Operation) || (result.State != "pending" && result.State != "failed" && result.State != "complete" && result.State != "interrupted") || (result.Linux != nil && result.Linux.Validate() != nil) {
		return result, errors.New(cliLanguage().Text("reclaim.text.invalid_saved"))
	}
	if result.Linux != nil && !result.LinuxStarted {
		return result, errors.New(cliLanguage().Text("reclaim.text.invalid_linux"))
	}
	if (result.State == "pending" || result.State == "interrupted") && result.Observation != nil {
		return result, errors.New(cliLanguage().Text("reclaim.text.invalid_pending"))
	}
	if result.Observation != nil {
		o := result.Observation
		switch o.Failure {
		case "":
			if o.NativeError != 0 {
				return result, errors.New(cliLanguage().Text("reclaim.text.invalid_native"))
			}
		case "stop", "compact", "compact_attached", "resume":
			if result.State != "failed" {
				return result, errors.New(cliLanguage().Text("reclaim.text.invalid_failed"))
			}
		default:
			return result, errors.New(cliLanguage().Text("reclaim.text.invalid_stage"))
		}
		if (o.StopRequested && !o.StopAttempted) || (o.Resumed && !o.ResumeAttempted) || (o.Compaction.Completed && !o.Compaction.Attempted) || (o.Compaction.Attempted && !o.StopRequested) || o.Compaction.OpenAttempts < 0 {
			return result, errors.New(cliLanguage().Text("reclaim.text.invalid_observations"))
		}
	}
	if result.State == "complete" && (result.Observation == nil || !result.Observation.StopRequested || !result.Observation.Compaction.Completed || !result.Observation.Resumed || result.Observation.Compaction.Virtual.Capacity == 0 || result.Observation.Compaction.Before.LogicalBytes == 0 || result.Observation.Compaction.After.LogicalBytes == 0 || (result.LinuxStarted && (result.Linux == nil || !result.Linux.Complete()))) {
		return result, errors.New(cliLanguage().Text("reclaim.text.invalid_complete"))
	}
	return result, nil
}

func writeReclamationStatus(out, diagnostic io.Writer, raw []byte) int {
	result, err := parseReclamationStatus(raw)
	if err != nil {
		_, _ = fmt.Fprintln(diagnostic, err)
		return 1
	}
	if result.State == "none" {
		_, err := fmt.Fprintln(out, cliLanguage().Text("reclaim.text.none"))
		if err != nil {
			return 1
		}
		return 0
	}
	if result.State == "complete" && result.Linux == nil {
		_, _ = fmt.Fprintln(out, cliLanguage().Text("reclaim.text.windows_only"))
	} else {
		_, _ = fmt.Fprintf(out, cliLanguage().Text("reclaim.text.state"), reclamationValue(result.State))
	}
	if result.Linux == nil {
		_, _ = fmt.Fprintln(out, cliLanguage().Text("reclaim.text.linux_unknown"))
	} else {
		for _, entry := range []struct {
			name  string
			stage reclamation.Stage
		}{{"Incus Btrfs", result.Linux.Pool}, {"WSL ext4", result.Linux.Outer}} {
			_, _ = fmt.Fprintf(out, "%s: %s\n", entry.name, reclamationValue(entry.stage.Status))
			if a, b := entry.stage.FilesystemBefore, entry.stage.FilesystemAfter; a != nil && b != nil {
				_, _ = fmt.Fprintf(out, cliLanguage().Text("reclaim.text.filesystem"), a.CapacityBytes, a.UsedBytes, b.UsedBytes)
			}
			if a, b := entry.stage.Before, entry.stage.After; a != nil && b != nil {
				_, _ = fmt.Fprintf(out, cliLanguage().Text("reclaim.text.file_capacity"), a.LogicalBytes, a.AllocatedBytes, b.AllocatedBytes)
			}
			if n := entry.stage.KernelTrimmedBytes; n != nil {
				_, _ = fmt.Fprintf(out, cliLanguage().Text("reclaim.text.discard"), *n)
			}
		}
		if result.Linux.Failure != "" {
			_, _ = fmt.Fprintf(out, cliLanguage().Text("reclaim.text.linux_failure"), reclamationValue(result.Linux.Failure))
		}
	}
	if result.Observation == nil {
		_, _ = fmt.Fprintln(out, cliLanguage().Text("reclaim.text.windows_unknown"))
	} else {
		o := result.Observation
		_, _ = fmt.Fprintf(out, cliLanguage().Text("reclaim.text.windows_state"), reclamationFlag(o.StopRequested), reclamationFlag(o.Compaction.Completed), reclamationFlag(o.Resumed))
		if o.Failure != "" {
			_, _ = fmt.Fprintf(out, cliLanguage().Text("reclaim.text.failure_stage"), reclamationValue(o.Failure))
			if o.Failure == "compact_attached" {
				_, _ = fmt.Fprintln(out, cliMessage("reclaim.attached"))
			}
		}
		if o.NativeError != 0 {
			_, _ = fmt.Fprintf(out, cliLanguage().Text("reclaim.text.native_code"), o.NativeError)
		}
		c := o.Compaction
		if c.Virtual.Capacity > 0 {
			_, _ = fmt.Fprintf(out, cliLanguage().Text("reclaim.text.virtual_capacity"), c.Virtual.Capacity)
		}
		if c.Before.LogicalBytes > 0 && c.After.LogicalBytes > 0 {
			_, _ = fmt.Fprintf(out, cliLanguage().Text("reclaim.text.allocated"), c.Before.AllocatedBytes, c.After.AllocatedBytes)
			if c.Before.AllocatedBytes >= c.After.AllocatedBytes {
				_, _ = fmt.Fprintf(out, cliLanguage().Text("reclaim.text.recovered"), c.Before.AllocatedBytes-c.After.AllocatedBytes)
			} else {
				_, _ = fmt.Fprintf(out, cliLanguage().Text("reclaim.text.increased"), c.After.AllocatedBytes-c.Before.AllocatedBytes)
			}
		}
	}
	if result.State == "pending" {
		_, _ = fmt.Fprintln(out, cliLanguage().Text("reclaim.text.pending"))
	}
	if result.State == "interrupted" {
		_, _ = fmt.Fprintln(out, cliLanguage().Text("reclaim.text.interrupted"))
	}
	if result.State == "failed" || result.State == "interrupted" {
		return 1
	}
	return 0
}
