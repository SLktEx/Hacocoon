package incus

import (
	"errors"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/seedbuild"
)

func TestPlanSeedMaintenanceProtectsCurrentAndInstanceBases(t *testing.T) {
	current := testFingerprintA
	instanceBase := testFingerprintB
	unused := "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	plan, err := planSeedMaintenance(
		[]seedMaintenanceInstance{
			{Name: "haco-app", Config: map[string]string{"volatile.base_image": instanceBase}},
			{Name: "haco-seed-build-012345abcdef", Config: map[string]string{"volatile.base_image": current}},
		},
		[]seedMaintenanceImage{
			{Fingerprint: current, Aliases: []seedMaintenanceAlias{{Name: "hacocoon-seed-haco-ubuntu-26-04-1"}}},
			{Fingerprint: instanceBase, Aliases: []seedMaintenanceAlias{{Name: "hacocoon-seed-haco-ubuntu-26-04-2"}}},
			{Fingerprint: unused, Aliases: []seedMaintenanceAlias{{Name: "hacocoon-seed-haco-ubuntu-26-04-3"}}},
		},
		seedbuild.MaintenanceProtection{Revisions: []core.BaseRevision{"sha256:" + current}},
		true,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Builders) != 1 || plan.Builders[0] != "haco-seed-build-012345abcdef" {
		t.Fatalf("builders=%#v", plan.Builders)
	}
	if len(plan.Delete) != 1 || plan.Delete[0] != unused {
		t.Fatalf("delete=%#v", plan.Delete)
	}
	if plan.Retain["sha256:"+current] != "current-manifest" {
		t.Fatalf("current retain=%#v", plan.Retain)
	}
	if plan.Retain["sha256:"+instanceBase] != "instance-base" {
		t.Fatalf("instance retain=%#v", plan.Retain)
	}
}

func TestPlanSeedMaintenanceRetainsExternalAliasesAndUsedBy(t *testing.T) {
	fpExternal := testFingerprintA
	fpUsed := testFingerprintB
	plan, err := planSeedMaintenance(nil, []seedMaintenanceImage{
		{Fingerprint: fpExternal, Aliases: []seedMaintenanceAlias{{Name: "hacocoon-seed-haco-ubuntu-26-04-1"}, {Name: "operator-backup"}}},
		{Fingerprint: fpUsed, Aliases: []seedMaintenanceAlias{{Name: "hacocoon-tooling-haco-ubuntu-26-04-2"}}, UsedBy: []string{"/1.0/instances/haco-app"}},
	}, seedbuild.MaintenanceProtection{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Delete) != 0 {
		t.Fatalf("delete=%#v", plan.Delete)
	}
	if plan.Retain["sha256:"+fpExternal] != "external-alias" || plan.Retain["sha256:"+fpUsed] != "incus-used-by" {
		t.Fatalf("retain=%#v", plan.Retain)
	}
}

func TestPlanSeedMaintenanceFailsClosedOnMalformedInventory(t *testing.T) {
	_, err := planSeedMaintenance(
		[]seedMaintenanceInstance{{Name: "haco-app", Config: map[string]string{"volatile.base_image": "not-a-fingerprint"}}},
		nil,
		seedbuild.MaintenanceProtection{},
		false,
	)
	if !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("err=%v want ErrIncompatibleState", err)
	}

	_, err = planSeedMaintenance(nil, []seedMaintenanceImage{{Fingerprint: "bad", Aliases: []seedMaintenanceAlias{{Name: "hacocoon-seed-haco-ubuntu-26-04-1"}}}}, seedbuild.MaintenanceProtection{}, false)
	if !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("err=%v want ErrIncompatibleState", err)
	}
}

func TestPlanSeedMaintenanceNeverTouchesUnownedImages(t *testing.T) {
	plan, err := planSeedMaintenance(nil, []seedMaintenanceImage{{
		Fingerprint: testFingerprintA,
		Aliases:     []seedMaintenanceAlias{{Name: "ubuntu-26.04"}},
	}}, seedbuild.MaintenanceProtection{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Delete) != 0 || len(plan.Retain) != 0 {
		t.Fatalf("plan=%#v", plan)
	}
}
