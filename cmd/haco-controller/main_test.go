package main

import "testing"

func TestProductionControlGroupGIDAcceptsNumericNonRootGroup(t *testing.T) {
	t.Setenv(controlGroupGIDEnv, "1002")
	gid, err := productionControlGroupGID()
	if err != nil {
		t.Fatal(err)
	}
	if gid != 1002 {
		t.Fatalf("gid = %d, want 1002", gid)
	}
}

func TestProductionControlGroupGIDFailsClosedWhenMissing(t *testing.T) {
	t.Setenv(controlGroupGIDEnv, "")
	if _, err := productionControlGroupGID(); err == nil {
		t.Fatal("production control group unexpectedly accepted missing gid")
	}
}

func TestProductionControlGroupGIDRejectsNonNumericValue(t *testing.T) {
	t.Setenv(controlGroupGIDEnv, "hacocoon")
	if _, err := productionControlGroupGID(); err == nil {
		t.Fatal("production control group unexpectedly accepted a non-numeric gid")
	}
}

func TestProductionControlGroupGIDRejectsRootGroup(t *testing.T) {
	t.Setenv(controlGroupGIDEnv, "0")
	if _, err := productionControlGroupGID(); err == nil {
		t.Fatal("production control group unexpectedly accepted gid 0")
	}
}
