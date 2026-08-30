package ebs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const operationVersion = 2

type Phase string

const (
	PhasePlanning         Phase = "planning"
	PhaseTargetCreated    Phase = "target-created"
	PhaseTargetAttached   Phase = "target-attached"
	PhaseMigrated         Phase = "migrated"
	PhaseVerified         Phase = "verified"
	PhaseSourceDetached   Phase = "source-detached"
	PhaseTargetPromoted   Phase = "target-promoted"
	PhaseActivated        Phase = "activated"
	PhaseComplete         Phase = "complete"
	PhaseRecoveryRequired Phase = "recovery-required"
)

var phaseOrder = map[Phase]int{
	PhasePlanning:       0,
	PhaseTargetCreated:  1,
	PhaseTargetAttached: 2,
	PhaseMigrated:       3,
	PhaseVerified:       4,
	PhaseSourceDetached: 5,
	PhaseTargetPromoted: 6,
	PhaseActivated:      7,
	PhaseComplete:       8,
}

type VolumeAttachment struct {
	InstanceID string `json:"instance_id"`
	Device     string `json:"device"`
	State      string `json:"state"`
}

type Volume struct {
	ID               string             `json:"id"`
	SizeGiB          int64              `json:"size_gib"`
	AvailabilityZone string             `json:"availability_zone"`
	Type             string             `json:"type"`
	Encrypted        bool               `json:"encrypted"`
	KMSKeyID         string             `json:"kms_key_id,omitempty"`
	State            string             `json:"state,omitempty"`
	Attachments      []VolumeAttachment `json:"attachments,omitempty"`
}

type ReplacementSpec struct {
	OperationID    string
	InstanceID     string
	SourceVolumeID string
	SourceDevice   string
	StagingDevice  string
	TargetSizeGiB  int64
}

type Operation struct {
	Version        int    `json:"version"`
	ID             string `json:"id"`
	InstanceID     string `json:"instance_id"`
	SourceVolumeID string `json:"source_volume_id"`
	TargetVolumeID string `json:"target_volume_id,omitempty"`
	SourceDevice   string `json:"source_device"`
	StagingDevice  string `json:"staging_device"`
	TargetSizeGiB  int64  `json:"target_size_gib"`
	Phase          Phase  `json:"phase"`
	RecoveryFrom   Phase  `json:"recovery_from,omitempty"`
}

type API interface {
	DescribeVolume(context.Context, string) (Volume, error)
	CreateVolume(context.Context, Volume, int64, string) (string, error)
	AttachVolume(context.Context, string, string, string) error
	DetachVolume(context.Context, string, string) error
	WaitAvailable(context.Context, string) error
	WaitInUse(context.Context, string) error
	DeleteVolume(context.Context, string) error
}

type Migrator interface {
	Preflight(context.Context, ReplacementSpec, Volume) error
	Migrate(context.Context, ReplacementSpec, string) error
	Verify(context.Context, ReplacementSpec, string) error
	Activate(context.Context, ReplacementSpec, string) error
}

type Journal interface {
	Save(context.Context, Operation) error
	Load(context.Context, string) (Operation, bool, error)
	List(context.Context) ([]Operation, error)
	Delete(context.Context, string) error
	Lock(context.Context, ...string) (func() error, error)
}

type Service struct {
	api      API
	migrator Migrator
	journal  Journal
}

func New(api API, migrator Migrator, journal Journal) *Service {
	return &Service{api: api, migrator: migrator, journal: journal}
}

func (s *Service) Replace(ctx context.Context, spec ReplacementSpec) (Operation, error) {
	if err := s.validate(spec); err != nil {
		return Operation{}, err
	}
	release, err := s.journal.Lock(ctx, replacementLockKeys(spec)...)
	if err != nil {
		return Operation{}, fmt.Errorf("lock EBS replacement: %w", err)
	}
	defer release()

	if conflict, found, err := s.findConflict(ctx, spec); err != nil {
		return Operation{}, err
	} else if found {
		return conflict, fmt.Errorf("EBS replacement %s already owns source/device: %w", conflict.ID, core.ErrStorageBusy)
	}

	existing, found, err := s.journal.Load(ctx, spec.OperationID)
	if err != nil {
		return Operation{}, fmt.Errorf("load EBS replacement journal: %w", err)
	}
	if found {
		return s.resumeLocked(ctx, spec, existing)
	}
	return s.startLocked(ctx, spec)
}

