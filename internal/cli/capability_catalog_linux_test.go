//go:build linux

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/network/egress"
	"github.com/SLktEx/Hacocoon/internal/policy"
)

type catalogCLIClient struct {
	catalog capability.CapabilityCatalog
	err     error
	calls   int
}

func (c *catalogCLIClient) CapabilityCatalog(context.Context) (capability.CapabilityCatalog, error) {
	c.calls++
	return c.catalog, c.err
}
func (*catalogCLIClient) ReadConfiguration(context.Context) (capability.PolicySnapshot, error) {
	panic("discovery read Policy")
}
func (*catalogCLIClient) ReplaceConfiguration(context.Context, capability.PolicySnapshot) (capability.PolicySnapshot, error) {
	panic("discovery changed Policy")
}

func cliCatalog(t *testing.T) capability.CapabilityCatalog {
	t.Helper()
	service, err := capability.New(nil, nil, nil, egress.Provider{})
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := service.CapabilityCatalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}

func TestConfigurationCapabilitiesCLIIsReadOnlyAndJSONStable(t *testing.T) {
	var firstJSON string
	for _, locale := range []string{"C", "ja_JP.UTF-8"} {
		setCLITestLocale(t, locale)
		client := &catalogCLIClient{catalog: cliCatalog(t)}
		for _, asJSON := range []bool{false, true} {
			args := []string{"capabilities"}
			if asJSON {
				args = append(args, "--json")
			}
			var out, diagnostic bytes.Buffer
			if code := configurationCommand(context.Background(), client, args, &out, &diagnostic, nil); code != 0 || diagnostic.Len() != 0 {
				t.Fatal(code, diagnostic.String())
			}
			if asJSON {
				var got capability.CapabilityCatalog
				if json.Unmarshal(out.Bytes(), &got) != nil || capability.ValidateCapabilityCatalog(got) != nil {
					t.Fatal("invalid JSON", out.String())
				}
				if firstJSON == "" {
					firstJSON = out.String()
				} else if firstJSON != out.String() {
					t.Fatal("locale changed metadata JSON")
				}
			} else if json.Valid(out.Bytes()) || !strings.Contains(out.String(), "network.egress") || !strings.Contains(out.String(), cliMessage("config.capabilities_readonly")) {
				t.Fatal(out.String())
			}
		}
		if client.calls != 2 {
			t.Fatal("unexpected request count", client.calls)
		}
	}
}

func TestConfigurationCapabilitiesCLIRejectsEditsAndInvalidResults(t *testing.T) {
	client := &catalogCLIClient{catalog: cliCatalog(t)}
	for _, args := range [][]string{{"capabilities", "--edit"}, {"capabilities", "--file", "private.json"}, {"capabilities", "extra"}, {"--json", "capabilities"}} {
		var out, diagnostic bytes.Buffer
		if code := configurationCommand(context.Background(), client, args, &out, &diagnostic, nil); code != 2 || out.Len() != 0 || client.calls != 0 {
			t.Fatal(args, code, out.String(), client.calls)
		}
	}
	for _, invalid := range []bool{false, true} {
		client.err = errors.New("private backend failure")
		if invalid {
			client.err = nil
			client.catalog.Capabilities[0].Actions[0].Description = "unsafe\x1b[2J"
		}
		var out, diagnostic bytes.Buffer
		if code := configurationCommand(context.Background(), client, []string{"capabilities", "--json"}, &out, &diagnostic, nil); code != 1 || out.Len() != 0 || strings.Contains(diagnostic.String(), "private") {
			t.Fatal(code, out.String(), diagnostic.String())
		}
	}
}

func TestConfigurationCapabilitiesHelpDoesNotContactController(t *testing.T) {
	t.Setenv("HACO_CONTROL_SOCKET", "/missing/capability-help.sock")
	for _, locale := range []string{"C", "ja_JP.UTF-8"} {
		setCLITestLocale(t, locale)
		for _, args := range [][]string{{"config", "--help"}, {"config", "capabilities", "--help"}} {
			code, out, diagnostic := captureRun(t, args...)
			if code != 0 || diagnostic != "" || !strings.Contains(out, "capabilities") || !strings.Contains(out, "--json") {
				t.Fatal(args, code, out, diagnostic)
			}
		}
	}
}
