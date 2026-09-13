package composition

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/recipes"
	"github.com/SLktEx/Hacocoon/modules/runtime/incus"
)

func TestHostShellWaitsForSetupWithoutReleasingItsExclusion(t *testing.T) {
	a := &App{Runtime: incus.New(nil)}
	release, err := a.acquireHostSetup(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err = a.PrepareTrustedHostShellStream(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("shell should wait for setup until its deadline, got %v", err)
	}
	if err := a.SetupHost(context.Background(), recipes.Update{}); err == nil || err.Error() != "Host setup is busy" {
		t.Fatalf("cancelled shell released active setup exclusion: %v", err)
	}
}

func TestHostSetupWaiterAcquiresOnlyAfterOperationCompletes(t *testing.T) {
	a := &App{}
	release, err := a.acquireHostSetup(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	acquired := make(chan func(), 1)
	errorsFound := make(chan error, 1)
	go func() {
		next, err := a.acquireHostSetup(ctx, true)
		if err != nil {
			errorsFound <- err
			return
		}
		acquired <- next
	}()
	select {
	case next := <-acquired:
		next()
		t.Fatal("overlapping setup acquired exclusion")
	case <-time.After(20 * time.Millisecond):
	}
	release()
	select {
	case next := <-acquired:
		next()
	case err := <-errorsFound:
		t.Fatal(err)
	case <-ctx.Done():
		t.Fatal("waiter did not resume after completed operation")
	}
	if next, err := a.acquireHostSetup(context.Background(), false); err != nil {
		t.Fatal(err)
	} else {
		next()
	}
}
