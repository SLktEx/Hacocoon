package bytesize

import (
	"github.com/SLktEx/Hacocoon/internal/core"
	"math"
	"strconv"
	"strings"
)

// Parse accepts a positive integer with an explicit binary byte unit.
func Parse(raw string, max uint64) (uint64, error) {
	units := []struct {
		suffix string
		mult   uint64
	}{
		{"TiB", 1 << 40},
		{"GiB", 1 << 30},
		{"MiB", 1 << 20},
		{"KiB", 1 << 10},
		{"B", 1},
	}
	for _, unit := range units {
		if !strings.HasSuffix(raw, unit.suffix) {
			continue
		}
		number := strings.TrimSuffix(raw, unit.suffix)
		if number == "" {
			return 0, core.ErrInvalidArgument
		}
		for _, ch := range number {
			if ch < '0' || ch > '9' {
				return 0, core.ErrInvalidArgument
			}
		}
		value, err := strconv.ParseUint(number, 10, 64)
		if err != nil || value == 0 || value > math.MaxUint64/unit.mult {
			return 0, core.ErrInvalidArgument
		}
		value *= unit.mult
		if value > max {
			return 0, core.ErrInvalidArgument
		}
		return value, nil
	}
	return 0, core.ErrInvalidArgument
}
