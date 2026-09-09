package main

import (
	"bytes"
	"context"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/modules/plugin/oci"
	"strings"
	"testing"
)

type reviewedOCIClient struct {
	response controlapi.OCIStoreResponse
	requests []controlapi.OCIStoreRequest
	err      error
}

func (c *reviewedOCIClient) OCIStore(_ context.Context, r controlapi.OCIStoreRequest) (controlapi.OCIStoreResponse, error) {
	c.requests = append(c.requests, r)
	if r.Operation == "delete" {
		return controlapi.OCIStoreResponse{}, c.err
	}
	return c.response, nil
}
func TestOCIStoreReviewedDelete(t *testing.T) {
	for _, mode := range []string{"yes", "no", "eof", "oversize", "busy", "copy", "source", "creating", "missing-use", "duplicate", "failed", "machine"} {
		t.Run(mode, func(t *testing.T) {
			r := core.PersistentResource{ID: "oci:dev", Owner: strings.Repeat("a", 32), Kind: oci.StoreKind, State: "ready"}
			u := oci.StoreUse{Resource: r.Ref(), Role: "independent-store", IndependentSnapshots: []string{"saved"}}
			input := "yes\n"
			args := []string{"delete", "dev"}
			switch mode {
			case "no":
				input = "no\n"
			case "eof":
				input = "yes"
			case "oversize":
				input = strings.Repeat(" ", 128) + "yes\n"
			case "busy":
				u.Environments = []string{"env"}
			case "copy":
				u.PendingCopies = []string{"oci:copy"}
			case "source":
				r.SourceOnly = true
			case "creating":
				r.State = "creating"
			case "failed":
				args = []string{"delete", "--yes", "dev"}
			case "machine":
				args = []string{"list", "--json"}
			}
			c := &reviewedOCIClient{response: controlapi.OCIStoreResponse{Resources: []core.PersistentResource{r}, Uses: []oci.StoreUse{u}}}
			if mode == "missing-use" {
				c.response.Uses = nil
			}
			if mode == "duplicate" {
				c.response.Resources = append(c.response.Resources, r)
			}
			if mode == "failed" {
				c.err = core.ErrRecoveryRequired
			}
			var out, diagnostic bytes.Buffer
			code := ociStoreManageCommand(context.Background(), c, args, strings.NewReader(input), &out, &diagnostic)
			deleted := len(c.requests) == 2
			wantDelete := mode == "yes" || mode == "failed"
			if deleted != wantDelete {
				t.Fatalf("requests=%+v code=%d output=%s", c.requests, code, diagnostic.String())
			}
			if deleted && c.requests[1].Owner != r.Owner {
				t.Fatal("reviewed owner not sent")
			}
			if (code == 0) != (mode == "yes" || mode == "machine") {
				t.Fatalf("code=%d output=%s", code, diagnostic.String())
			}
			if mode == "failed" && strings.Contains(out.String(), "OCI Store deleted") {
				t.Fatal("failure reported as success")
			}
			if mode == "yes" && !strings.Contains(out.String(), "saved") {
				t.Fatal("independent snapshot impact missing")
			}
		})
	}
}
