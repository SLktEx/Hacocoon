package environment

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"testing"
)

type receiptTestProvider struct {
	baseTestProvider
	mode string
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
			got, err := NewBaseRouter(router).CreateEnvironmentWithReceipt(context.Background(), core.EnvironmentRuntimeSpec{}, func(v core.EnvironmentRuntime) error {
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
