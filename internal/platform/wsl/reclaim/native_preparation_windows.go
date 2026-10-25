//go:build windows && (amd64 || arm64)

package wslreclaim

import (
	"context"
	"errors"

	"github.com/SLktEx/Hacocoon/internal/storage/reclamation"
)

// PrepareNativeWorker is the fresh Windows-native helper entry. Unlike the
// Host-origin preparation, it also discovers the controller's exact target.
// Pre-dispatch discovery requires successful fixed reads and empty private
// Windows jobs before intent; this does not prove Linux-side teardown.
// It never launches or adopts a worker, retries discovery, or reviews old state.
func PrepareNativeWorker(ctx context.Context, registrationID string) (PreparedStatus, error) {
	if err := ctx.Err(); err != nil {
		return PreparedStatus{}, err
	}
	r, err := readRegistration(registrationID)
	if err != nil {
		return PreparedStatus{}, targetStage("registration", err)
	}
	var intent operationRecord
	err = r.withEnrolledReclamationTarget(ctx, func(records *operationStore, pin *pinnedDisk, enrolled installationObservation) error {
		target := reclamation.WSLTarget{RegistrationID: enrolled.Installation.RegistrationID, InstallationID: enrolled.Installation.InstallationID}
		return prepareNativeDiscovery(ctx, target, records.refusePendingPreparation,
			func(ctx context.Context) (reclamation.WSLTarget, error) {
				identity, err := r.readOwnedInstallation(ctx)
				return reclamation.WSLTarget{RegistrationID: identity.RegistrationID, InstallationID: identity.InstallationID}, err
			},
			func(ctx context.Context) (reclamation.WSLTarget, error) {
				return r.readOwnedControllerTarget(ctx, target)
			},
			func() error {
				// Fixed discovery succeeded and both private Windows jobs are empty.
				// Recheck native identity before canonical version-2 persistence.
				if err := r.revalidate(); err != nil {
					return err
				}
				if _, err := pin.Allocation(); err != nil {
					return err
				}
				if err := records.requireBinding(enrolled); err != nil {
					return err
				}
				var err error
				intent, err = records.beginVersion(r, pin.identity, 2)
				return err
			})
	})
	if err != nil {
		return PreparedStatus{}, err
	}
	if intent.State != "pending" || intent.Version != 2 {
		return PreparedStatus{}, errors.New("fresh native preparation did not produce a pending intent")
	}
	// Guard release above precedes the separate, unchanged worker reacquisition.
	return PreparedStatus{Operation: intent.Operation.String(), State: intent.State}, nil
}
