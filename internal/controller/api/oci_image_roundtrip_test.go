package controlapi

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/storage/oci"
)

type imageAPIService struct {
	mu      sync.Mutex
	listed  []string
	deleted []oci.ImageTarget
	ids     []string
}

func (s *imageAPIService) ListHost(ctx context.Context, runtime string) (oci.ManagedImageList, error) {
	return s.list(ctx, "host", runtime)
}
func (s *imageAPIService) List(ctx context.Context, environment, runtime string) (oci.ManagedImageList, error) {
	return s.list(ctx, environment, runtime)
}
func (s *imageAPIService) list(ctx context.Context, name, runtime string) (oci.ManagedImageList, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > 5*time.Minute {
		return oci.ManagedImageList{}, core.ErrInvalidArgument
	}
	s.listed = append(s.listed, name+":"+runtime)
	if name == "missing" {
		return oci.ManagedImageList{}, core.ErrNotFound
	}
	target := oci.ImageTarget{Environment: name, Instance: "env-" + strings.Repeat("a", 32), Store: core.PersistentResourceRef{ID: "oci:dev", Owner: strings.Repeat("b", 32)}, Runtime: runtime}
	if name == "host" {
		target.Host = true
		target.Environment = ""
		target.Instance = ""
		target.Store.ID = oci.HostStoreID
	}
	return oci.ManagedImageList{Target: target, Images: []oci.ManagedImage{{ID: "sha256:" + strings.Repeat("c", 64), Tags: []string{"example:dev"}, Digests: []string{}, Containers: []string{}}}, IndependentSnapshots: []string{"saved-independent-copy"}}, nil
}
func (s *imageAPIService) Delete(_ context.Context, target oci.ImageTarget, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleted = append(s.deleted, target)
	s.ids = append(s.ids, id)
	if target.Host {
		return core.ErrStorageBusy
	}
	return nil
}

func TestOCIImageWirePreservesReviewedTargetAndSnapshotObservation(t *testing.T) {
	service := &imageAPIService{}
	path := doctorTestSocket(t, func(s *control.Server) {
		if err := RegisterOCIImages(s, service); err != nil {
			t.Fatal(err)
		}
	})
	client, err := NewClient(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	var targets []oci.ImageTarget
	for _, host := range []bool{false, true} {
		request := OCIImageRequest{Operation: "list", Host: host, Runtime: "docker"}
		if !host {
			request.Environment = "dev"
			request.Runtime = "nerdctl"
		}
		result, err := client.OCIImage(ctx, request)
		if err != nil || len(result.Images) != 1 || !reflect.DeepEqual(result.IndependentSnapshots, []string{"saved-independent-copy"}) || result.Target.Host != host || result.Target.Runtime != request.Runtime {
			t.Fatal("image listing lost reviewed identity", result, err)
		}
		targets = append(targets, result.Target)
		_, err = client.OCIImage(ctx, OCIImageRequest{Operation: "delete", Target: result.Target, ID: result.Images[0].ID})
		if host {
			var status *control.StatusError
			if !errors.As(err, &status) || status.Code != "busy" {
				t.Fatal("busy image deletion reported success", err)
			}
		} else if err != nil {
			t.Fatal(err)
		}
	}
	var status *control.StatusError
	if _, err := client.OCIImage(ctx, OCIImageRequest{Operation: "list", Environment: "missing", Runtime: "docker"}); !errors.As(err, &status) || status.Code != "not_found" {
		t.Fatal(err)
	}
	for _, request := range []OCIImageRequest{{Operation: "unknown"}, {Operation: "delete", Target: targets[0], ID: "example:mutable-tag"}} {
		if _, err := client.OCIImage(ctx, request); err == nil {
			t.Fatal("unreviewed image operation accepted", request)
		}
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	if !reflect.DeepEqual(service.listed, []string{"dev:nerdctl", "host:docker", "missing:docker"}) || !reflect.DeepEqual(service.deleted, targets) || !reflect.DeepEqual(service.ids, []string{"sha256:" + strings.Repeat("c", 64), "sha256:" + strings.Repeat("c", 64)}) {
		t.Fatal("wire changed native deletion target", service.listed, service.deleted, service.ids)
	}
}
