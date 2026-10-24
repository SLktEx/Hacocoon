package hostsetup

import "github.com/SLktEx/Hacocoon/internal/core"

// PackageFailure carries only fixed package-operation classifications. It does
// not retain a command, package name, process output or original error message.
type PackageFailure struct{ Phase, Kind string }

func (e *PackageFailure) reason() string {
	if e == nil {
		return "unavailable"
	}
	switch e.Phase {
	case "query", "update", "install", "git_verify", "gh_verify":
	default:
		return "unavailable"
	}
	switch e.Kind {
	case "timeout", "exit", "other":
		return "packages_" + e.Phase + "_" + e.Kind
	default:
		return "unavailable"
	}
}

func (e *PackageFailure) Error() string { return e.reason() }
func (e *PackageFailure) Unwrap() error { return core.ErrRuntimeUnavailable }