func (s *Service) Resume(ctx context.Context, spec ReplacementSpec) (Operation, error) {
	if err := s.validate(spec); err != nil {
		return Operation{}, err
	}
	release, err := s.journal.Lock(ctx, replacementLockKeys(spec)...)
	if err != nil {
		return Operation{}, fmt.Errorf("lock EBS replacement recovery: %w", err)
	}
	defer release()

	op, found, err := s.journal.Load(ctx, spec.OperationID)
	if err != nil {
		return Operation{}, fmt.Errorf("load EBS replacement journal: %w", err)
	}
	if !found {
		return Operation{}, core.ErrNotFound
	}
	return s.resumeLocked(ctx, spec, op)
}

func (s *Service) startLocked(ctx context.Context, spec ReplacementSpec) (Operation, error) {
	source, err := s.api.DescribeVolume(ctx, spec.SourceVolumeID)
	if err != nil {
		return Operation{}, fmt.Errorf("describe source EBS volume: %w", err)
	}
	if spec.TargetSizeGiB >= source.SizeGiB {
		return Operation{}, fmt.Errorf("target %d GiB must be smaller than source %d GiB: %w", spec.TargetSizeGiB, source.SizeGiB, core.ErrInvalidArgument)
	}
	if err := s.migrator.Preflight(ctx, spec, source); err != nil {
		return Operation{}, fmt.Errorf("EBS replacement preflight: %w", err)
	}

	op := Operation{
		Version:        operationVersion,
		ID:             spec.OperationID,
		InstanceID:     spec.InstanceID,
		SourceVolumeID: spec.SourceVolumeID,
		SourceDevice:   spec.SourceDevice,
		StagingDevice:  spec.StagingDevice,
		TargetSizeGiB:  spec.TargetSizeGiB,
		Phase:          PhasePlanning,
	}
	if err := s.save(ctx, op); err != nil {
		return op, err
	}
	return s.continueLocked(ctx, spec, source, op)
}

func (s *Service) resumeLocked(ctx context.Context, spec ReplacementSpec, op Operation) (Operation, error) {
	if err := operationMatchesSpec(op, spec); err != nil {
		return op, err
	}
	if op.Phase == PhaseComplete {
		if err := s.journal.Delete(ctx, op.ID); err != nil {
			return s.recoveryRequired(ctx, op, fmt.Errorf("remove completed EBS journal: %w", err))
		}
		return op, nil
	}
	if op.Phase == PhaseRecoveryRequired && op.RecoveryFrom == "" {
		return op, errors.Join(fmt.Errorf("legacy recovery journal lacks a safe resume phase"), core.ErrRecoveryRequired)
	}

	reconciled, source, err := s.reconcile(ctx, spec, op)
	if err != nil {
		return s.recoveryRequired(ctx, op, fmt.Errorf("reconcile EBS replacement: %w", err))
	}
	if reconciled != op {
		if err := s.save(ctx, reconciled); err != nil {
			return s.recoveryRequired(ctx, reconciled, err)
		}
	}
	return s.continueLocked(ctx, spec, source, reconciled)
}

