package composition

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/env/creation"
	"github.com/SLktEx/Hacocoon/internal/host/setup"
	"github.com/SLktEx/Hacocoon/internal/state"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSetupDownloadsBeforeSettingInitialDefault(t *testing.T) {
	ctx := context.Background()
	store := state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "environments.json"))
	downloads := 0
	a := &App{Creation: &creation.Service{Catalog: store}, InitialImage: func(context.Context) (core.BaseName, error) { downloads++; return "standard", nil }}
	if err := a.initializeDefaultImage(ctx); err != nil {
		t.Fatal(err)
	}
	if got, err := store.DefaultImage(ctx); err != nil || got != "standard" || downloads != 1 {
		t.Fatal(got, err, downloads)
	}
	if err := store.SetDefaultImage(ctx, "custom", false); err != nil {
		t.Fatal(err)
	}
	if err := a.initializeDefaultImage(ctx); err != nil {
		t.Fatal(err)
	}
	if got, _ := store.DefaultImage(ctx); got != "custom" || downloads != 1 {
		t.Fatal("setup overwrote preference", got)
	}
}

type defaultImageProgressCatalog struct {
	creation.Catalog
	read  func(context.Context) (core.BaseName, error)
	write func(context.Context, core.BaseName, bool) error
}

func (c defaultImageProgressCatalog) DefaultImage(ctx context.Context) (core.BaseName, error) {
	return c.read(ctx)
}

func (c defaultImageProgressCatalog) SetDefaultImage(ctx context.Context, image core.BaseName, initial bool) error {
	return c.write(ctx, image, initial)
}

func TestDefaultImageCatalogProgressPreservesOrderAndFailure(t *testing.T) {
	for _, failureStage := range []string{"", "default_image_read", "default_image_write", "existing"} {
		t.Run(failureStage, func(t *testing.T) {
			failure := &os.PathError{Op: "SECRET-operation", Path: "SECRET-catalog-path-and-value", Err: errors.New("SECRET-cause")}
			var calls []string
			catalog := defaultImageProgressCatalog{
				read: func(context.Context) (core.BaseName, error) {
					calls = append(calls, "read")
					if failureStage == "default_image_read" {
						return "", failure
					}
					if failureStage == "existing" {
						return "SECRET-existing-preference", nil
					}
					return "", nil
				},
				write: func(_ context.Context, image core.BaseName, initial bool) error {
					calls = append(calls, "write")
					if image != "SECRET-acquired-image" || !initial {
						t.Fatal("changed persistence arguments")
					}
					if failureStage == "default_image_write" {
						return failure
					}
					return nil
				},
			}
			a := &App{Creation: &creation.Service{Catalog: catalog}, InitialImage: func(context.Context) (core.BaseName, error) {
				calls = append(calls, "acquire")
				return "SECRET-acquired-image", nil
			}}
			var events []hostsetup.Event
			ctx := hostsetup.Observe(context.Background(), func(e hostsetup.Event) { events = append(events, e) })
			err := a.initializeDefaultImage(ctx)
			wantCalls := []string{"read", "acquire", "write"}
			wantStages := []string{"default_image_read", "default_image_write"}
			if failureStage == "default_image_read" || failureStage == "existing" {
				wantCalls, wantStages = wantCalls[:1], wantStages[:1]
			}
			if !reflect.DeepEqual(calls, wantCalls) || len(events) != 2*len(wantStages) {
				t.Fatal(calls, events)
			}
			for n, stage := range wantStages {
				start, end := events[2*n], events[2*n+1]
				wantState, wantReason := "succeeded", ""
				if stage == failureStage {
					wantState, wantReason = "failed", "failed"
				}
				if start.Stage != stage || start.State != "running" || end.Stage != stage || end.State != wantState || end.Reason != wantReason || end.DurationMS < 0 {
					t.Fatal(events)
				}
			}
			if failureStage == "default_image_read" || failureStage == "default_image_write" {
				stage, reason := hostsetup.Details(err)
				var original *os.PathError
				if !errors.Is(err, failure) || !errors.As(err, &original) || original != failure || stage != failureStage || reason != "failed" || strings.Contains(err.Error(), "SECRET") {
					t.Fatal(stage, reason, err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			encoded, _ := json.Marshal(events)
			if strings.Contains(string(encoded), "SECRET") {
				t.Fatal("private catalog data escaped progress")
			}
		})
	}
}

func TestDefaultImageObservationDoesNotAddCancellationChecks(t *testing.T) {
	for _, cancelAt := range []string{"before", "read", "acquire", "write"} {
		t.Run(cancelAt, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if cancelAt == "before" {
				cancel()
			}
			var calls []string
			called := func(stage string) {
				calls = append(calls, stage)
				if stage == cancelAt {
					cancel()
				}
			}
			catalog := defaultImageProgressCatalog{
				read:  func(context.Context) (core.BaseName, error) { called("read"); return "", nil },
				write: func(context.Context, core.BaseName, bool) error { called("write"); return nil },
			}
			a := &App{Creation: &creation.Service{Catalog: catalog}, InitialImage: func(context.Context) (core.BaseName, error) { called("acquire"); return "standard", nil }}
			if err := a.initializeDefaultImage(ctx); err != nil || !reflect.DeepEqual(calls, []string{"read", "acquire", "write"}) {
				t.Fatal(err, calls)
			}
		})
	}
}

func TestDefaultImageCatalogPreservesCancellationCauses(t *testing.T) {
	for _, stage := range []string{"default_image_read", "default_image_write"} {
		for _, failure := range []error{context.Canceled, context.DeadlineExceeded} {
			t.Run(stage+"/"+hostsetup.Reason(failure), func(t *testing.T) {
				catalog := defaultImageProgressCatalog{
					read: func(context.Context) (core.BaseName, error) {
						if stage == "default_image_read" {
							return "", failure
						}
						return "", nil
					},
					write: func(context.Context, core.BaseName, bool) error { return failure },
				}
				a := &App{Creation: &creation.Service{Catalog: catalog}, InitialImage: func(context.Context) (core.BaseName, error) { return "standard", nil }}
				err := a.initializeDefaultImage(context.Background())
				gotStage, reason := hostsetup.Details(err)
				if !errors.Is(err, failure) || gotStage != stage || reason != hostsetup.Reason(failure) {
					t.Fatal(gotStage, reason, err)
				}
			})
		}
	}
}
func TestSetupDownloadFailureDoesNotSetDefault(t *testing.T) {
	ctx := context.Background()
	store := state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "environments.json"))
	a := &App{Creation: &creation.Service{Catalog: store}, InitialImage: func(context.Context) (core.BaseName, error) { return "", errors.New("download") }}
	if err := a.initializeDefaultImage(ctx); err == nil {
		t.Fatal("download failure swallowed")
	}
	if got, _ := store.DefaultImage(ctx); got != "" {
		t.Fatal(got)
	}
}
