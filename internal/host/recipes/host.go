package recipes

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/host"
)

// HostResult is private command output, never an application log or audit field.
// Running also means interrupted/unknown after controller loss, not permission to retry.
type HostResult struct {
	Instance  string      `json:"instance"`
	Digest    string      `json:"digest"`
	State     string      `json:"state"`
	Execution host.Result `json:"execution"`
}

func (r HostResult) Valid() bool {
	if r.State == "none" {
		return r.Instance == "" && r.Digest == "" && r.Execution == (host.Result{})
	}
	if r.State != "running" && r.State != "succeeded" && r.State != "failed" {
		return false
	}
	if len(r.Instance) != 36 || len(r.Digest) != 64 {
		return false
	}
	if r.Instance[8] != '-' || r.Instance[13] != '-' || r.Instance[18] != '-' || r.Instance[23] != '-' || strings.Count(r.Instance, "-") != 4 {
		return false
	}
	if _, err := hex.DecodeString(strings.ReplaceAll(r.Instance, "-", "")); err != nil {
		return false
	}
	if _, err := hex.DecodeString(r.Digest); err != nil {
		return false
	}
	return len(r.Execution.Stdout) <= MaxScriptBytes/4 && len(r.Execution.Stderr) <= MaxScriptBytes/4 &&
		r.Execution.ExitCode >= -1 && r.Execution.ExitCode <= 255 && (r.State != "succeeded" || r.Execution.ExitCode == 0)
}

type resultObserverKey struct{}

func ObserveHostResult(ctx context.Context, report func(HostResult)) context.Context {
	return context.WithValue(ctx, resultObserverKey{}, report)
}
func reportHostResult(ctx context.Context, result HostResult) {
	if report, ok := ctx.Value(resultObserverKey{}).(func(HostResult)); ok {
		report(result)
	}
}

// HostService owns automatic application per provider incarnation. Project
// recipes retain their explicit replay semantics in Service.
type HostService struct {
	Root     string
	Identity func(context.Context) (string, error)
	Execute  func(context.Context, []byte) (host.Result, error)
}

func (s *HostService) Apply(ctx context.Context, update Update) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("%w: inspect haco setup --script-result; retry with --reapply-script", ErrExecutionFailed)
		}
	}()
	if err := update.Validate(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.Identity == nil || s.Execute == nil {
		return ErrExecutionFailed
	}
	files, err := openStore(s.Root)
	if err != nil {
		return err
	}
	defer files.close()
	var result HostResult
	data, err := files.readFile("result.json")
	if err != nil {
		return err
	}
	if data != nil {
		if err := json.Unmarshal(data, &result); err != nil || !result.Valid() {
			return ErrExecutionFailed
		}
	}
	if update.ResultOnly {
		if data == nil {
			result = HostResult{State: "none"}
		}
		reportHostResult(ctx, result)
		return nil
	}
	if update.Clear {
		return files.remove()
	}
	if update.Script != nil {
		script := strings.ReplaceAll(strings.TrimPrefix(*update.Script, "\ufeff"), "\r\n", "\n")
		if err := files.save([]byte(script)); err != nil {
			return err
		}
	}
	script, err := files.read()
	if err != nil {
		return err
	}
	if script == nil {
		if update.Reapply {
			return ErrExecutionFailed
		}
		return nil
	}
	instance, err := s.Identity(ctx)
	if err != nil {
		return err
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(script))
	explicit := update.Script != nil || update.Reapply
	if !explicit && data != nil && result.Instance == instance {
		reportHostResult(ctx, result)
		if result.State != "succeeded" || result.Digest != digest {
			return ErrExecutionFailed
		}
		return nil
	}
	result = HostResult{Instance: instance, Digest: digest, State: "running", Execution: host.Result{ExitCode: -1}}
	if !result.Valid() {
		return ErrExecutionFailed
	}
	save := func() error {
		b, err := json.Marshal(result)
		if err != nil {
			return err
		}
		return files.saveFile("result.json", b)
	}
	// Durable intent precedes execution. Unknown completion is never auto-replayed.
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := save(); err != nil {
		return err
	}
	execution, executeErr := s.Execute(ctx, script)
	result.Execution, result.State = execution, "failed"
	current, identityErr := s.Identity(ctx)
	if executeErr == nil && ctx.Err() == nil && identityErr == nil && current == instance && execution.ExitCode == 0 {
		result.State = "succeeded"
	}
	if !result.Valid() {
		return ErrExecutionFailed
	}
	if err := save(); err != nil {
		return err
	}
	reportHostResult(ctx, result)
	if result.State != "succeeded" {
		return ErrExecutionFailed
	}
	return nil
}
