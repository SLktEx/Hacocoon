package main

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/modules/plugin/oci"
	"strings"
	"testing"
)

type imageReviewClient struct {
	result   oci.ManagedImageList
	requests []controlapi.OCIImageRequest
	fail     bool
}

func (c *imageReviewClient) OCIImage(_ context.Context, r controlapi.OCIImageRequest) (oci.ManagedImageList, error) {
	c.requests = append(c.requests, r)
	if r.Operation == "delete" && c.fail {
		return oci.ManagedImageList{}, errors.New("runtime refused")
	}
	return c.result, nil
}
func TestImageReviewConfirmationAndImmutableDeletion(t *testing.T) {
	for _, mode := range []string{"yes", "flag", "no", "eof", "oversize", "busy", "failure", "duplicate", "stale-target"} {
		t.Run(mode, func(t *testing.T) {
			id := "sha256:" + strings.Repeat("a", 64)
			c := &imageReviewClient{result: oci.ManagedImageList{Target: oci.ImageTarget{Environment: "dev", Instance: "env-" + strings.Repeat("b", 32), Store: core.PersistentResourceRef{ID: "oci:dev", Owner: strings.Repeat("c", 32)}, Runtime: "nerdctl"}, Images: []oci.ManagedImage{{ID: id, Tags: []string{"app:dev"}}}}}
			args := []string{"delete", "dev", "app:dev"}
			input := "yes\n"
			switch mode {
			case "flag":
				args = []string{"delete", "--yes", "dev", "app:dev"}
			case "no":
				input = "no\n"
			case "eof":
				input = "yes"
			case "oversize":
				input = strings.Repeat(" ", 128) + "yes\n"
			case "busy":
				c.result.Images[0].Containers = []string{"user"}
			case "failure":
				c.fail = true
			case "duplicate":
				c.result.Images = append(c.result.Images, c.result.Images[0])
			case "stale-target":
				c.result.Target.Environment = "other"
			}
			var out, diagnostic strings.Builder
			code := ociImageManageCommand(context.Background(), c, args, strings.NewReader(input), &out, &diagnostic)
			success := mode == "yes" || mode == "flag"
			if (code == 0) != success {
				t.Fatalf("code %d: %s", code, diagnostic.String())
			}
			shouldDelete := success || mode == "failure"
			if (len(c.requests) == 2) != shouldDelete {
				t.Fatalf("requests: %+v", c.requests)
			}
			if shouldDelete && (c.requests[1].ID != id || c.requests[1].Target != c.result.Target) {
				t.Fatal("review identity lost")
			}
			if !success && strings.Contains(out.String(), "OCI image deleted") {
				t.Fatal("failure reported successful")
			}
		})
	}
}

func TestHostImageConfirmationKeepsSourceIdentity(t *testing.T) {
	id := "sha256:" + strings.Repeat("a", 64)
	for _, mode := range []string{"valid", "extra-env", "wrong-role"} {
		t.Run(mode, func(t *testing.T) {
			c := &imageReviewClient{result: oci.ManagedImageList{Target: oci.ImageTarget{Host: true, Store: core.PersistentResourceRef{ID: oci.HostStoreID, Owner: strings.Repeat("c", 32)}, Runtime: "docker"}, Images: []oci.ManagedImage{{ID: id, Tags: []string{"app:dev"}}}}}
			args := []string{"delete", "--host", "--runtime", "docker", "--yes", "app:dev"}
			if mode == "extra-env" {
				args = append(args, "dev")
			}
			if mode == "wrong-role" {
				c.result.Target.Host = false
			}
			var out, diagnostic strings.Builder
			code := ociImageManageCommand(context.Background(), c, args, strings.NewReader(""), &out, &diagnostic)
			if mode == "valid" {
				if code != 0 || len(c.requests) != 2 || !c.requests[0].Host || c.requests[0].Environment != "" || c.requests[1].Target != c.result.Target || c.requests[1].ID != id || !strings.Contains(out.String(), "Host source") {
					t.Fatalf("host review: code=%d requests=%+v output=%s diagnostic=%s", code, c.requests, out.String(), diagnostic.String())
				}
			} else if code == 0 || len(c.requests) > 1 {
				t.Fatalf("mixed authority accepted: %d %+v", code, c.requests)
			}
		})
	}
}
