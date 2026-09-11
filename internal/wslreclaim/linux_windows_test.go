//go:build windows && (amd64 || arm64)

package wslreclaim

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/reclamation"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func linuxSuccess() *reclamation.LinuxReport {
	count := uint64(0)
	usage := &reclamation.FilesystemUsage{CapacityBytes: 1024, UsedBytes: 512}
	allocation := &reclamation.Allocation{LogicalBytes: 1024, AllocatedBytes: 512}
	stage := reclamation.Stage{Status: "complete", Attempted: true, FilesystemBefore: usage, FilesystemAfter: usage, KernelTrimmedBytes: &count}
	r := &reclamation.LinuxReport{Pool: stage, Outer: stage}
	r.Pool.Before, r.Pool.After = allocation, allocation
	return r
}
func linuxWire(t *testing.T, r *reclamation.LinuxReport) []byte {
	t.Helper()
	data, err := json.Marshal(struct {
		ProtocolVersion int `json:"protocol_version"`
		*reclamation.LinuxReport
	}{1, r})
	if err != nil {
		t.Fatal(err)
	}
	return append(data, '\n')
}

func TestLinuxReclaimUsesOnlyEnrolledIDsAndFixedExecutable(t *testing.T) {
	id, _ := windows.GenerateGUID()
	installation, _ := windows.GenerateGUID()
	r := registration{ID: id, Name: "Hacocoon-Test", BasePath: `C:\owned`, VHDFileName: "ext4.vhdx"}
	identity := installationIdentity{SchemaVersion: 1, RegistrationID: strings.ToLower(id.String()), InstallationID: strings.Trim(strings.ToLower(installation.String()), "{}")}
	args, err := r.linuxReclaimArguments(identity)
	expected := []string{"--distribution-id", id.String(), "--user", "root", "--cd", "/", "--exec", "/usr/bin/env", "-i", "PATH=/usr/sbin:/usr/bin:/sbin:/bin", "/usr/local/bin/haco", "_reclaim-linux", identity.RegistrationID, identity.InstallationID}
	if err != nil || !reflect.DeepEqual(args, expected) {
		t.Fatal(args, err)
	}
	for _, bad := range []installationIdentity{
		{SchemaVersion: 2, RegistrationID: identity.RegistrationID, InstallationID: identity.InstallationID},
		{SchemaVersion: 1, RegistrationID: strings.ToLower(installation.String()), InstallationID: identity.InstallationID},
		{SchemaVersion: 1, RegistrationID: identity.RegistrationID, InstallationID: "--shutdown"},
	} {
		if _, err := r.linuxReclaimArguments(bad); err == nil {
			t.Fatal("foreign or malformed identity accepted")
		}
	}
}

