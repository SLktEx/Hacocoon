package ec2

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func (r *Runtime) resolveAccountID(ctx context.Context) (string, error) {
	result, err := r.aws(ctx,
		"sts", "get-caller-identity",
		"--query", "Account",
		"--output", "text",
	)
	if err != nil {
		return "", fmt.Errorf("resolve AWS caller account: %w", err)
	}
	accountID := strings.TrimSpace(result.Stdout)
	if !awsAccountIDPattern.MatchString(accountID) {
		return "", fmt.Errorf("invalid AWS caller account id %q: %w", accountID, core.ErrIncompatibleState)
	}
	return accountID, nil
}

func (r *Runtime) authorizeRuntimeRef(ctx context.Context, ref runtimeRef) error {
	cfg, err := r.config.normalized()
	if err != nil {
		return err
	}
	if cfg.Region != ref.Region {
		return errors.Join(
			fmt.Errorf("EC2 runtime authority region changed from %s to %s", ref.Region, cfg.Region),
			core.ErrCapabilityStale,
			core.ErrRecoveryRequired,
		)
	}
	accountID, err := r.resolveAccountID(ctx)
	if err != nil {
		return errors.Join(
			fmt.Errorf("cannot prove EC2 runtime AWS account %s: %w", ref.AccountID, err),
			core.ErrRecoveryRequired,
		)
	}
	if accountID != ref.AccountID {
		return errors.Join(
			fmt.Errorf("EC2 runtime authority account changed from %s to %s", ref.AccountID, accountID),
			core.ErrCapabilityStale,
			core.ErrRecoveryRequired,
		)
	}
	return nil
}
