package ec2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

var createOperationIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

func clientTokenForCreateOperation(operationID string) (string, error) {
	if !createOperationIDPattern.MatchString(operationID) {
		return "", fmt.Errorf("invalid EC2 create operation identity: %w", core.ErrInvalidArgument)
	}
	// 37 ASCII bytes, safely below EC2's 64-byte ClientToken limit.
	return "haco-" + operationID, nil
}

type clientTokenInstance struct {
	InstanceID string `json:"InstanceID"`
	State      string `json:"State"`
}

func (r *Runtime) reconcileRunInstances(ctx context.Context, clientToken, expectedAccount string) (string, error) {
	attempts := r.pollAttempts
	if attempts <= 0 {
		attempts = 1
	}
	var lastErr error
	for i := 0; i < attempts; i++ {
		accountID, authorityErr := r.resolveAccountID(ctx)
		if authorityErr != nil || accountID != expectedAccount {
			return "", errors.Join(
				fmt.Errorf("cannot reconcile EC2 create token %q because AWS account identity changed or is unavailable (expected %s, observed %q): %w", clientToken, expectedAccount, accountID, authorityErr),
				core.ErrRecoveryRequired,
			)
		}

		result, err := r.aws(ctx,
			"ec2", "describe-instances",
			"--filters", "Name=client-token,Values="+clientToken,
			"--query", "Reservations[].Instances[].{InstanceID:InstanceId,State:State.Name}",
			"--output", "json",
		)
		if err != nil {
			lastErr = fmt.Errorf("observe EC2 create token %q: %w", clientToken, err)
		} else {
			var instances []clientTokenInstance
			if err := json.Unmarshal([]byte(result.Stdout), &instances); err != nil {
				lastErr = fmt.Errorf("decode EC2 create-token reconciliation: %w", err)
			} else {
				live := make([]clientTokenInstance, 0, len(instances))
				for _, instance := range instances {
					instance.InstanceID = strings.TrimSpace(instance.InstanceID)
					instance.State = strings.TrimSpace(instance.State)
					if !validInstanceID(instance.InstanceID) {
						return "", errors.Join(
							fmt.Errorf("EC2 create-token reconciliation returned invalid instance id %q: %w", instance.InstanceID, core.ErrIncompatibleState),
							core.ErrRecoveryRequired,
						)
					}
					if instance.State == "terminated" {
						continue
					}
					live = append(live, instance)
				}
				switch len(live) {
				case 0:
					if len(instances) > 0 {
						return "", errors.Join(
							fmt.Errorf("EC2 create token %q resolves only to terminated instances", clientToken),
							core.ErrRecoveryRequired,
						)
					}
					lastErr = fmt.Errorf("EC2 create token %q is not visible yet", clientToken)
				case 1:
					return live[0].InstanceID, nil
				default:
					return "", errors.Join(
						fmt.Errorf("EC2 create token %q resolves to %d live instances: %w", clientToken, len(live), core.ErrIncompatibleState),
						core.ErrRecoveryRequired,
					)
				}
			}
		}
		if i+1 < attempts {
			if err := r.waitPollRetry(ctx); err != nil {
				return "", errors.Join(err, core.ErrRecoveryRequired)
			}
		}
	}
	if lastErr == nil {
		lastErr = core.ErrRuntimeUnavailable
	}
	return "", errors.Join(lastErr, core.ErrRecoveryRequired)
}