func (s *Service) continueLocked(ctx context.Context, spec ReplacementSpec, source Volume, op Operation) (Operation, error) {
	for {
		switch op.Phase {
		case PhasePlanning:
			target, err := s.api.CreateVolume(ctx, source, spec.TargetSizeGiB, clientTokenForOperation(spec.OperationID))
			if err != nil {
				return op, fmt.Errorf("create replacement EBS volume: %w", err)
			}
			op.TargetVolumeID = target
			op.Phase = PhaseTargetCreated
			op.RecoveryFrom = ""
			if err := s.save(ctx, op); err != nil {
				return s.cleanupCreatedTarget(ctx, op, err)
			}

		case PhaseTargetCreated:
			if err := s.api.WaitAvailable(ctx, op.TargetVolumeID); err != nil {
				return s.recoveryRequired(ctx, op, fmt.Errorf("replacement EBS volume readiness is ambiguous: %w", err))
			}
			if err := s.api.AttachVolume(ctx, op.TargetVolumeID, spec.InstanceID, spec.StagingDevice); err != nil {
				return s.recoveryRequired(ctx, op, fmt.Errorf("attach replacement EBS volume became ambiguous: %w", err))
			}
			if err := s.api.WaitInUse(ctx, op.TargetVolumeID); err != nil {
				return s.recoveryRequired(ctx, op, fmt.Errorf("wait replacement EBS attachment: %w", err))
			}
			op.Phase = PhaseTargetAttached
			if err := s.save(ctx, op); err != nil {
				return s.cleanupBeforeDetach(ctx, op, err)
			}

		case PhaseTargetAttached:
			if err := s.ensureTargetStaging(ctx, spec, op.TargetVolumeID); err != nil {
				return s.recoveryRequired(ctx, op, err)
			}
			if err := s.migrator.Migrate(ctx, spec, op.TargetVolumeID); err != nil {
				return s.cleanupBeforeDetach(ctx, op, fmt.Errorf("migrate EBS data: %w", err))
			}
			op.Phase = PhaseMigrated
			if err := s.save(ctx, op); err != nil {
				return s.cleanupBeforeDetach(ctx, op, err)
			}

		case PhaseMigrated:
			if err := s.ensureTargetStaging(ctx, spec, op.TargetVolumeID); err != nil {
				return s.recoveryRequired(ctx, op, err)
			}
			if err := s.migrator.Verify(ctx, spec, op.TargetVolumeID); err != nil {
				return s.cleanupBeforeDetach(ctx, op, fmt.Errorf("verify EBS migration: %w", err))
			}
			op.Phase = PhaseVerified
			if err := s.save(ctx, op); err != nil {
				return s.cleanupBeforeDetach(ctx, op, err)
			}

		case PhaseVerified:
			if err := s.ensureTargetStaging(ctx, spec, op.TargetVolumeID); err != nil {
				return s.recoveryRequired(ctx, op, err)
			}
			if err := s.api.DetachVolume(ctx, spec.SourceVolumeID, spec.InstanceID); err != nil {
				return s.recoveryRequired(ctx, op, fmt.Errorf("detach source EBS volume became ambiguous: %w", err))
			}
			if err := s.api.WaitAvailable(ctx, spec.SourceVolumeID); err != nil {
				return s.recoveryRequired(ctx, op, fmt.Errorf("source EBS detach became ambiguous: %w", err))
			}
			op.Phase = PhaseSourceDetached
			if err := s.save(ctx, op); err != nil {
				return s.recoveryRequired(ctx, op, err)
			}

		case PhaseSourceDetached:
			if err := s.promoteTarget(ctx, spec, op.TargetVolumeID); err != nil {
				return s.recoveryRequired(ctx, op, err)
			}
			op.Phase = PhaseTargetPromoted
			if err := s.save(ctx, op); err != nil {
				return s.recoveryRequired(ctx, op, err)
			}

		case PhaseTargetPromoted:
			if err := s.migrator.Activate(ctx, spec, op.TargetVolumeID); err != nil {
				return s.recoveryRequired(ctx, op, fmt.Errorf("activate replacement EBS volume: %w", err))
			}
			op.Phase = PhaseActivated
			if err := s.save(ctx, op); err != nil {
				return s.recoveryRequired(ctx, op, err)
			}

		case PhaseActivated:
			op.Phase = PhaseComplete
			if err := s.save(ctx, op); err != nil {
				return s.recoveryRequired(ctx, op, err)
			}

		case PhaseComplete:
			if err := s.journal.Delete(ctx, op.ID); err != nil {
				return s.recoveryRequired(ctx, op, fmt.Errorf("remove completed EBS journal: %w", err))
			}
			return op, nil

		default:
			return s.recoveryRequired(ctx, op, fmt.Errorf("cannot continue EBS phase %q: %w", op.Phase, core.ErrIncompatibleState))
		}
	}
}

