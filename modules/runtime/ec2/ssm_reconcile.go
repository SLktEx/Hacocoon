package ec2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func (r *Runtime) reconcileSSMCommand(ctx context.Context, instanceID, commandID string) (core.ExecutionResult, error) {
	attempts := r.pollAttempts
	if attempts <= 0 {
		attempts = 1
	}

	lastStatus := "unobserved"
	var lastObservationErr error
	for attempt := 0; attempt < attempts; attempt++ {
		got, err := r.aws(ctx,
			"ssm", "get-command-invocation",
			"--command-id", commandID,
			"--instance-id", instanceID,
			"--output", "json",
		)
		if err == nil {
			var invocation commandInvocation
			if decodeErr := json.Unmarshal([]byte(got.Stdout), &invocation); decodeErr != nil {
				lastObservationErr = fmt.Errorf("decode SSM invocation: %w", decodeErr)
			} else {
				lastStatus = strings.TrimSpace(invocation.Status)
				switch lastStatus {
				case "Pending", "InProgress", "Delayed", "Cancelling", "":
					lastObservationErr = nil
				case "Success", "Failed":
					if invocation.ResponseCode < 0 {
						return core.ExecutionResult{}, fmt.Errorf(
							"SSM command %s reached %s without a trustworthy response code: %w",
							commandID,
							lastStatus,
							core.ErrExecutionOutcomeUnknown,
						)
					}
					return core.ExecutionResult{
						ExitCode: invocation.ResponseCode,
						Stdout:   invocation.Stdout,
						Stderr:   invocation.Stderr,
					}, nil
				case "Cancelled", "TimedOut":
					return core.ExecutionResult{}, fmt.Errorf(
						"SSM command %s terminal status %s does not prove whether side effects occurred: %w",
						commandID,
						lastStatus,
						core.ErrExecutionOutcomeUnknown,
					)
				default:
					return core.ExecutionResult{}, fmt.Errorf(
						"SSM command %s returned unknown status %q: %w",
						commandID,
						lastStatus,
						core.ErrExecutionOutcomeUnknown,
					)
				}
			}
		} else {
			lastObservationErr = err
		}

		if ctx.Err() != nil {
			return core.ExecutionResult{}, fmt.Errorf(
				"SSM command %s was accepted but observation stopped: %w",
				commandID,
				errors.Join(ctx.Err(), core.ErrExecutionOutcomeUnknown),
			)
		}
		if attempt+1 < attempts && r.pollDelay > 0 {
			timer := time.NewTimer(r.pollDelay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return core.ExecutionResult{}, fmt.Errorf(
					"SSM command %s was accepted but observation stopped: %w",
					commandID,
					errors.Join(ctx.Err(), core.ErrExecutionOutcomeUnknown),
				)
			case <-timer.C:
			}
		}
	}

	var cause error = core.ErrExecutionOutcomeUnknown
	if lastObservationErr != nil {
		cause = errors.Join(lastObservationErr, core.ErrExecutionOutcomeUnknown)
	}
	return core.ExecutionResult{}, fmt.Errorf(
		"SSM command %s outcome unresolved after %d observation attempts (last status %q): %w",
		commandID,
		attempts,
		lastStatus,
		cause,
	)
}
