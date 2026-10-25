package wslreclaim

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf16"

	"github.com/SLktEx/Hacocoon/internal/storage/reclamation"
)

var nativeDistributionName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,255}$`)

const maxNativeRegistrations = 1024

type nativeRegistrationName struct{ id, name string }

// A name selects exactly one candidate, never grants disk authority. Native
// enrollment, owner and pinned-file checks must follow this read-only selection.
func selectNativeRegistration(name string, candidates []nativeRegistrationName) (string, error) {
	if !nativeDistributionName.MatchString(name) {
		return "", errors.New("an explicit managed WSL distribution name is required")
	}
	if len(candidates) > maxNativeRegistrations {
		return "", errors.New("WSL registration enumeration exceeded limit")
	}
	var selected string
	for _, candidate := range candidates {
		if !strings.EqualFold(name, candidate.name) {
			continue
		}
		if selected != "" || candidate.id == "" {
			return "", errors.New("WSL distribution name is ambiguous")
		}
		selected = candidate.id
	}
	if selected == "" {
		return "", errors.New("named WSL distribution is not registered")
	}
	return selected, nil
}

// DistributionName is a bounded literal REG_SZ, never an expandable string.
func decodeNativeDistributionName(raw []byte) (string, error) {
	if len(raw) < 4 || len(raw) > 514 || len(raw)%2 != 0 {
		return "", errors.New("invalid WSL distribution name encoding")
	}
	units := make([]uint16, len(raw)/2)
	for i := range units {
		units[i] = uint16(raw[2*i]) | uint16(raw[2*i+1])<<8
		if units[i] == 0 && i != len(units)-1 {
			return "", errors.New("ambiguous WSL distribution name")
		}
	}
	if units[len(units)-1] != 0 {
		return "", errors.New("unterminated WSL distribution name")
	}
	return string(utf16.Decode(units[:len(units)-1])), nil
}

var errNativeDiscoveryCleanup = errors.New("private reclamation discovery cleanup not confirmed")

// A successful response is not sufficient: callers keep the concrete owner
// even when the RPC transport has already called Close and discarded its error.
// Private.Close is idempotent and preserves its Windows-job absence observation.
// It does not establish teardown of WSL-service-created Linux processes.
func finishNativeDiscovery(ctx context.Context, operationErr error, close func() error) error {
	cleanupErr := close()
	if cleanupErr != nil {
		cleanupErr = fmt.Errorf("%w: %w", errNativeDiscoveryCleanup, cleanupErr)
	}
	return errors.Join(operationErr, ctx.Err(), cleanupErr)
}

func closePreparationReservation(operationErr error, close func() error) error {
	if errors.Is(operationErr, errNativeDiscoveryCleanup) {
		return operationErr
	}
	return errors.Join(operationErr, close())
}

// This is the fresh native entry's entire read-before-intent order. The caller
// holds native guards and pins; each reader must confirm its owned Windows job
// empty before returning. No worker launch or old-operation selection belongs here.
func prepareNativeDiscovery(ctx context.Context, enrolled reclamation.WSLTarget,
	preflight func() error,
	installation, controller func(context.Context) (reclamation.WSLTarget, error),
	persist func() error,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := enrolled.Validate(); err != nil {
		return targetStage("binding", err)
	}
	if err := preflight(); err != nil {
		return targetStage("intent", err)
	}
	for _, read := range []func(context.Context) (reclamation.WSLTarget, error){installation, controller} {
		if err := ctx.Err(); err != nil {
			return err
		}
		observed, err := read(ctx)
		if err != nil {
			return targetStage("installation", err)
		}
		if observed.Validate() != nil || observed != enrolled {
			return targetStage("binding", errors.New("discovered installation differs from native enrollment"))
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return targetStage("intent", persist())
}
