package incus

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/basebuild"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestOfficialBaseResolvesBuilderRevisionWhileRebuildUsesUpstreamParent(t *testing.T) {
	buildName, ok := basebuild.OfficialBuildName(defaultBaseName)
	if !ok {
		t.Fatal("default Base is not registered as official")
	}
	buildInstance := "env-0123456789abcdef0123456789abcdef"
	aliasName := builtBasePrefix + string(buildName)
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "/1.0/images/aliases?"):
			return host.Result{Stdout: `[{"name":"` + aliasName + `","target":"` + testFingerprintA + `","description":"` + builtBaseDescription + `","type":"container"}]`}, nil
		case strings.Contains(joined, "/1.0/images/"+testFingerprintA+"?"):
			return host.Result{Stdout: `{"fingerprint":"` + testFingerprintA + `","public":false,"type":"container","properties":{"user.hacocoon.kind":"base-image","user.hacocoon.base-name":"` + string(buildName) + `","user.hacocoon.build-instance":"` + buildInstance + `"}}`}, nil
		case len(args) >= 2 && args[0] == "image" && args[1] == "info":
			if len(args) < 3 || args[2] != "images:ubuntu/26.04" {
				return host.Result{}, errors.New("unexpected image source")
			}
			return host.Result{Stdout: `{"fingerprint":"` + testFingerprintB + `"}`}, nil
		default:
			return host.Result{}, errors.New("unexpected call: " + joined)
		}
	}}
	provider, err := NewBaseProvider(New(runner))
	if err != nil {
		t.Fatal(err)
	}

	resolved, err := provider.resolveBase(context.Background(), defaultBaseName)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.ref.Name != defaultBaseName || resolved.ref.Revision != core.BaseRevision("sha256:"+testFingerprintA) || resolved.pinnedSource != "local:"+testFingerprintA || !resolved.built {
		t.Fatalf("official resolved = %#v", resolved)
	}

	parent, err := provider.resolveParentBase(context.Background(), defaultBaseName)
	if err != nil {
		t.Fatal(err)
	}
	if parent.ref.Name != defaultBaseName || parent.ref.Revision != core.BaseRevision("sha256:"+testFingerprintB) || parent.pinnedSource != "images:"+testFingerprintB || parent.built {
		t.Fatalf("parent resolved = %#v", parent)
	}

	bases, err := provider.ListBases(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, base := range bases {
		if base.Name == buildName {
			t.Fatalf("internal official build leaked into Base list: %#v", bases)
		}
	}
}