func (s *Service) reconcile(ctx context.Context, spec ReplacementSpec, op Operation) (Operation, Volume, error) {
	logical := op.Phase
	if logical == PhaseRecoveryRequired {
		logical = op.RecoveryFrom
	}
	if _, ok := phaseOrder[logical]; !ok {
		return op, Volume{}, core.ErrIncompatibleState
	}

	source, err := s.stabilizeVolume(ctx, op.SourceVolumeID)
	if err != nil {
		return op, Volume{}, fmt.Errorf("observe source EBS volume: %w", err)
	}
	sourceDevice, sourceAttached, err := stablePlacement(source, spec.InstanceID)
	if err != nil {
		return op, Volume{}, fmt.Errorf("source EBS attachment state: %w", err)
	}
	if sourceAttached && sourceDevice != spec.SourceDevice {
		return op, Volume{}, fmt.Errorf("source EBS attached at %s instead of %s: %w", sourceDevice, spec.SourceDevice, core.ErrIncompatibleState)
	}

	if op.TargetVolumeID == "" {
		if logical != PhasePlanning || !sourceAttached {
			return op, Volume{}, fmt.Errorf("journal has no replacement target in phase %s: %w", logical, core.ErrIncompatibleState)
		}
		op.Version = operationVersion
		op.Phase = PhasePlanning
		op.RecoveryFrom = ""
		return op, source, nil
	}

	target, err := s.stabilizeVolume(ctx, op.TargetVolumeID)
	if err != nil {
		return op, Volume{}, fmt.Errorf("observe target EBS volume: %w", err)
	}
	targetDevice, targetAttached, err := stablePlacement(target, spec.InstanceID)
	if err != nil {
		return op, Volume{}, fmt.Errorf("target EBS attachment state: %w", err)
	}
	if targetAttached && targetDevice != spec.StagingDevice && targetDevice != spec.SourceDevice {
		return op, Volume{}, fmt.Errorf("target EBS attached at unexpected device %s: %w", targetDevice, core.ErrIncompatibleState)
	}

	if sourceAttached {
		if phaseAtLeast(logical, PhaseSourceDetached) {
			return op, Volume{}, fmt.Errorf("journal says source was detached but AWS shows it attached: %w", core.ErrIncompatibleState)
		}
		if targetAttached && targetDevice == spec.SourceDevice {
			return op, Volume{}, fmt.Errorf("source and replacement target both occupy source attachment path: %w", core.ErrIncompatibleState)
		}
		if logical == PhaseTargetCreated && targetAttached && targetDevice == spec.StagingDevice {
			logical = PhaseTargetAttached
		}
	} else {
		if !phaseAtLeast(logical, PhaseVerified) {
			return op, Volume{}, fmt.Errorf("source detached before migration was durably verified: %w", core.ErrRecoveryRequired)
		}
		if targetAttached && targetDevice == spec.SourceDevice {
			logical = laterPhase(logical, PhaseTargetPromoted)
		} else {
			if phaseAtLeast(logical, PhaseTargetPromoted) {
				return op, Volume{}, fmt.Errorf("journal says target was promoted but AWS does not: %w", core.ErrIncompatibleState)
			}
			logical = laterPhase(logical, PhaseSourceDetached)
		}
	}

	op.Version = operationVersion
	op.Phase = logical
	op.RecoveryFrom = ""
	return op, source, nil
}

func (s *Service) stabilizeVolume(ctx context.Context, id string) (Volume, error) {
	for attempt := 0; attempt < 3; attempt++ {
		volume, err := s.api.DescribeVolume(ctx, id)
		if err != nil {
			return Volume{}, err
		}
		if volume.State == "creating" {
			if err := s.api.WaitAvailable(ctx, id); err != nil {
				return Volume{}, err
			}
			continue
		}
		transient := ""
		for _, attachment := range volume.Attachments {
			switch attachment.State {
			case "attaching":
				transient = "attaching"
			case "detaching", "busy":
				transient = "detaching"
			}
		}
		switch transient {
		case "attaching":
			if err := s.api.WaitInUse(ctx, id); err != nil {
				return Volume{}, err
			}
			continue
		case "detaching":
			if err := s.api.WaitAvailable(ctx, id); err != nil {
				return Volume{}, err
			}
			continue
		default:
			return volume, nil
		}
	}
	return Volume{}, fmt.Errorf("EBS volume %s did not reach a stable observable state: %w", id, core.ErrRecoveryRequired)
}

func stablePlacement(volume Volume, instanceID string) (string, bool, error) {
	var active *VolumeAttachment
	for i := range volume.Attachments {
		attachment := &volume.Attachments[i]
		if attachment.State == "detached" {
			continue
		}
		if attachment.State != "" && attachment.State != "attached" {
			return "", false, fmt.Errorf("attachment state %s is not stable: %w", attachment.State, core.ErrIncompatibleState)
		}
		if active != nil {
			return "", false, fmt.Errorf("multiple active EBS attachments: %w", core.ErrIncompatibleState)
		}
		active = attachment
	}
	if active == nil {
		if volume.State == "in-use" {
			return "", false, fmt.Errorf("volume is in-use without an active attachment: %w", core.ErrIncompatibleState)
		}
		return "", false, nil
	}
	if active.InstanceID != instanceID {
		return "", false, fmt.Errorf("volume attached to instance %s: %w", active.InstanceID, core.ErrIncompatibleState)
	}
	return active.Device, true, nil
}

