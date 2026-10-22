package basebuild

import (
	"strings"
	"testing"
)

func TestArchiveLimitLargeAndInvalid(t *testing.T) {
	for _, size := range []int64{65 << 30, 500 << 30, 2 << 40, MaxArchiveLimitBytes} {
		request := ImportRequest{Name: "tools", MaxBytes: size, Artifact: &Artifact{ID: strings.Repeat("a", 32), SHA256: strings.Repeat("b", 64), Size: size, Architecture: "x86_64"}}
		if err := request.Validate(); err != nil {
			t.Fatal(size, err)
		}
		request.Artifact.Size++
		if request.Validate() == nil {
			t.Fatal("accepted over-budget image", size)
		}
	}
	for _, size := range []int64{2 << 40, 100 << 40, MaxArchiveLimitBytes} {
		req := ImportRequest{Name: "tools", Artifact: &Artifact{ID: strings.Repeat("a", 32), SHA256: strings.Repeat("b", 64), Size: size, Architecture: "x86_64"}}
		if err := req.Validate(); err != nil {
			t.Fatal("default imposed an image cap", size, err)
		}
	}
	for _, limit := range []int64{-1, MaxArchiveLimitBytes + 1} {
		if (ImportRequest{Name: "tools", MaxBytes: limit}).Validate() == nil {
			t.Fatal("invalid budget", limit)
		}
	}
	if (ImportRequest{}).ArchiveLimit() != MaxArchiveLimitBytes {
		t.Fatal("default changed")
	}
}
