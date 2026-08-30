package buildinfo

import (
	"runtime/debug"
	"strings"
)

// These values are overridden for release builds with linker flags.
// Checkpoint is generated from the authoritative development-checkpoint docs.
var (
	SoftwareVersion = "dev"
	Commit          = "unknown"
	BuildDate       = "unknown"
)

type Info struct {
	Checkpoint string `json:"checkpoint"`
	Version    string `json:"version"`
	Commit     string `json:"commit"`
	BuildDate  string `json:"build_date"`
}

func Current() Info {
	info := Info{
		Checkpoint: normalize(GeneratedCheckpoint, "unknown"),
		Version:    normalize(SoftwareVersion, "dev"),
		Commit:     normalize(Commit, "unknown"),
		BuildDate:  normalize(BuildDate, "unknown"),
	}

	// Plain `go build` does not inject release metadata. Preserve `dev` as the
	// software version, but use Go's embedded VCS revision when available so a
	// local binary can still be traced to source.
	if info.Commit == "unknown" {
		if bi, ok := debug.ReadBuildInfo(); ok {
			for _, setting := range bi.Settings {
				if setting.Key == "vcs.revision" && strings.TrimSpace(setting.Value) != "" {
					info.Commit = setting.Value
					break
				}
			}
		}
	}
	return info
}

func normalize(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func ShortCommit(commit string) string {
	commit = strings.TrimSpace(commit)
	if len(commit) > 12 {
		return commit[:12]
	}
	if commit == "" {
		return "unknown"
	}
	return commit
}
