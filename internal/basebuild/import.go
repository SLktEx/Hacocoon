package basebuild

import "github.com/SLktEx/Hacocoon/internal/core"

const MaxArchiveBytes int64 = 64 << 30

type ImportRequest struct {
	Name core.BaseName `json:"name"`
}

func (r ImportRequest) Validate() error {
	if !NamePattern.MatchString(string(r.Name)) {
		return core.ErrInvalidArgument
	}
	return nil
}
