//go:build linux

package incus

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/internal/state"
	"github.com/SLktEx/Hacocoon/internal/workspace"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// This verifies installed guest provisioning and the explicit backend adapter.
// It does not grant fixture DNS Policy or claim VPN/NRPT/human acceptance.
func TestRealIncusDNSModesE2E(t *testing.T) {
	if os.Getenv("HACO_E2E_DNS_MODES") != "1" {
		t.Skip("set HACO_E2E_DNS_MODES=1 on an Incus host")
	}
	pool, image, companion := os.Getenv("HACO_E2E_INCUS_RESUME_POOL"), os.Getenv("HACO_E2E_INCUS_RESUME_IMAGE"), os.Getenv("HACO_E2E_DNS_COMPANION")
	if !safeIncusRef(pool) || !baseFingerprintPattern.MatchString(image) || companion == "" {
		t.Fatal("explicit cached image, pool and product companion required")
	}
	for _, mode := range []core.DNSMode{core.DNSHost, core.DNSBackend, core.DNSDisabled} {
		t.Run(string(mode), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			var nonce [8]byte
			_, err := rand.Read(nonce[:])
			must(err)
			name := "dns-e2e-" + hex.EncodeToString(nonce[:])
			root, err := os.MkdirTemp("/var/lib", "haco-dns-modes-")
			must(err)
			work := filepath.Join(root, "work")
			must(os.Mkdir(work, 0755))
			t.Logf("owned Env=%s catalog=%s mode=%s", name, filepath.Join(root, "state.json"), mode)
			runtime := New(WrapEnvironmentNetworkOwnershipRunner(host.ExecRunner{}))
			runtime.setRootPool(pool)
			must(runtime.ConfigureEnvironmentDNS(companion))
			provider, err := NewSandboxProvider(runtime)
			must(err)
			provider.sources["fixture-parent"] = "local:" + image
			catalog := state.NewEnvironmentJSONStore(filepath.Join(root, "state.json"))
			service := workspace.New(provider, catalog)
			env, err := service.Create(ctx, core.EnvironmentSpec{Name: name, WorkspacePath: work, Base: "fixture-parent", DNSMode: mode, SkipDefaultResource: true})
			must(err)
			defer func() {
				cleanup, stop := context.WithTimeout(context.Background(), 45*time.Second)
				defer stop()
				if err := service.Delete(cleanup, name); err != nil {
					t.Errorf("owned cleanup requires recovery: %v", err)
				}
			}()
			inspect := func() {
				output, err := runtime.runner.Run(ctx, "incus", "exec", env.RuntimeRef, "--project", runtime.project, "--", "/bin/sh", "-ec", "systemctl show --property=ActiveState --value hacocoon-dns.service; cat /etc/resolv.conf")
				must(err)
				if output.ExitCode != 0 || output.StdoutTruncated {
					t.Fatal("incomplete state probe")
				}
				active := strings.HasPrefix(output.Stdout, "active\n")
				if active != (mode != core.DNSDisabled) || !strings.Contains(output.Stdout, "nameserver 127.0.0.1\n") {
					t.Fatal("wrong managed resolver state", output.Stdout)
				}
			}
			inspect()
			must(service.Stop(ctx, name))
			must(service.Start(ctx, name))
			inspect()
			if mode == core.DNSBackend {
				instance, err := catalog.EnvironmentInstance(ctx, env)
				must(err)
				lookup, stop := context.WithTimeout(ctx, 10*time.Second)
				defer stop()
				addresses, err := runtime.ResolveEnvironmentName(lookup, env.RuntimeRef, instance, "example.com")
				must(err)
				if len(addresses) == 0 {
					t.Fatal("backend returned no answers")
				}
			}
		})
	}
}
