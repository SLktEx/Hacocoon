package workspace

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/state"
)

type dataResumeRuntime struct {
	*dataEnvironmentRuntime
	ref, instance string
	areas         []core.EnvironmentRuntimeAttachment
	err           error
}

func (r *dataResumeRuntime) StartEnvironment(context.Context, string) error {
	return errors.New("reference-only resume must not be used")
}
func (r *dataResumeRuntime) StartEnvironmentWithResources(_ context.Context, ref, instance string, areas []core.EnvironmentRuntimeAttachment) error {
	r.ref, r.instance, r.areas = ref, instance, areas
	return r.err
}

type dataResumeCatalog struct {
	*state.EnvironmentJSONStore
	corrupt bool
}

func (s *dataResumeCatalog) GetPersistentResource(ctx context.Context, id string) (core.PersistentResource, error) {
	r, err := s.EnvironmentJSONStore.GetPersistentResource(ctx, id)
	if s.corrupt && r.EnvironmentInstance != "" {
		r.EnvironmentInstance = "env-00000000000000000000000000000000"
	}
	return r, err
}

func TestEnvironmentDataResumeSuppliesExactCatalogResources(t *testing.T) {
	for _, scenario := range []string{"ready", "wrong-parent", "provider-error", "unsupported"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			s, catalog, _, spec, _ := dataEnvironmentService(t, "")
			env, err := s.Create(ctx, spec)
			if err != nil {
				t.Fatal(err)
			}
			runtime := &dataResumeRuntime{dataEnvironmentRuntime: s.runtime.(*dataEnvironmentRuntime)}
			s.runtime = runtime
			s.store = &dataResumeCatalog{EnvironmentJSONStore: catalog, corrupt: scenario == "wrong-parent"}
			if scenario == "provider-error" {
				runtime.err = core.ErrCapabilityStale
			}
			if scenario == "unsupported" {
				s.runtime = runtime.dataEnvironmentRuntime
			}
			err = s.Start(ctx, spec.Name)
			startErr := err
			if (err == nil) != (scenario == "ready") {
				t.Fatal(err)
			}
			if scenario == "wrong-parent" || scenario == "unsupported" {
				if runtime.ref != "" {
					t.Fatal("unsafe resume")
				}
				return
			}
			lease, err := catalog.GetWorkspaceLease(ctx, spec.Name)
			if err != nil {
				t.Fatal(err)
			}
			if runtime.ref != env.RuntimeRef || runtime.instance != lease.InstanceID || len(runtime.areas) != len(lease.Attachments) {
				t.Fatal("incomplete resume binding")
			}
			for i, a := range runtime.areas {
				r, err := catalog.GetPersistentResource(ctx, a.Resource.ID)
				if err != nil || !reflect.DeepEqual(r, a.Resource) || a.Attachment != lease.Attachments[i] {
					t.Fatal("wrong resource", err)
				}
			}
			if scenario == "provider-error" && !errors.Is(startErr, core.ErrCapabilityStale) {
				t.Fatal("provider refusal lost", startErr)
			}
		})
	}
}

type dataSnapshotRuntime struct {
	*snapshotRuntime
	stops int
}

func (r *dataSnapshotRuntime) StopEnvironment(context.Context, string) error { r.stops++; return nil }
func (*dataSnapshotRuntime) StartEnvironment(context.Context, string) error  { return nil }

func TestEnvironmentDataSnapshotRefusesBeforeStopping(t *testing.T) {
	store, runtime := snapshotFixture()
	env := store.environments["resume"]
	lease := store.leases["resume"]
	env.Attachments = []core.EnvironmentAttachment{{Key: "cache"}}
	lease.Attachments = env.Attachments
	store.environments["resume"], store.leases["resume"] = env, lease
	runtime.inspect = func(context.Context, string) (core.EnvironmentRuntimeStatus, error) {
		return core.EnvironmentRuntimeStatus{State: core.EnvironmentRunning}, nil
	}
	r := &dataSnapshotRuntime{snapshotRuntime: runtime}
	s := New(r, store)
	err := s.withSnapshotSourceMode(context.Background(), "resume", true, func(context.Context, core.SnapshotSource) error { return core.ErrUnsupported })
	if !errors.Is(err, core.ErrUnsupported) || r.stops != 0 {
		t.Fatal("unsupported snapshot stopped producer", err, r.stops)
	}
}