func (s *Service) ensureTargetStaging(ctx context.Context, spec ReplacementSpec, targetID string) error {
	target, err := s.stabilizeVolume(ctx, targetID)
	if err != nil {
		return fmt.Errorf("observe replacement EBS staging attachment: %w", err)
	}
	device, attached, err := stablePlacement(target, spec.InstanceID)
	if err != nil {
		return err
	}
	if attached {
		if device != spec.StagingDevice {
			return fmt.Errorf("replacement EBS target attached at %s while staging is required: %w", device, core.ErrIncompatibleState)
		}
		return nil
	}
	if err := s.api.WaitAvailable(ctx, targetID); err != nil {
		return fmt.Errorf("wait replacement EBS target before staging attach: %w", err)
	}
	if err := s.api.AttachVolume(ctx, targetID, spec.InstanceID, spec.StagingDevice); err != nil {
		return fmt.Errorf("reattach replacement EBS staging volume: %w", err)
	}
	if err := s.api.WaitInUse(ctx, targetID); err != nil {
		return fmt.Errorf("wait replacement EBS staging reattach: %w", err)
	}
	return nil
}

func (s *Service) promoteTarget(ctx context.Context, spec ReplacementSpec, targetID string) error {
	target, err := s.stabilizeVolume(ctx, targetID)
	if err != nil {
		return fmt.Errorf("observe replacement EBS target before promotion: %w", err)
	}
	device, attached, err := stablePlacement(target, spec.InstanceID)
	if err != nil {
		return err
	}
	if attached && device == spec.SourceDevice {
		return nil
	}
	if attached {
		if device != spec.StagingDevice {
			return fmt.Errorf("replacement EBS target attached at %s before promotion: %w", device, core.ErrIncompatibleState)
		}
		if err := s.api.DetachVolume(ctx, targetID, spec.InstanceID); err != nil {
			return fmt.Errorf("detach replacement EBS staging attachment: %w", err)
		}
		if err := s.api.WaitAvailable(ctx, targetID); err != nil {
			return fmt.Errorf("wait replacement EBS staging detach: %w", err)
		}
	} else if err := s.api.WaitAvailable(ctx, targetID); err != nil {
		return fmt.Errorf("wait replacement EBS target before promotion: %w", err)
	}
	if err := s.api.AttachVolume(ctx, targetID, spec.InstanceID, spec.SourceDevice); err != nil {
		return fmt.Errorf("promote replacement EBS volume: %w", err)
	}
	if err := s.api.WaitInUse(ctx, targetID); err != nil {
		return fmt.Errorf("wait replacement EBS promotion: %w", err)
	}
	return nil
}

func (s *Service) cleanupCreatedTarget(ctx context.Context, op Operation, cause error) (Operation, error) {
	if op.TargetVolumeID == "" {
		return op, cause
	}
	cleanupCtx := context.WithoutCancel(ctx)
	if err := s.api.WaitAvailable(cleanupCtx, op.TargetVolumeID); err != nil {
		return s.recoveryRequired(ctx, op, errors.Join(cause, fmt.Errorf("wait replacement EBS volume during cleanup: %w", err)))
	}
	if err := s.api.DeleteVolume(cleanupCtx, op.TargetVolumeID); err != nil {
		return s.recoveryRequired(ctx, op, errors.Join(cause, fmt.Errorf("cleanup replacement EBS volume: %w", err)))
	}
	if err := s.journal.Delete(cleanupCtx, op.ID); err != nil {
		return s.recoveryRequired(ctx, op, errors.Join(cause, fmt.Errorf("remove cleaned EBS journal: %w", err)))
	}
	return op, cause
}

