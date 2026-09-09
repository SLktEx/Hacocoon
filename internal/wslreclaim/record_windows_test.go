//go:build windows && (amd64 || arm64)

package wslreclaim

import (
	"bytes"
	"encoding/json"
	"errors"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"strings"
	"testing"
)

func TestNativeOperationRecordRetainsInterruptedAndFailedIntent(t *testing.T) {
	id, err := windows.GenerateGUID()
	if err != nil {
		t.Fatal(err)
	}
	path := `Software\Hacocoon\Tests\` + id.String()
	key, existed, err := registry.CreateKey(registry.CURRENT_USER, path, registry.ALL_ACCESS)
	if err != nil {
		t.Fatal(err)
	}
	if existed {
		key.Close()
		t.Fatal("test key exists")
	}
	defer func() {
		key.Close()
		if err := registry.DeleteKey(registry.CURRENT_USER, path); err != nil {
			t.Error(err)
		}
	}()
	s := &operationStore{key: key}
	r := registration{ID: id, Name: "Hacocoon-Test", BasePath: `C:\owned`, VHDFileName: "ext4.vhdx"}
	disk := diskIdentity{Volume: 1, Low: 2}
	intent, err := s.begin(r, disk)
	if err != nil {
		t.Fatal(err)
	}
	if saved, err := s.requirePending(intent.Operation, r, disk); err != nil || saved != intent {
		t.Fatal("exact handoff refused", saved, err)
	}
	otherID, err := windows.GenerateGUID()
	if err != nil {
		t.Fatal(err)
	}
	changedRegistration := r
	changedRegistration.Name = "replacement"
	for _, request := range []struct {
		id     windows.GUID
		target registration
		file   diskIdentity
	}{
		{windows.GUID{}, r, disk}, {otherID, r, disk}, {intent.Operation, changedRegistration, disk},
		{intent.Operation, r, diskIdentity{Volume: 1, Low: 3}},
	} {
		if _, err := s.requirePending(request.id, request.target, request.file); err == nil {
			t.Fatal("foreign handoff accepted")
		}
		if saved, err := s.read(); err != nil || saved != intent {
			t.Fatal("refusal changed pending intent", saved, err)
		}
	}
	reopened, err := registry.OpenKey(registry.CURRENT_USER, path, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	other := &operationStore{key: reopened}
	defer other.close()
	if saved, err := other.read(); err != nil || saved != intent {
		t.Fatal(saved, err)
	}
	if _, err := other.begin(r, disk); !errors.Is(err, errOperationNeedsReview) {
		t.Fatal("interrupted intent overwritten", err)
	}
	changed := intent
	changed.Disk.Low++
	if err := other.finish(changed, continuationObservation{}, errors.New("failed")); err == nil {
		t.Fatal("foreign intent replaced")
	}
	if err := other.finish(intent, continuationObservation{}, nil); err == nil {
		t.Fatal("unexecuted marked success")
	}
	if err := other.finish(intent, continuationObservation{StopAttempted: true}, errors.New("stop failed")); err != nil {
		t.Fatal(err)
	}
	failed, err := s.read()
	if err != nil || failed.State != "failed" {
		t.Fatal(failed, err)
	}
	if _, err := s.requirePending(intent.Operation, r, disk); err == nil {
		t.Fatal("failed handoff replay accepted")
	}
	if _, err := s.begin(r, disk); !errors.Is(err, errOperationNeedsReview) {
		t.Fatal("failed intent overwritten", err)
	}
	after, err := s.read()
	if err != nil || after != failed {
		t.Fatal("record lost", after, err)
	}
}

func TestNativeOperationRecordSuccessfulReplacementAndMalformedRefusal(t *testing.T) {
	id, err := windows.GenerateGUID()
	if err != nil {
		t.Fatal(err)
	}
	path := `Software\Hacocoon\Tests\` + id.String()
	key, existed, err := registry.CreateKey(registry.CURRENT_USER, path, registry.ALL_ACCESS)
	if err != nil {
		t.Fatal(err)
	}
	if existed {
		key.Close()
		t.Fatal("test key exists")
	}
	defer func() {
		key.Close()
		if err := registry.DeleteKey(registry.CURRENT_USER, path); err != nil {
			t.Error(err)
		}
	}()
	s := &operationStore{key: key}
	r := registration{ID: id, Name: "Hacocoon-Test", BasePath: `C:\owned`, VHDFileName: "ext4.vhdx"}
	disk := diskIdentity{Volume: 1, Low: 2}
	intent, err := s.begin(r, disk)
	if err != nil {
		t.Fatal(err)
	}
	o := continuationObservation{StopAttempted: true, StopRequested: true, Compaction: compactObservation{Attempted: true, Completed: true}, ResumeAttempted: true, Resumed: true}
	if err := s.finish(intent, o, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.requirePending(intent.Operation, r, disk); err == nil {
		t.Fatal("completed handoff replay accepted")
	}
	if _, err := s.begin(r, diskIdentity{Volume: 1, Low: 3}); err == nil {
		t.Fatal("replacement disk accepted")
	}
	next, err := s.begin(r, disk)
	if err != nil || next.Operation == intent.Operation {
		t.Fatal(next, err)
	}
	valid, err := json.Marshal(next)
	if err != nil {
		t.Fatal(err)
	}
	for _, data := range [][]byte{
		[]byte(strings.Replace(string(valid), `"Version":1`, `"Version":2`, 1)),
		[]byte(strings.Replace(string(valid), `"Version":1`, `"Version":1,"Version":1`, 1)),
		[]byte(strings.Replace(string(valid), `"Version":1`, `"Version":1,"Future":true`, 1)),
		make([]byte, 16385),
	} {
		if err := key.SetBinaryValue("Operation", data); err != nil {
			t.Fatal(err)
		}
		if _, err := s.read(); err == nil {
			t.Fatal("malformed record readable")
		}
		if _, err := s.begin(r, disk); err == nil {
			t.Fatal("malformed record accepted")
		}
		saved, _, err := key.GetBinaryValue("Operation")
		if err != nil || !bytes.Equal(saved, data) {
			t.Fatal("bad record discarded", err)
		}
	}
	if err := key.SetStringValue("Operation", "unknown"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.begin(r, disk); err == nil {
		t.Fatal("wrong registry type accepted")
	}
}
