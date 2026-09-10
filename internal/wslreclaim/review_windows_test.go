//go:build windows && (amd64 || arm64)

package wslreclaim

import (
	"bytes"
	"encoding/json"
	"errors"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"testing"
)

func TestNativeFailedReviewPreservesEvidenceAndRefusesPending(t *testing.T) {
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
	pending, err := s.begin(r, disk)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.reviewFailed(pending.Operation, r, disk); err == nil {
		t.Fatal("unknown pending operation reviewed")
	}
	if _, err := s.readValue(failedReviewName(pending.Operation)); !errors.Is(err, registry.ErrNotExist) {
		t.Fatal("pending evidence created", err)
	}
	if err := s.finish(pending, continuationObservation{StopAttempted: true, ResumeAttempted: true, Resumed: true}, errors.New("stop failed")); err != nil {
		t.Fatal(err)
	}
	failed, err := s.read()
	if err != nil {
		t.Fatal(err)
	}
	original, err := json.Marshal(failed)
	if err != nil {
		t.Fatal(err)
	}
	other, err := windows.GenerateGUID()
	if err != nil {
		t.Fatal(err)
	}
	changed := r
	changed.Name = "replacement"
	for _, request := range []struct {
		id   windows.GUID
		r    registration
		disk diskIdentity
	}{
		{windows.GUID{}, r, disk}, {other, r, disk}, {pending.Operation, changed, disk}, {pending.Operation, r, diskIdentity{Volume: 1, Low: 3}},
	} {
		if err := s.reviewFailed(request.id, request.r, request.disk); err == nil {
			t.Fatal("foreign review accepted")
		}
	}
	name := failedReviewName(pending.Operation)
	if err := key.SetBinaryValue(name, []byte("unknown")); err != nil {
		t.Fatal(err)
	}
	if err := s.reviewFailed(pending.Operation, r, disk); err == nil {
		t.Fatal("unknown evidence overwritten")
	}
	if _, err := s.begin(r, disk); err == nil {
		t.Fatal("unknown evidence authorizes replacement")
	}
	raw, _, err := key.GetBinaryValue(name)
	if err != nil || string(raw) != "unknown" {
		t.Fatal("malformed evidence lost", err)
	}
	// Remove only this invocation's injected malformed value.
	if err := key.DeleteValue(name); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := s.reviewFailed(pending.Operation, r, disk); err != nil {
			t.Fatal(err)
		}
		now, err := s.read()
		if err != nil || now != failed {
			t.Fatal("review changed original failure", err)
		}
	}
	raw, _, err = key.GetBinaryValue(name)
	if err != nil || !bytes.Equal(raw, original) {
		t.Fatal("failed result not retained verbatim", err)
	}
	next, err := s.begin(r, disk)
	if err != nil || next.Operation == pending.Operation {
		t.Fatal("fresh attempt not created", err)
	}
	if _, err := s.requirePending(pending.Operation, r, disk); err == nil {
		t.Fatal("old operation replay accepted")
	}
	saved, err := s.readOperation(pending.Operation)
	if err != nil || saved != failed {
		t.Fatal("older failed status lost", err)
	}
	if err := s.reviewFailed(pending.Operation, r, disk); err == nil {
		t.Fatal("stale review accepted after replacement")
	}
	if err := s.reviewFailed(next.Operation, r, disk); err == nil {
		t.Fatal("new pending review accepted")
	}
	current, err := s.read()
	if err != nil || current != next {
		t.Fatal("refusal changed new operation", err)
	}
	observation := continuationObservation{StopAttempted: true, StopRequested: true, Compaction: compactObservation{Attempted: true, Completed: true}, ResumeAttempted: true, Resumed: true}
	if err := s.finish(next, observation, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.reviewFailed(next.Operation, r, disk); err == nil {
		t.Fatal("completed result treated as failed")
	}
	saved, err = s.readOperation(pending.Operation)
	if err != nil || saved != failed {
		t.Fatal("completion discarded prior failure", err)
	}
}