func TestLinuxReportRefusesMalformedOrUnprovenSuccess(t *testing.T) {
	valid := linuxWire(t, linuxSuccess())
	if report, err := decodeLinuxReport(valid); err != nil || !report.Complete() {
		t.Fatal(report, err)
	}
	invalid := [][]byte{
		nil, []byte("{}\n"), append(append([]byte{}, valid...), valid...),
		bytes.Replace(valid, []byte(`"protocol_version":1`), []byte(`"protocol_version":2`), 1),
		bytes.Replace(valid, []byte(`"protocol_version":1`), []byte(`"protocol_version":1,"protocol_version":1`), 1),
		bytes.Replace(valid, []byte(`"protocol_version":1`), []byte(`"protocol_version":1,"command":"token=private"`), 1),
		bytes.Replace(valid, []byte(`"kernel_trimmed_bytes":0`), []byte(`"kernel_trimmed_bytes":-1`), 1),
		bytes.Repeat([]byte("x"), 4097),
	}
	for i, raw := range invalid {
		if _, err := decodeLinuxReport(raw); err == nil {
			t.Fatalf("invalid report %d accepted", i)
		}
	}
	for _, tc := range []struct {
		name                string
		raw                 []byte
		runErr              error
		wantReport, wantErr bool
	}{
		{"complete", valid, nil, true, false},
		{"exit lost", valid, errors.New("token=private-output"), true, true},
		{"oversize", bytes.Repeat([]byte("x"), 4097), nil, false, true},
		{"failed result", linuxWire(t, func() *reclamation.LinuxReport { r := reclamation.NotStarted("identity_changed"); return &r }()), nil, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			report, err := receiveLinuxReport(context.Background(), func(_ context.Context, w io.Writer) error { calls++; _, _ = w.Write(tc.raw); return tc.runErr })
			if calls != 1 || (report != nil) != tc.wantReport || (err != nil) != tc.wantErr {
				t.Fatal(calls, report, err)
			}
			if err != nil && strings.Contains(err.Error(), "private") {
				t.Fatal("raw child error exposed")
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := receiveLinuxReport(ctx, func(context.Context, io.Writer) error { t.Fatal("canceled command ran"); return nil }); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func linuxRecordFixture(t *testing.T, version int) (*operationStore, operationRecord) {
	t.Helper()
	id, err := windows.GenerateGUID()
	if err != nil {
		t.Fatal(err)
	}
	path := `Software\Hacocoon\Tests\` + id.String()
	key, existed, err := registry.CreateKey(registry.CURRENT_USER, path, registry.ALL_ACCESS)
	if err != nil || existed {
		t.Fatal("isolated registry fixture", err)
	}
	t.Cleanup(func() {
		key.Close()
		if err := registry.DeleteKey(registry.CURRENT_USER, path); err != nil {
			t.Error(err)
		}
	})
	store := &operationStore{key: key}
	r := registration{ID: id, Name: "Hacocoon-Test", BasePath: `C:\owned`, VHDFileName: "ext4.vhdx"}
	intent, err := store.beginVersion(r, diskIdentity{Volume: 1, Low: 2}, version)
	if err != nil {
		t.Fatal(err)
	}
	return store, intent
}

func TestRecordedLinuxStopsOnlyAfterPersistedCompleteReport(t *testing.T) {
	for _, tc := range []struct {
		name    string
		report  *reclamation.LinuxReport
		runErr  error
		windows bool
	}{
		{"complete", linuxSuccess(), nil, true},
		{"missing", nil, nil, false},
		{"lost", nil, errors.New("lost result"), false},
		{"transport", linuxSuccess(), errors.New("lost exit"), false},
		{"failed", func() *reclamation.LinuxReport { r := reclamation.NotStarted("pool_unavailable"); return &r }(), errors.New("failed"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, intent := linuxRecordFixture(t, 2)
			linuxCalls, windowsCalls := 0, 0
			_, err := executeRecordedStages(context.Background(), store, intent, func(context.Context) (*reclamation.LinuxReport, error) {
				linuxCalls++
				current, e := store.read()
				if e != nil || !current.LinuxStarted || current.Linux != nil {
					t.Fatal("attempt not durable before Linux call", e)
				}
				if _, e := store.requirePending(intent.Operation, intent.Registration, intent.Disk); e == nil {
					t.Fatal("interrupted attempt could replay")
				}
				return tc.report, tc.runErr
			}, func(context.Context) (continuationObservation, error) {
				windowsCalls++
				current, e := store.read()
				if e != nil || current.Linux == nil || !current.Linux.Complete() {
					t.Fatal("Windows stop before durable Linux report", e)
				}
				return continuationObservation{StopAttempted: true, StopRequested: true, Compaction: compactObservation{Attempted: true, Completed: true}, ResumeAttempted: true, Resumed: true}, nil
			})
			if linuxCalls != 1 || (windowsCalls == 1) != tc.windows || (err == nil) != tc.windows {
				t.Fatal(linuxCalls, windowsCalls, err)
			}
			saved, e := store.read()
			if e != nil {
				t.Fatal(e)
			}
			if !saved.LinuxStarted || !reflect.DeepEqual(saved.Linux, tc.report) || (saved.State == "complete") != tc.windows {
				t.Fatal("lost or misreported evidence", saved)
			}
			if !tc.windows {
				before, _ := json.Marshal(saved)
				if e := store.reviewFailed(saved.Operation, saved.Registration, saved.Disk); e != nil {
					t.Fatal(e)
				}
				if _, e := store.beginVersion(saved.Registration, saved.Disk, 2); e != nil {
					t.Fatal(e)
				}
				retained, e := store.readOperation(saved.Operation)
				after, _ := json.Marshal(retained)
				if e != nil || !bytes.Equal(before, after) {
					t.Fatal("failed Linux evidence changed", e)
				}
			}
		})
	}
}

func TestRecordedLinuxPreservesLegacyAndChangedIntent(t *testing.T) {
	store, intent := linuxRecordFixture(t, 1)
	raw, _, _ := store.key.GetBinaryValue("Operation")
	if bytes.Contains(raw, []byte("Linux")) {
		t.Fatal("legacy encoding changed")
	}
	if _, err := executeRecordedStages(context.Background(), store, intent, func(context.Context) (*reclamation.LinuxReport, error) {
		t.Fatal("legacy intent silently broadened")
		return nil, nil
	}, func(context.Context) (continuationObservation, error) {
		return continuationObservation{}, errors.New("legacy failure")
	}); err == nil {
		t.Fatal("missing failure")
	}
	original, _, _ := store.key.GetBinaryValue("Operation")
	if err := store.reviewFailed(intent.Operation, intent.Registration, intent.Disk); err != nil {
		t.Fatal(err)
	}
	retained, _, _ := store.key.GetBinaryValue(failedReviewName(intent.Operation))
	if !bytes.Equal(original, retained) {
		t.Fatal("legacy evidence rewritten")
	}
	next, err := store.beginVersion(intent.Registration, intent.Disk, 2)
	if err != nil {
		t.Fatal(err)
	}
	changed := next
	changed.LinuxStarted = true
	if err := store.write(changed); err != nil {
		t.Fatal(err)
	}
	if err := runRecordedLinux(context.Background(), store, &next, func(context.Context) (*reclamation.LinuxReport, error) {
		t.Fatal("changed intent ran")
		return nil, nil
	}); err == nil {
		t.Fatal("changed evidence overwritten")
	}
	saved, err := store.read()
	if err != nil || !reflect.DeepEqual(saved, changed) {
		t.Fatal("changed evidence lost", err)
	}
	changed.State = "complete"
	changed.Observation = continuationObservation{StopAttempted: true, StopRequested: true, Compaction: compactObservation{Attempted: true, Completed: true}, ResumeAttempted: true, Resumed: true}
	if err := changed.validate(); err == nil {
		t.Fatal("Windows success without Linux proof accepted")
	}
}
