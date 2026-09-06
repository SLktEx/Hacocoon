package oci

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"io"
	"strings"
	"testing"
)

type transferFake struct {
	fail    bool
	loads   int
	saves   int
	content string
}

func (f *transferFake) SaveImage(_ context.Context, _ Driver, _ string, w io.Writer) error {
	f.saves++
	if _, err := io.WriteString(w, "image archive"); err != nil {
		return err
	}
	if f.fail {
		return core.ErrRuntimeUnavailable
	}
	return nil
}
func (f *transferFake) LoadImage(_ context.Context, _ core.Environment, _ Driver, r io.Reader) error {
	f.loads++
	b, e := io.ReadAll(r)
	f.content = string(b)
	return e
}

type transferState struct{}

func (transferState) GetEnvironment(context.Context, string) (core.Environment, error) {
	return core.Environment{Name: "dev", RuntimeRef: "owned"}, nil
}
func TestTransferNeverImportsPartialSaveAndCopiesBothDrivers(t *testing.T) {
	for _, driver := range []Driver{DriverDocker, DriverNerdctl} {
		for _, fail := range []bool{false, true} {
			f := &transferFake{fail: fail}
			s := TransferService{Backend: f, Environments: transferState{}}
			report, err := s.Distribute(context.Background(), TransferRequest{Environment: "dev", Driver: driver, Image: "example:test"})
			if fail {
				if !errors.Is(err, core.ErrRuntimeUnavailable) || f.loads != 0 {
					t.Fatalf("partial image imported: %v %+v", err, f)
				}
			} else if err != nil || f.content != "image archive" || report.Bytes != 13 || len(report.ArchiveSHA256) != 64 {
				t.Fatalf("report=%+v err=%v", report, err)
			}
		}
	}
}
func TestTransferRejectsOptionsPathsAndOversize(t *testing.T) {
	f := &transferFake{}
	s := TransferService{Backend: f, Environments: transferState{}}
	for _, image := range []string{"--output=/etc/passwd", "/tmp/image", "x\n--flag", "x;whoami", ""} {
		if _, err := s.Distribute(context.Background(), TransferRequest{Driver: DriverDocker, Image: image}); err == nil {
			t.Fatal(image)
		}
	}
	if f.saves != 0 {
		t.Fatal("invalid request reached Host")
	}
	w := &transferWriter{Writer: io.Discard, remaining: 2}
	if _, err := w.Write([]byte(strings.Repeat("x", 3))); err == nil {
		t.Fatal("unbounded archive")
	}
}
