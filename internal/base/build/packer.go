package basebuild

import (
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const MaxContextBytes = 512 << 10
const MaxContextFiles = 128

type SourceFile struct {
	Path string `json:"path"`
	Data []byte `json:"data"`
}
type PackerTemplate struct {
	Files []SourceFile `json:"files"`
}

var sourceComponent = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]*$`)

func ValidSourcePath(name string) bool {
	if len(name) == 0 || len(name) > 240 || path.Clean(name) != name {
		return false
	}
	parts := strings.Split(name, "/")
	if len(parts) > 8 {
		return false
	}
	for _, part := range parts {
		if !sourceComponent.MatchString(part) {
			return false
		}
	}
	return true
}

func (p PackerTemplate) Validate() error {
	if len(p.Files) == 0 || len(p.Files) > MaxContextFiles {
		return core.ErrInvalidArgument
	}
	seen := make(map[string]bool)
	total := 0
	hcl := false
	for _, f := range p.Files {
		if !ValidSourcePath(f.Path) || seen[f.Path] {
			return core.ErrInvalidArgument
		}
		for parent := path.Dir(f.Path); parent != "."; parent = path.Dir(parent) {
			if seen[parent] {
				return core.ErrInvalidArgument
			}
		}
		for name := range seen {
			if strings.HasPrefix(name, f.Path+"/") {
				return core.ErrInvalidArgument
			}
		}
		seen[f.Path] = true
		total += len(f.Data)
		if total > MaxContextBytes {
			return core.ErrInvalidArgument
		}
		if !strings.Contains(f.Path, "/") && strings.HasSuffix(f.Path, ".pkr.hcl") {
			hcl = true
		}
	}
	if !hcl {
		return core.ErrInvalidArgument
	}
	return nil
}

// Stage is selected by the trusted orchestration, never guest output. Execution
// is a separate private result so it cannot leak through Error/log wrapping.
type ProvisionFailure struct {
	Stage     string
	Execution core.ExecutionResult
}

func (e *ProvisionFailure) Error() string {
	return fmt.Sprintf("Packer %s failed (exit %d)", e.Stage, e.Execution.ExitCode)
}
func (e *ProvisionFailure) Unwrap() error { return core.ErrRuntimeUnavailable }
