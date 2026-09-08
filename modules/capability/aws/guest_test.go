package aws

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"io"
	"testing"
)

func TestGuestSourceCannotSelectOrAdoptAnotherEnvironment(t *testing.T) {
	const instance = "env-11111111111111111111111111111111"
	for _, operation := range []string{"list", "download"} {
		for _, scenario := range []string{"valid", "recreated", "missing-instance", "malformed-instance", "caller-environment", "other-environment"} {
			t.Run(operation+"/"+scenario, func(t *testing.T) {
				source := GuestSource{"dev", instance}
				spec := ListSpec{URL: "s3://example-bucket/project/data.bin"}
				switch scenario {
				case "recreated":
					source.Instance = "env-22222222222222222222222222222222"
				case "missing-instance":
					source.Instance = ""
				case "malformed-instance":
					source.Instance = "dev"
				case "caller-environment":
					spec.Environment = "dev"
				case "other-environment":
					source.Environment = "other"
				}
				authenticated, requested := 0, 0
				ordinary := fixtureHost(t, new(int))
				b := &Broker{Host: func(ctx context.Context, script string, input []byte) ([]byte, error) {
					authenticated++
					return ordinary(ctx, script, input)
				}, Environments: environments{}, Capabilities: requester(func(ctx context.Context, r core.CapabilityRequest) (core.CapabilityResult, error) {
					requested++
					if r.Environment != "dev" || r.EnvironmentInstance != instance {
						t.Fatal("source identity lost")
					}
					if operation == "download" && ctx.Value(downloadSinkKey{}) == nil {
						t.Fatal("download sink lost")
					}
					return core.CapabilityResult{}, nil
				})}
				var err error
				if operation == "list" {
					_, err = b.ListFromGuest(context.Background(), source, spec)
				} else {
					_, err = b.DownloadFromGuest(context.Background(), source, GetSpec(spec), io.Discard)
				}
				if scenario == "valid" {
					if err != nil || authenticated != 1 || requested != 1 {
						t.Fatal(err, authenticated, requested)
					}
				} else if err == nil || authenticated != 0 || requested != 0 {
					t.Fatal("guest identity bypass", err, authenticated, requested)
				}
				if scenario == "recreated" && !errors.Is(err, core.ErrCapabilityStale) {
					t.Fatal(err)
				}
			})
		}
	}
}
