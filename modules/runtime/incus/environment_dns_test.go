package incus

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestEnvironmentDNSProvisioningRequiresOwnedTargetAndVerifiedCompanion(t *testing.T) {
	for _, scenario := range []string{"success", "foreign", "hash-mismatch", "setup-failure", "invalid-ref", "disabled"} {
		t.Run(scenario, func(t *testing.T) {
			source := filepath.Join(t.TempDir(), "haco")
			if err := os.WriteFile(source, []byte("companion fixture"), 0755); err != nil {
				t.Fatal(err)
			}
			digest := fmt.Sprintf("%x", sha256.Sum256([]byte("companion fixture")))
			pushes, setups, calls := 0, 0, 0
			r := New(&fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
				calls++
				if name != "incus" {
					t.Fatal("unexpected Host executable")
				}
				if args[0] == "config" {
					marker := managedEnvironmentMarkerValue
					if scenario == "foreign" {
						marker = "foreign"
					}
					return host.Result{Stdout: marker}, nil
				}
				if args[0] == "file" {
					pushes++
					if args[2] != source || args[3] != "haco-demo/usr/local/libexec/hacocoon-dns.next" {
						t.Fatal("wrong copy target")
					}
					return host.Result{}, nil
				}
				if args[0] == "exec" && args[5] == "sha256sum" {
					hash := digest
					if scenario == "hash-mismatch" {
						hash = strings.Repeat("0", 64)
					}
					return host.Result{Stdout: hash + "  /usr/local/libexec/hacocoon-dns.next"}, nil
				}
				if args[0] == "exec" && args[len(args)-1] == environmentDNSSetup {
					setups++
					if scenario == "setup-failure" {
						return host.Result{}, fmt.Errorf("unit failed")
					}
					return host.Result{}, nil
				}
				t.Fatalf("unexpected operation: %v", args)
				return host.Result{}, nil
			}})
			if scenario != "disabled" {
				if err := r.ConfigureEnvironmentDNS(source); err != nil {
					t.Fatal(err)
				}
			}
			ref := "haco-demo"
			if scenario == "invalid-ref" {
				ref = "--project=foreign"
			}
			err := r.provisionEnvironmentDNS(context.Background(), ref)
			success := scenario == "success" || scenario == "disabled"
			if (err == nil) != success {
				t.Fatalf("error=%v", err)
			}
			if (scenario == "disabled" || scenario == "invalid-ref") && calls != 0 {
				t.Fatal("called provider before validation")
			}
			if scenario == "foreign" && pushes != 0 {
				t.Fatal("copied into foreign runtime")
			}
			if scenario == "hash-mismatch" && setups != 0 {
				t.Fatal("executed unverified companion")
			}
			if scenario == "success" && (pushes != 1 || setups != 1) {
				t.Fatal("missing automatic provisioning")
			}
		})
	}
}

func TestDNSSetupFailureStageDoesNotExposeGuestOutput(t *testing.T) {
	for input, want := range map[string]string{
		"secret=do-not-return":                                "unknown",
		"HACO_DNS_STAGE=restart\nHACO_DNS_UNIT_EXIT=226\n":    "restart; service_exit=226",
		"HACO_DNS_STAGE=restart\nHACO_DNS_UNIT_EXIT=secret\n": "restart",
		"HACO_DNS_STAGE=restart\n":                            "restart",
		"HACO_DNS_STAGE=restart secret=do-not-return":         "unknown",
		"HACO_DNS_STAGE=resolver\nsecret=do-not-return":       "resolver",
	} {
		if got := dnsSetupFailureStage(input); got != want {
			t.Fatalf("stage=%q want %q", got, want)
		}
	}
}

func TestEnvironmentDNSSetupShellSyntax(t *testing.T) {
	command := exec.Command("/bin/sh", "-n")
	command.Stdin = strings.NewReader(environmentDNSSetup)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("invalid DNS setup shell: %v: %s", err, output)
	}
}
