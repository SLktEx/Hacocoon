package environment

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"reflect"
	"testing"
)

type receiptTestProvider struct {
	baseTestProvider
	mode string
}

func TestCompleteProviderReceiptPreservesMetadataAndFailureOwnership(t *testing.T) {
	budget, err := core.ResolveResourceBudget(core.ResourceBudget{})
	if err != nil {
		t.Fatal(err)
	}
	native := core.EnvironmentRuntime{Ref: "created-native", Base: &core.BaseRef{Name: "base", Revision: "immutable"}, Resources: budget}
	spec := core.EnvironmentRuntimeSpec{Name: "demo", Base: "base", WorkspacePath: "/work", Resources: budget}
	for _, mode := range []string{"ok", "create-failure", "record-failure", "empty-ref"} {
		t.Run(mode, func(t *testing.T) {
			p := &baseTestProvider{created: native}
			failure := errors.New("durable operation failed")
			if mode == "create-failure" {
				p.createErr = failure
			}
			if mode == "empty-ref" {
				p.created.Ref = " \n"
			}
			r, _ := NewRouter(testProvider, Register(testProvider, p))
			calls := 0
			var recorded core.EnvironmentRuntime
			got, err := r.CreateEnvironmentWithReceipt(context.Background(), spec, func(v core.EnvironmentRuntime) error {
				calls++
				recorded = v
				if mode == "record-failure" {
					return failure
				}
				return nil
			})
			if p.createCalls != 1 || !reflect.DeepEqual(p.spec, spec) {
				t.Fatal("creation request changed", p)
			}
			if mode == "create-failure" || mode == "empty-ref" {
				want := failure
				if mode == "empty-ref" {
					want = core.ErrIncompatibleState
				}
				if !errors.Is(err, want) || calls != 0 || got.Ref != "" {
					t.Fatal("failed creation recorded as owned", got, err, calls)
				}
				return
			}
			want := native
			want.Ref = encodeRouteRef(testProvider, native.Ref)
			if calls != 1 || !reflect.DeepEqual(recorded, want) || !reflect.DeepEqual(got, want) {
				t.Fatal("ownership or metadata lost", got, recorded, calls)
			}
			if mode == "record-failure" {
				if !errors.Is(err, failure) {
					t.Fatal("receipt failure lost", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCreationRefusalsHaveNoProviderOrReceiptSideEffects(t *testing.T) {
	p := &baseTestProvider{created: core.EnvironmentRuntime{Ref: "native"}}
	r, _ := NewRouter(testProvider, Register(testProvider, p))
	work := core.NewTemporaryWorkspace()
	for _, tc := range []struct {
		name       string
		router     *Router
		spec       core.EnvironmentRuntimeSpec
		omitRecord bool
		want       error
	}{
		{"nil", nil, core.EnvironmentRuntimeSpec{}, false, core.ErrInvalidArgument},
		{"missing-receipt", r, core.EnvironmentRuntimeSpec{}, true, core.ErrInvalidArgument},
		{"missing-provider", &Router{defaultProvider: "missing"}, core.EnvironmentRuntimeSpec{}, false, core.ErrUnsupported},
		{"unsupported-temporary", r, core.EnvironmentRuntimeSpec{TemporaryWorkspace: true, WorkspacePath: work.Path}, false, core.ErrUnsupported},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			record := func(core.EnvironmentRuntime) error { calls++; return nil }
			if tc.omitRecord {
				record = nil
			}
			if _, err := tc.router.CreateEnvironmentWithReceipt(context.Background(), tc.spec, record); !errors.Is(err, tc.want) || calls != 0 || p.createCalls != 0 {
				t.Fatal("refused creation had side effects", err, calls, p.createCalls)
			}
		})
	}
	if _, err := (&Router{defaultProvider: "missing"}).CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{}); !errors.Is(err, core.ErrUnsupported) {
		t.Fatal(err)
	}
}

func (p *receiptTestProvider) CreateEnvironmentWithReceipt(ctx context.Context, spec core.EnvironmentRuntimeSpec, record func(core.EnvironmentRuntime) error) (core.EnvironmentRuntime, error) {
	v := p.created
	if p.mode == "omitted" {
		return v, nil
	}
	if err := record(v); err != nil {
		return core.EnvironmentRuntime{}, err
	}
	if p.mode == "duplicate" {
		_ = record(v)
	}
	if p.mode == "drift" {
		v.Ref = "other"
	}
	return v, nil
}
func TestCreationReceiptRoutingAndProtocol(t *testing.T) {
	for _, mode := range []string{"ok", "omitted", "duplicate", "drift", "record-failure"} {
		t.Run(mode, func(t *testing.T) {
			p := &receiptTestProvider{baseTestProvider: baseTestProvider{created: core.EnvironmentRuntime{Ref: "haco-demo", Base: &core.BaseRef{Name: "base", Revision: "immutable"}}}, mode: mode}
			router, err := NewRouter(ProviderIncus, Register(ProviderIncus, p))
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			sentinel := errors.New("receipt failed")
			got, err := router.CreateEnvironmentWithReceipt(context.Background(), core.EnvironmentRuntimeSpec{}, func(v core.EnvironmentRuntime) error {
				count++
				if v.Ref != encodeRouteRef(ProviderIncus, "haco-demo") || v.Base == nil {
					t.Fatal(v)
				}
				if mode == "record-failure" {
					return sentinel
				}
				return nil
			})
			if mode == "ok" {
				if err != nil || got.Ref != encodeRouteRef(ProviderIncus, "haco-demo") || count != 1 {
					t.Fatal(got, err, count)
				}
			} else if err == nil {
				t.Fatal("protocol violation accepted", mode)
			}
			if mode == "record-failure" && !errors.Is(err, sentinel) {
				t.Fatal("lost receipt failure", err)
			}
			if mode == "duplicate" && count != 1 {
				t.Fatal("duplicate durable receipt", count)
			}
		})
	}
}
