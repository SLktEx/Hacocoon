package cli

import (
	"regexp"

	"github.com/SLktEx/Hacocoon/internal/controller/api"
)

// Select once from recorded capture times. The canonical restore subsequently
// rechecks this exact ID; a deletion or failed restore never selects another save.
func selectLatestSnapshot(saved []controlapi.SnapshotSummary, environment string) (controlapi.SnapshotSummary, string) {
	var latest controlapi.SnapshotSummary
	tied := false
	for _, item := range saved {
		if item.Environment != environment || item.State != "ready" {
			continue
		}
		if item.CreatedAt.IsZero() || !regexp.MustCompile(`^snap-[a-f0-9]{32}$`).MatchString(item.ID) {
			return controlapi.SnapshotSummary{}, "snapshot.latest_unknown"
		}
		if latest.ID == "" || item.CreatedAt.After(latest.CreatedAt) {
			latest = item
			tied = false
		} else if item.CreatedAt.Equal(latest.CreatedAt) {
			tied = true
		}
	}
	if latest.ID == "" {
		return latest, "snapshot.latest_none"
	}
	if tied {
		return controlapi.SnapshotSummary{}, "snapshot.latest_unknown"
	}
	return latest, ""
}
