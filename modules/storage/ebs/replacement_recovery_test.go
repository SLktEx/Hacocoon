package ebs

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestReplacementResumeAcrossMutationBoundaries(t *testing.T) {
	tests := []struct {
		name           string
		op             Operation
		sourceAttached bool
		targetDevice   string
		wantAbsent     []string
		wantMigrator   string
	}{
		{
			name:           "planning before create",
			op:             operationWithoutTarget(PhasePlanning),
			sourceAttached: true,
			wantMigrator:   "migrate|verify|activate",
		},
		{
			name:           "created before readiness",
			op:             operationFromSpec(spec(), PhaseTargetCreated),
			sourceAttached: true,
			wantMigrator:   "migrate|verify|activate",
		},
		{
			name:           "attach completed before phase save",
			op:             operationFromSpec(spec(), PhaseTargetCreated),
			sourceAttached: true,
			targetDevice:   spec().StagingDevice,
			wantAbsent:     []string{"attach:vol-target:/dev/sdg|"},
			wantMigrator:   "migrate|verify|activate",
		},
		{
			name:           "target attached",
			op:             operationFromSpec(spec(), PhaseTargetAttached),
			sourceAttached: true,
			targetDevice:   spec().StagingDevice,
			wantMigrator:   "migrate|verify|activate",
		},
		{
			name:           "migration completed",
			op:             operationFromSpec(spec(), PhaseMigrated),
			sourceAttached: true,
			targetDevice:   spec().StagingDevice,
			wantMigrator:   "verify|activate",
		},
		{
			name:           "source detach completed before phase save",
			op:             operationFromSpec(spec(), PhaseVerified),
			sourceAttached: false,
			targetDevice:   spec().StagingDevice,
			wantAbsent:     []string{"detach:vol-source"},
			wantMigrator:   "activate",
		},
		{
			name:           "target staging detach completed",
			op:             operationFromSpec(spec(), PhaseSourceDetached),
			sourceAttached: false,
			wantAbsent:     []string{"detach:vol-target"},
			wantMigrator:   "activate",
		},
		{
			name:           "promotion completed before phase save",
			op:             operationFromSpec(spec(), PhaseSourceDetached),
			sourceAttached: false,
			targetDevice:   spec().SourceDevice,
			wantAbsent:     []string{"detach:vol-target", "attach:vol-target:/dev/sdf"},
			wantMigrator:   "activate",
		},
		{
			name:           "target promoted",
			op:             operationFromSpec(spec(), PhaseTargetPromoted),
			sourceAttached: false,
			targetDevice:   spec().SourceDevice,
			wantMigrator:   "activate",
		},
		{
			name:           "activation completed",
			op:             operationFromSpec(spec(), PhaseActivated),
			sourceAttached: false,
			targetDevice:   spec().SourceDevice,
			wantMigrator:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := &fakeAPI{volumes: recoveryVolumes(tt.sourceAttached, tt.targetDevice)}
			journal := &fakeJournal{current: map[string]Operation{tt.op.ID: tt.op}}
			migrator := &fakeMigrator{}

			got, err := New(api, migrator, journal).Resume(context.Background(), spec())
			if err != nil {
				t.Fatalf("resume failed: op=%#v err=%v calls=%v", got, err, api.calls)
			}
			if got.Phase != PhaseComplete || got.TargetVolumeID != "vol-target" {
				t.Fatalf("got=%#v", got)
			}
			if _, found := journal.current[spec().OperationID]; found {
				t.Fatalf("completed journal was not deleted: %#v", journal.current)
			}
			if gotCalls := strings.Join(migrator.calls, "|"); gotCalls != tt.wantMigrator {
				t.Fatalf("migrator calls=%q want=%q", gotCalls, tt.wantMigrator)
			}
			joined := strings.Join(api.calls, "|")
			for _, absent := range tt.wantAbsent {
				if strings.Contains(joined, strings.TrimSuffix(absent, "|")) {
					t.Fatalf("unexpected replay %q in calls=%v", absent, api.calls)
				}
			}
		})
	}
}

func TestReplacementResumeRecoveryRequiredAfterAmbiguousAttach(t *testing.T) {
	op := operationFromSpec(spec(), PhaseRecoveryRequired)
	op.RecoveryFrom = PhaseTargetCreated
	api := &fakeAPI{volumes: recoveryVolumes(true, spec().StagingDevice)}
	journal := &fakeJournal{current: map[string]Operation{op.ID: op}}
	migrator := &fakeMigrator{}

	got, err := New(api, migrator, journal).Resume(context.Background(), spec())
	if err != nil {
		t.Fatalf("resume failed: op=%#v err=%v", got, err)
	}
	if got.Phase != PhaseComplete {
		t.Fatalf("got=%#v", got)
	}
	if strings.Contains(strings.Join(api.calls, "|"), "attach:vol-target:/dev/sdg") {
		t.Fatalf("already-completed attachment was replayed: %v", api.calls)
	}
}

