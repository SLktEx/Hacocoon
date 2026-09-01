package main

import "testing"

func TestProductionClientOwnerRequiresBothNumericIDs(t *testing.T) {
	t.Setenv(controlClientUIDEnv, "1001")
	t.Setenv(controlClientGIDEnv, "1002")
	uid, gid, err := productionClientOwner()
	if err != nil {
		t.Fatal(err)
	}
	if uid != 1001 || gid != 1002 {
		t.Fatalf("owner = %d:%d, want 1001:1002", uid, gid)
	}
}

func TestProductionClientOwnerFailsClosedWithoutIdentity(t *testing.T) {
	t.Setenv(controlClientUIDEnv, "")
	t.Setenv(controlClientGIDEnv, "")
	if _, _, err := productionClientOwner(); err == nil {
		t.Fatal("production client owner unexpectedly accepted missing identity")
	}
}

func TestProductionClientOwnerRejectsNonNumericIdentity(t *testing.T) {
	t.Setenv(controlClientUIDEnv, "runner")
	t.Setenv(controlClientGIDEnv, "1001")
	if _, _, err := productionClientOwner(); err == nil {
		t.Fatal("production client owner unexpectedly accepted a non-numeric uid")
	}
}
