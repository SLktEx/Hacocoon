package winprocess

import (
	"context"
	"errors"
	"testing"
)

func TestRootExitWaitRequiresObservedSuccessfulExit(t *testing.T) {
	for _, code := range []uint32{0, 1, 259, 0xffffffff} {
		calls := 0
		err := waitRootExit(context.Background(), func() (bool, uint32, error) {
			calls++
			return calls == 2, code, nil
		})
		if (err == nil) != (code == 0) || calls != 2 {
			t.Fatal(code, calls, err)
		}
	}
}

func TestRootExitWaitPreservesCancellationAndObservationFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitRootExit(ctx, func() (bool, uint32, error) { t.Fatal("observed after cancellation"); return false, 0, nil }); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	if err := waitRootExit(ctx, func() (bool, uint32, error) { cancel(); return true, 0, nil }); !errors.Is(err, context.Canceled) {
		t.Fatal("exit success overrode cancellation", err)
	}
	cause := errors.New("closed native process handle")
	if err := waitRootExit(context.Background(), func() (bool, uint32, error) { return false, 0, cause }); !errors.Is(err, cause) {
		t.Fatal(err)
	}
}
