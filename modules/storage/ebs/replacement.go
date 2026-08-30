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

type Volume struct {
	ID               string `json:"id"`
	SizeGiB          int64  `json:"size_gib"`
	AvailabilityZone string `json:"availability_zone"`
	Type             string `json:"type"`
	Encrypted        bool   `json:"encrypted"`
	KMSKeyID         string `json:"kms_key_id,omitempty"`
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
	Delete(context.Context, string) error
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
	if s == nil || s.api == nil || s.migrator == nil || s.journal == nil {
		return Operation{}, core.ErrInvalidArgument
	}
	if err := validateSpec(spec); err != nil {
		return Operation{}, err
	}

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
		Version:        1,
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

	target, err := s.api.CreateVolume(ctx, source, spec.TargetSizeGiB, clientTokenForOperation(spec.OperationID))
	if err != nil {
		return op, fmt.Errorf("create replacement EBS volume: %w", err)
	}
	op.TargetVolumeID = target
	op.Phase = PhaseTargetCreated
	if err := s.save(ctx, op); err != nil {
		return s.cleanupCreatedTarget(ctx, op, err)
	}
	if err := s.api.WaitAvailable(ctx, target); err != nil {
		return s.recoveryRequired(ctx, op, fmt.Errorf("replacement EBS volume readiness is ambiguous: %w", err))
	}

	if err := s.api.AttachVolume(ctx, target, spec.InstanceID, spec.StagingDevice); err != nil {
		return s.cleanupBeforeDetach(ctx, op, fmt.Errorf("attach replacement EBS volume: %w", err))
	}
	if err := s.api.WaitInUse(ctx, target); err != nil {
		return s.cleanupBeforeDetach(ctx, op, fmt.Errorf("wait replacement EBS attachment: %w", err))
	}
	op.Phase = PhaseTargetAttached
	if err := s.save(ctx, op); err != nil {
		return s.cleanupBeforeDetach(ctx, op, err)
	}

	if err := s.migrator.Migrate(ctx, spec, target); err != nil {
		return s.cleanupBeforeDetach(ctx, op, fmt.Errorf("migrate EBS data: %w", err))
	}
	op.Phase = PhaseMigrated
	if err := s.save(ctx, op); err != nil {
		return s.cleanupBeforeDetach(ctx, op, err)
	}
	if err := s.migrator.Verify(ctx, spec, target); err != nil {
		return s.cleanupBeforeDetach(ctx, op, fmt.Errorf("verify EBS migration: %w", err))
	}
	op.Phase = PhaseVerified
	if err := s.save(ctx, op); err != nil {
		return s.cleanupBeforeDetach(ctx, op, err)
	}

	if err := s.api.DetachVolume(ctx, spec.SourceVolumeID, spec.InstanceID); err != nil {
		return s.cleanupBeforeDetach(ctx, op, fmt.Errorf("detach source EBS volume: %w", err))
	}
	if err := s.api.WaitAvailable(ctx, spec.SourceVolumeID); err != nil {
		return s.recoveryRequired(ctx, op, fmt.Errorf("source EBS detach became ambiguous: %w", err))
	}
	op.Phase = PhaseSourceDetached
	if err := s.save(ctx, op); err != nil {
		return s.recoveryRequired(ctx, op, err)
	}

	if err := s.api.DetachVolume(ctx, target, spec.InstanceID); err != nil {
		return s.recoveryRequired(ctx, op, fmt.Errorf("detach replacement EBS staging attachment: %w", err))
	}
	if err := s.api.WaitAvailable(ctx, target); err != nil {
		return s.recoveryRequired(ctx, op, fmt.Errorf("wait replacement EBS staging detach: %w", err))
	}
	if err := s.api.AttachVolume(ctx, target, spec.InstanceID, spec.SourceDevice); err != nil {
		return s.recoveryRequired(ctx, op, fmt.Errorf("promote replacement EBS volume: %w", err))
	}
	if err := s.api.WaitInUse(ctx, target); err != nil {
		return s.recoveryRequired(ctx, op, fmt.Errorf("wait replacement EBS promotion: %w", err))
	}
	op.Phase = PhaseTargetPromoted
	if err := s.save(ctx, op); err != nil {
		return s.recoveryRequired(ctx, op, err)
	}

	if err := s.migrator.Activate(ctx, spec, target); err != nil {
		return s.recoveryRequired(ctx, op, fmt.Errorf("activate replacement EBS volume: %w", err))
	}
	op.Phase = PhaseActivated
	if err := s.save(ctx, op); err != nil {
		return s.recoveryRequired(ctx, op, err)
	}
	op.Phase = PhaseComplete
	if err := s.save(ctx, op); err != nil {
		return s.recoveryRequired(ctx, op, err)
	}
	if err := s.journal.Delete(ctx, op.ID); err != nil {
		return s.recoveryRequired(ctx, op, fmt.Errorf("remove completed EBS journal: %w", err))
	}
	return op, nil
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
