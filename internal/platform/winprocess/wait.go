package winprocess

import (
	"context"
	"fmt"
	"time"
)

// Root exit alone never proves Windows descendant absence. Close owns that
// separate observation, including after a successful root exit.
func waitRootExit(ctx context.Context, observe func() (bool, uint32, error)) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		exited, code, err := observe()
		if err != nil {
			return err
		}
		if exited {
			if code != 0 {
				return fmt.Errorf("private process exited with code %d", code)
			}
			return ctx.Err()
		}
		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