func TestReplacementResumeRefusesUnsafePreVerificationSourceDetach(t *testing.T) {
	op := operationFromSpec(spec(), PhaseRecoveryRequired)
	op.RecoveryFrom = PhaseMigrated
	api := &fakeAPI{volumes: recoveryVolumes(false, spec().StagingDevice)}
	journal := &fakeJournal{current: map[string]Operation{op.ID: op}}

	got, err := New(api, &fakeMigrator{}, journal).Resume(context.Background(), spec())
	if !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("got=%#v err=%v", got, err)
	}
	if got.Phase != PhaseRecoveryRequired || got.RecoveryFrom != PhaseMigrated {
		t.Fatalf("unsafe state was advanced: %#v", got)
	}
	joined := strings.Join(api.calls, "|")
	if strings.Contains(joined, "attach:") || strings.Contains(joined, "detach:") || strings.Contains(joined, "delete:") {
		t.Fatalf("unsafe recovery mutated AWS state: %v", api.calls)
	}
}

func TestReplacementResumeRefusesLegacyRecoveryWithoutResumePhase(t *testing.T) {
	op := operationFromSpec(spec(), PhaseRecoveryRequired)
	op.Version = 1
	op.RecoveryFrom = ""
	api := &fakeAPI{volumes: recoveryVolumes(true, spec().StagingDevice)}
	journal := &fakeJournal{current: map[string]Operation{op.ID: op}}

	got, err := New(api, &fakeMigrator{}, journal).Resume(context.Background(), spec())
	if !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("got=%#v err=%v", got, err)
	}
	if len(api.calls) != 0 {
		t.Fatalf("legacy ambiguous journal should not be auto-mutated: %v", api.calls)
	}
}

func TestReplacementReplaceResumesMatchingExistingJournal(t *testing.T) {
	op := operationFromSpec(spec(), PhaseVerified)
	api := &fakeAPI{volumes: recoveryVolumes(true, spec().StagingDevice)}
	journal := &fakeJournal{current: map[string]Operation{op.ID: op}}

	got, err := New(api, &fakeMigrator{}, journal).Replace(context.Background(), spec())
	if err != nil {
		t.Fatalf("replace did not resume: op=%#v err=%v", got, err)
	}
	if got.Phase != PhaseComplete {
		t.Fatalf("got=%#v", got)
	}
	if strings.Contains(strings.Join(api.calls, "|"), "create") {
		t.Fatalf("resume created another target: %v", api.calls)
	}
}

func TestReplacementResumeCleansCompletedJournalWithoutAWS(t *testing.T) {
	op := operationFromSpec(spec(), PhaseComplete)
	journal := &fakeJournal{current: map[string]Operation{op.ID: op}}
	api := &fakeAPI{volumes: recoveryVolumes(false, spec().SourceDevice)}

	got, err := New(api, &fakeMigrator{}, journal).Resume(context.Background(), spec())
	if err != nil || got.Phase != PhaseComplete {
		t.Fatalf("got=%#v err=%v", got, err)
	}
	if len(api.calls) != 0 {
		t.Fatalf("completed recovery touched AWS: %v", api.calls)
	}
}

func operationWithoutTarget(phase Phase) Operation {
	op := operationFromSpec(spec(), phase)
	op.TargetVolumeID = ""
	return op
}

func recoveryVolumes(sourceAttached bool, targetDevice string) map[string]Volume {
	s := spec()
	source := Volume{
		ID:               s.SourceVolumeID,
		SizeGiB:          100,
		AvailabilityZone: "ap-northeast-1a",
		Type:             "gp3",
		Encrypted:        true,
		State:            "available",
	}
	if sourceAttached {
		source.State = "in-use"
		source.Attachments = []VolumeAttachment{{InstanceID: s.InstanceID, Device: s.SourceDevice, State: "attached"}}
	}
	target := Volume{
		ID:               "vol-target",
		SizeGiB:          s.TargetSizeGiB,
		AvailabilityZone: "ap-northeast-1a",
		Type:             "gp3",
		Encrypted:        true,
		State:            "available",
	}
	if targetDevice != "" {
		target.State = "in-use"
		target.Attachments = []VolumeAttachment{{InstanceID: s.InstanceID, Device: targetDevice, State: "attached"}}
	}
	return map[string]Volume{s.SourceVolumeID: source, "vol-target": target}
}
