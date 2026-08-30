package seedbuild

import (
	"reflect"
	"testing"

	ociplugin "github.com/SLktEx/Hacocoon/modules/plugin/oci"
)

func TestSelectedImagesIncludesOperatorPinsWithoutRelabelingAsAutomatic(t *testing.T) {
	digestA := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	digestB := "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	images, err := selectedImages([]ociplugin.Recommendation{
		{Reference: "example.invalid/auto:latest", Digest: digestA, AutoPromote: true},
		{Reference: "example.invalid/pinned:latest", Digest: digestB, Pinned: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []ImageIdentity{
		{Reference: "example.invalid/auto:latest", Digest: digestA},
		{Reference: "example.invalid/pinned:latest", Digest: digestB},
	}
	if !reflect.DeepEqual(images, want) {
		t.Fatalf("images=%#v want=%#v", images, want)
	}
}
