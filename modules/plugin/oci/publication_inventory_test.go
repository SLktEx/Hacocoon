package oci

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"strings"
	"testing"
)

type inventorySource struct {
	result host.Result
	err    error
	calls  int
}

func (s *inventorySource) LocalOCIImages(context.Context, string) (host.Result, error) {
	s.calls++
	return s.result, s.err
}
func TestPublicationInventoryKeepsBuiltAndDanglingImages(t *testing.T) {
	id := "sha256:" + strings.Repeat("a", 64)
	built := "local/build\tdev\t" + id + "\t<none>\n"
	dangling := "<none>\t<none>\t" + id + "\t<none>\n"
	for _, driver := range []Driver{DriverDocker, DriverNerdctl} {
		source := &inventorySource{result: host.Result{Stdout: built + dangling}}
		got, err := ReadPublicationInventory(context.Background(), source, driver)
		if err != nil || len(got.Images) != 2 || got.Images[1].ID != id || got.Images[1].Reference != "local/build:dev" || got.Images[1].Digest != "" {
			t.Fatalf("built image lost: %+v %v", got, err)
		}
		source.result.Stdout = dangling + built + built
		reordered, err := ReadPublicationInventory(context.Background(), source, driver)
		if err != nil || got.Revision != reordered.Revision {
			t.Fatal("order or duplicate changes revision")
		}
		source.result.Stdout = strings.ReplaceAll(built+dangling, strings.Repeat("a", 64), strings.Repeat("b", 64))
		changed, err := ReadPublicationInventory(context.Background(), source, driver)
		if err != nil || changed.Revision == got.Revision {
			t.Fatal("tag movement not detected")
		}
	}
	docker, _ := parsePublicationInventory(built, DriverDocker)
	nerdctl, _ := parsePublicationInventory(built, DriverNerdctl)
	if docker.Revision == nerdctl.Revision {
		t.Fatal("runtime formats conflated")
	}
}
func TestPublicationInventoryRejectsAmbiguousOrHostileRows(t *testing.T) {
	id := "sha256:" + strings.Repeat("a", 64)
	valid := "example\tlatest\t" + id + "\t<none>\n"
	for _, row := range []string{
		"garbage", " \n", "-option\tlatest\t" + id + "\t<none>\n",
		"example\t$(id)\t" + id + "\t<none>\n", "example\tlatest\tshort\t<none>\n",
		"example\t<none>\t" + id + "\t<none>\n", "<none>\tlatest\t" + id + "\t<none>\n",
		valid + strings.Replace(valid, strings.Repeat("a", 64), strings.Repeat("b", 64), 1),
		strings.Replace(valid, "<none>", "broken", 1),
	} {
		source := &inventorySource{result: host.Result{Stdout: row}}
		if _, err := ReadPublicationInventory(context.Background(), source, DriverDocker); !errors.Is(err, core.ErrIncompatibleState) {
			t.Fatalf("accepted hostile/ambiguous inventory: %v", err)
		}
	}
}
func TestPublicationInventoryFailureIsNotEmpty(t *testing.T) {
	for _, source := range []*inventorySource{
		{result: host.Result{StdoutTruncated: true}}, {result: host.Result{ExitCode: 1, Stderr: "secret"}}, {err: errors.New("secret")},
	} {
		_, err := ReadPublicationInventory(context.Background(), source, DriverDocker)
		if err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatal("failure hidden or raw output leaked")
		}
	}
	empty := &inventorySource{}
	got, err := ReadPublicationInventory(context.Background(), empty, DriverDocker)
	if err != nil || len(got.Images) != 0 || got.Revision == "" {
		t.Fatal("empty successful inventory rejected")
	}
	if _, err := ReadPublicationInventory(context.Background(), empty, Driver("--evil")); !errors.Is(err, core.ErrInvalidArgument) || empty.calls != 1 {
		t.Fatal("invalid driver executed")
	}
}
