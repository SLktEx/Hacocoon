package basebuild

import (
	"math"
	"regexp"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// No configured size cap by default. Native file offsets use signed int64;
// reserve one byte for the archive validators' overrun detection.
const MaxArchiveLimitBytes int64 = math.MaxInt64 - 1
const DefaultArchiveLimitBytes = MaxArchiveLimitBytes

// Artifact binds a transport upload to one exact operation, without exposing
// Packer concepts to the controller. All fields are verified before creation.
type Artifact struct {
	ID           string `json:"id"`
	SHA256       string `json:"sha256"`
	Size         int64  `json:"size"`
	Architecture string `json:"architecture"`
}
type ImportRequest struct {
	Name     core.BaseName `json:"name"`
	MaxBytes int64         `json:"max_bytes,omitempty"` // Zero means no configured size cap.
	Artifact *Artifact     `json:"artifact,omitempty"`
}

var artifactID = regexp.MustCompile("^[a-f0-9]{32}$")
var artifactDigest = regexp.MustCompile("^[a-f0-9]{64}$")

func (r ImportRequest) ArchiveLimit() int64 {
	if r.MaxBytes == 0 {
		return DefaultArchiveLimitBytes
	}
	return r.MaxBytes
}

func (r ImportRequest) Validate() error {
	if !NamePattern.MatchString(string(r.Name)) || r.MaxBytes < 0 || r.MaxBytes > MaxArchiveLimitBytes {
		return core.ErrInvalidArgument
	}
	if a := r.Artifact; a != nil {
		if !artifactID.MatchString(a.ID) || !artifactDigest.MatchString(a.SHA256) || a.Size <= 0 || a.Size > r.ArchiveLimit() || (a.Architecture != "x86_64" && a.Architecture != "aarch64") {
			return core.ErrInvalidArgument
		}
	}
	return nil
}
