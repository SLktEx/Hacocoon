package basebuild

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"strings"
	"testing"
)

func validTemplate() *PackerTemplate {
	return &PackerTemplate{Files: []SourceFile{{Path: "base.pkr.hcl", Data: []byte("source")}, {Path: "setup.sh", Data: []byte("true")}}}
}

func TestPackerContextRejectsPathsCollisionsAndBounds(t *testing.T) {
	for _, name := range []string{"/etc/shadow", "../secret", "a/../secret", "a//b", "-option", "C:/secret", "a\\b", ".env", "a/./b"} {
		p := validTemplate()
		p.Files = append(p.Files, SourceFile{Path: name})
		if p.Validate() == nil {
			t.Fatal("unsafe source accepted", name)
		}
	}
	for _, paths := range [][]string{{"setup.sh"}, {"dir", "dir/file"}, {"dir/file", "dir"}} {
		p := validTemplate()
		for _, name := range paths {
			p.Files = append(p.Files, SourceFile{Path: name})
		}
		if p.Validate() == nil {
			t.Fatal("collision accepted", paths)
		}
	}
	p := validTemplate()
	p.Files[0].Data = make([]byte, MaxContextBytes+1)
	if p.Validate() == nil {
		t.Fatal("unbounded source")
	}
	if (Definition{Name: "tools", Run: "true", Packer: validTemplate()}).Validate() == nil {
		t.Fatal("two execution engines accepted")
	}
}

type failingPacker struct{}

func (failingPacker) Provision(ctx context.Context, _ PackerTemplate, execute Execute) error {
	if _, err := execute(ctx, core.ExecutionRequest{Argv: []string{"/bin/sh", "-eu", "-s"}, Stdin: []byte("true")}); err != nil {
		return err
	}
	return &ProvisionFailure{Stage: "validate", Execution: core.ExecutionResult{ExitCode: 2, Stderr: "private-build-token"}}
}

func TestPackerFailureRetainsPrivateResultAndCanonicalCleanup(t *testing.T) {
	f := &fakeEnv{t: t, script: "true"}
	got, err := (&Service{Environments: f, Packer: failingPacker{}}).Build(context.Background(), Definition{Name: "tools", Packer: validTemplate()})
	if !errors.Is(err, core.ErrRuntimeUnavailable) || strings.Contains(err.Error(), "private") || got.Stage != "validate" || got.Execution == nil || got.Execution.Stderr != "private-build-token" || got.Builder != "" {
		t.Fatal(got, err)
	}
	if strings.Join(f.calls, ",") != "create,script,delete" {
		t.Fatal("failure published or bypassed cleanup", f.calls)
	}
	f.calls = nil
	if _, err := (&Service{Environments: f}).Build(context.Background(), Definition{Name: "tools", Packer: validTemplate()}); !errors.Is(err, core.ErrUnsupported) || len(f.calls) != 0 {
		t.Fatal("missing plugin created Env", err, f.calls)
	}
}
