package core

import (
	"context"
	"errors"
	"testing"
)

type cleanupFailureRuntime struct {
	fakeRuntime
	deleteErr error
}

func (r *cleanupFailureRuntime) Delete(context.Context, string) error {
	r.deleted = true
	return r.deleteErr
}

func TestCreateReportsRecoveryRequiredWhenPersistAndCleanupFail(t *testing.T) {
	runtime := &cleanupFailureRuntime{deleteErr: errors.New("runtime delete failed")}
	store := newMemStore()
	store.putErr = errors.New("state disk full")
	manager := NewManager(runtime, &fakeStorage{}, store)
	manager.newID = func() (SessionID, error) { return "0123456789abcdef", nil }

	_, err := manager.Create(context.Background(), SessionSpec{Name: "x"})
	if err == nil {
		t.Fatal("expected create failure")
	}
	if !runtime.deleted {
		t.Fatal("runtime cleanup was not attempted")
	}
	if !errors.Is(err, ErrRecoveryRequired) {
		t.Fatalf("cleanup failure must require recovery: %v", err)
	}
	if !errors.Is(err, runtime.deleteErr) {
		t.Fatalf("cleanup error was lost: %v", err)
	}
	if !errors.Is(err, store.putErr) {
		t.Fatalf("persistence error was lost: %v", err)
	}
}