func (s *Service) cleanupBeforeDetach(ctx context.Context, op Operation, cause error) (Operation, error) {
	if op.TargetVolumeID == "" {
		return op, cause
	}
	cleanupCtx := context.WithoutCancel(ctx)
	if err := s.api.DetachVolume(cleanupCtx, op.TargetVolumeID, op.InstanceID); err != nil {
		return s.recoveryRequired(ctx, op, errors.Join(cause, fmt.Errorf("detach replacement EBS volume during cleanup: %w", err)))
	}
	if err := s.api.WaitAvailable(cleanupCtx, op.TargetVolumeID); err != nil {
		return s.recoveryRequired(ctx, op, errors.Join(cause, fmt.Errorf("wait replacement EBS cleanup detach: %w", err)))
	}
	if err := s.api.DeleteVolume(cleanupCtx, op.TargetVolumeID); err != nil {
		return s.recoveryRequired(ctx, op, errors.Join(cause, fmt.Errorf("cleanup replacement EBS volume: %w", err)))
	}
	if err := s.journal.Delete(cleanupCtx, op.ID); err != nil {
		return s.recoveryRequired(ctx, op, errors.Join(cause, fmt.Errorf("remove cleaned EBS journal: %w", err)))
	}
	return op, cause
}

func (s *Service) recoveryRequired(ctx context.Context, op Operation, cause error) (Operation, error) {
	if op.Phase != PhaseRecoveryRequired {
		op.RecoveryFrom = op.Phase
	}
	op.Version = operationVersion
	op.Phase = PhaseRecoveryRequired
	saveErr := s.journal.Save(context.WithoutCancel(ctx), op)
	return op, errors.Join(cause, saveErr, core.ErrRecoveryRequired)
}

func (s *Service) save(ctx context.Context, op Operation) error {
	if err := s.journal.Save(ctx, op); err != nil {
		return fmt.Errorf("persist EBS replacement phase %s: %w", op.Phase, err)
	}
	return nil
}

func (s *Service) findConflict(ctx context.Context, spec ReplacementSpec) (Operation, bool, error) {
	operations, err := s.journal.List(ctx)
	if err != nil {
		return Operation{}, false, fmt.Errorf("list EBS replacement journals: %w", err)
	}
	for _, op := range operations {
		if op.ID == spec.OperationID {
			continue
		}
		if op.SourceVolumeID == spec.SourceVolumeID || (op.InstanceID == spec.InstanceID && op.SourceDevice == spec.SourceDevice) {
			return op, true, nil
		}
	}
	return Operation{}, false, nil
}

func (s *Service) validate(spec ReplacementSpec) error {
	if s == nil || s.api == nil || s.migrator == nil || s.journal == nil {
		return core.ErrInvalidArgument
	}
	return validateSpec(spec)
}

func replacementLockKeys(spec ReplacementSpec) []string {
	return []string{
		"operation:" + spec.OperationID,
		"source:" + spec.SourceVolumeID,
		"device:" + spec.InstanceID + ":" + spec.SourceDevice,
	}
}

func operationMatchesSpec(op Operation, spec ReplacementSpec) error {
	if op.ID != spec.OperationID || op.InstanceID != spec.InstanceID || op.SourceVolumeID != spec.SourceVolumeID || op.SourceDevice != spec.SourceDevice || op.StagingDevice != spec.StagingDevice || op.TargetSizeGiB != spec.TargetSizeGiB {
		return fmt.Errorf("existing EBS replacement journal does not match request: %w", core.ErrIncompatibleState)
	}
	return nil
}

func phaseAtLeast(got, want Phase) bool {
	gotRank, gotOK := phaseOrder[got]
	wantRank, wantOK := phaseOrder[want]
	return gotOK && wantOK && gotRank >= wantRank
}

func laterPhase(a, b Phase) Phase {
	if phaseAtLeast(a, b) {
		return a
	}
	return b
}

func validPhase(phase Phase) bool {
	if phase == PhaseRecoveryRequired {
		return true
	}
	_, ok := phaseOrder[phase]
	return ok
}

func clientTokenForOperation(operationID string) string {
	sum := sha256.Sum256([]byte("hacocoon-ebs-replacement\x00" + operationID))
	return hex.EncodeToString(sum[:])
}

func validateSpec(spec ReplacementSpec) error {
	values := []string{spec.OperationID, spec.InstanceID, spec.SourceVolumeID, spec.SourceDevice, spec.StagingDevice}
	for _, value := range values {
		if strings.TrimSpace(value) == "" || strings.TrimSpace(value) != value || strings.ContainsAny(value, "\r\n\x00") {
			return core.ErrInvalidArgument
		}
	}
	if spec.SourceDevice == spec.StagingDevice || spec.TargetSizeGiB <= 0 {
		return core.ErrInvalidArgument
	}
	return nil
}
