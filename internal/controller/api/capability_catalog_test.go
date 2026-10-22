package controlapi

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/network/egress"
	"github.com/SLktEx/Hacocoon/internal/policy"
)

type catalogAPIService struct {
	catalog capability.CapabilityCatalog
	err     error
	calls   int
}

func (s *catalogAPIService) CapabilityCatalog(context.Context) (capability.CapabilityCatalog, error) {
	s.calls++
	return s.catalog, s.err
}

func apiCatalog(t *testing.T) capability.CapabilityCatalog {
	t.Helper()
	s, err := capability.New(nil, nil, nil, egress.Provider{})
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := s.CapabilityCatalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}

func TestCapabilityCatalogManagementWireIsReadOnlyAndStrict(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service := &catalogAPIService{catalog: apiCatalog(t)}
		server := control.NewServer()
		if err := RegisterCapabilityCatalog(server, service); err != nil {
			t.Fatal(err)
		}
		client, _, _ := setupTransportClient(t, server, context.Background())
		got, err := client.CapabilityCatalog(context.Background())
		if err != nil || len(got.Capabilities) != 1 || got.Capabilities[0].Capability != "network.egress" {
			t.Fatal(got, err)
		}
		for _, raw := range []string{"{}", "{ }", "null"} {
			if err := client.wire.Call(context.Background(), MethodConfigurationCapabilities, json.RawMessage(raw), &got); err != nil {
				t.Fatal(raw, err)
			}
		}
		calls := service.calls
		for _, raw := range []string{`{"policy":{}}`, `{"parameters":{"secret":"private"}}`, `[]`, `true`, `"unknown"`} {
			if err := client.wire.Call(context.Background(), MethodConfigurationCapabilities, json.RawMessage(raw), &got); err == nil {
				t.Fatal("accepted payload", raw)
			}
		}
		if service.calls != calls {
			t.Fatal("invalid request reached service")
		}
		if err := client.wire.Call(context.Background(), MethodConfigurationReplace, nil, nil); err == nil {
			t.Fatal("metadata registration exposed configuration mutation")
		}
		service.err = errors.New("private credential error")
		if _, err := client.CapabilityCatalog(context.Background()); err == nil || strings.Contains(err.Error(), "private") {
			t.Fatal("backend detail escaped", err)
		}
		service.err = nil
		service.catalog.Version++
		if _, err := client.CapabilityCatalog(context.Background()); err == nil {
			t.Fatal("invalid service metadata accepted")
		}
	})
}

func TestCapabilityCatalogClientRefusesMalformedController(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		catalog := apiCatalog(t)
		catalog.Capabilities[0].Actions[0].Description = "untrusted\x1b[2J"
		server := control.NewServer()
		if err := server.Register(MethodConfigurationCapabilities, func(context.Context, json.RawMessage) (any, error) { return catalog, nil }); err != nil {
			t.Fatal(err)
		}
		client, _, _ := setupTransportClient(t, server, context.Background())
		if got, err := client.CapabilityCatalog(context.Background()); !errors.Is(err, control.ErrProtocol) || got.Version != 0 {
			t.Fatal(got, err)
		}
	})
}

func TestCapabilityCatalogRegistrationFailsClosed(t *testing.T) {
	service := &catalogAPIService{catalog: apiCatalog(t)}
	if RegisterCapabilityCatalog(nil, service) == nil || RegisterCapabilityCatalog(control.NewServer(), nil) == nil {
		t.Fatal("nil registration accepted")
	}
	server := control.NewServer()
	if err := RegisterCapabilityCatalog(server, service); err != nil {
		t.Fatal(err)
	}
	if RegisterCapabilityCatalog(server, service) == nil {
		t.Fatal("duplicate registration accepted")
	}
}
