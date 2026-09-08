package incus

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestProvisionTrustedHostProductClientPublishesOnlyVerifiedStaging(t *testing.T) {
	for _, corrupt := range []bool{false, true} {
		t.Run(fmt.Sprint(corrupt), func(t *testing.T) {
			source := writeTrustedClientFixture(t, 0755)
			payload, err := os.ReadFile(source)
			if err != nil {
				t.Fatal(err)
			}
			digest := fmt.Sprintf("%x", sha256.Sum256(payload))
			files := map[string]string{trustedHostProductClientPath: "previous"}
			published := false
			cleanup := false
			runner := trustedHostRunner("RUNNING", trustedHostRoleValue, nil)
			original := runner.run
			runner.run = func(ctx context.Context, n int, name string, args []string) (host.Result, error) {
				if len(args) >= 4 && args[0] == "file" && args[1] == "push" {
					target := strings.TrimPrefix(args[3], trustedHostName)
					if target == trustedHostProductClientPath || !strings.HasPrefix(target, "/usr/local/bin/.haco-client-") {
						t.Fatal("direct or unscoped executable write")
					}
					files[target] = digest
					if corrupt {
						files[target] = "corrupt"
					}
					return host.Result{}, nil
				}
				if len(args) >= 7 && args[0] == "exec" && args[5] == "sha256sum" {
					value, ok := files[args[6]]
					if !ok {
						return host.Result{}, errors.New("missing")
					}
					return host.Result{Stdout: value + "  " + args[6]}, nil
				}
				if len(args) >= 9 && args[0] == "exec" && args[5] == "stat" {
					return host.Result{Stdout: "755:0:0"}, nil
				}
				if len(args) >= 10 && args[0] == "exec" && args[5] == "mv" {
					if files[args[8]] != digest || args[9] != trustedHostProductClientPath || args[6] != "-T" || args[7] != "--" {
						t.Fatal("unverified or unsafe publication")
					}
					files[args[9]] = files[args[8]]
					delete(files, args[8])
					published = true
					return host.Result{}, nil
				}
				if len(args) >= 6 && args[0] == "exec" {
					switch args[5] {
					case "mkdir":
						return host.Result{}, nil
					case "rm":
						delete(files, args[8])
						return host.Result{}, nil
					case "rmdir":
						cleanup = true
						return host.Result{}, nil
					}
				}
				return original(ctx, n, name, args)
			}
			err = New(runner).ProvisionTrustedHostProductClient(context.Background(), source)
			if corrupt {
				if err == nil || published || files[trustedHostProductClientPath] != "previous" {
					t.Fatal("failed staging changed installed binary")
				}
			} else if err != nil || !published || files[trustedHostProductClientPath] != digest {
				t.Fatalf("publication failed: %v", err)
			}
			if !cleanup {
				t.Fatal("staging directory not cleaned")
			}
		})
	}
}
