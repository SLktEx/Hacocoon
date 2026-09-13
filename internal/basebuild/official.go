package basebuild

import (
	"context"
	"fmt"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const officialBuildNamePrefix = "haco-official-"

// OfficialBuildRevision versions the trusted recipe set independently of user
// Base definitions. Setup persists only this marker after all official Bases have
// been published successfully; changing the recipe must bump this value.
const OfficialBuildRevision = "ssh-v1"

var officialBaseDefinitions = map[core.BaseName]Definition{
	"haco/ubuntu-26.04": {
		Name: "haco-official-ubuntu-26.04",
		From: "haco/ubuntu-26.04",
		Run:  officialUbuntuSSHScript,
	},
	"haco/ubuntu-24.04": {
		Name: "haco-official-ubuntu-24.04",
		From: "haco/ubuntu-24.04",
		Run:  officialUbuntuSSHScript,
	},
}

var officialBaseOrder = []core.BaseName{
	"haco/ubuntu-26.04",
	"haco/ubuntu-24.04",
}

var officialUbuntuPackageHosts = []string{
	"archive.ubuntu.com",
	"security.ubuntu.com",
	"ports.ubuntu.com",
}

const officialUbuntuSSHScript = `export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y --no-install-recommends openssh-server
command -v sshd >/dev/null 2>&1
systemctl disable --now ssh.service >/dev/null 2>&1 || true
systemctl disable --now sshd.service >/dev/null 2>&1 || true
apt-get clean
rm -rf /var/lib/apt/lists/*
`

type OfficialNetwork interface {
	AcquireOfficialBuild(context.Context, string, []string) (func(), error)
}

func OfficialBases() []core.BaseName {
	return append([]core.BaseName(nil), officialBaseOrder...)
}

func OfficialDefinition(base core.BaseName) (Definition, []string, bool) {
	definition, ok := officialBaseDefinitions[base]
	if !ok {
		return Definition{}, nil, false
	}
	hosts := append([]string(nil), officialUbuntuPackageHosts...)
	return definition, hosts, true
}

func OfficialBuildName(base core.BaseName) (core.BaseName, bool) {
	definition, _, ok := OfficialDefinition(base)
	if !ok {
		return "", false
	}
	return definition.Name, true
}

func OfficialBaseForBuildName(name core.BaseName) (core.BaseName, bool) {
	for base, definition := range officialBaseDefinitions {
		if definition.Name == name {
			return base, true
		}
	}
	return "", false
}

func IsOfficialBuildName(name core.BaseName) bool {
	_, ok := OfficialBaseForBuildName(name)
	return ok
}

func reservedOfficialBuildName(name core.BaseName) bool {
	return strings.HasPrefix(string(name), officialBuildNamePrefix)
}

func (s *Service) BuildOfficial(ctx context.Context, base core.BaseName) (Result, error) {
	definition, hosts, ok := OfficialDefinition(base)
	if !ok {
		return Result{}, fmt.Errorf("official Base %q: %w", base, core.ErrInvalidArgument)
	}
	if s == nil || s.Environments == nil || s.OfficialNetwork == nil {
		return Result{}, core.ErrUnsupported
	}
	return s.build(ctx, definition, buildOptions{
		logicalName:    base,
		parentBaseOnly: true,
		networkHosts:   hosts,
	})
}
