package incus

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/core"
	"strings"
)

// Incus deletes a custom volume's child snapshots and backups with the parent.
// Callers first establish exact ownership and exclude active users; this native
// check is shared by Workspace and OCI Store deletion, including failure cleanup.
func (r *Runtime) checkVolumeSavedObjects(ctx context.Context, pool, name string, config map[string]string) error {
	if !safeIncusRef(pool) || !safeIncusRef(name) || config == nil {
		return core.ErrInvalidArgument
	}
	if strings.TrimSpace(config["snapshots.schedule"]) != "" {
		return fmt.Errorf("native snapshot schedule prevents volume deletion: %w", core.ErrStorageBusy)
	}
	for _, kind := range []string{"snapshots", "backups"} {
		out, err := r.runner.Run(ctx, "incus", "query", "/1.0/storage-pools/"+pool+"/volumes/custom/"+name+"/"+kind+"?project="+r.project)
		if err != nil || out.ExitCode != 0 || out.StdoutTruncated {
			return core.ErrRuntimeUnavailable
		}
		var saved []string
		if json.Unmarshal([]byte(out.Stdout), &saved) != nil || saved == nil {
			return core.ErrIncompatibleState
		}
		if len(saved) > 0 {
			return fmt.Errorf("native %s prevent volume deletion: %w", kind, core.ErrStorageBusy)
		}
	}
	return nil
}
