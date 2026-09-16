package incus

import (
	"context"
	_ "embed"
	"fmt"
	"os"

	"github.com/SLktEx/Hacocoon/internal/host/setup"
)

const trustedHostOpenSSHBaseBuildDir = "/root/base-builds/ubuntu-26.04-openssh"

//go:embed defaults/ubuntu-26.04-openssh/base.pkr.hcl
var trustedHostOpenSSHPacker []byte

//go:embed defaults/ubuntu-26.04-openssh/setup.sh
var trustedHostOpenSSHSetup []byte

//go:embed defaults/ubuntu-26.04-openssh/build.sh
var trustedHostOpenSSHBuild []byte

//go:embed defaults/ubuntu-26.04-openssh/README.md
var trustedHostOpenSSHReadme []byte

type trustedHostBaseBuildFile struct {
	name string
	mode string
	data []byte
}

func (r *Runtime) provisionTrustedHostBaseBuildDefaults(ctx context.Context) (resultErr error) {
	defer hostsetup.Track(ctx, "base_build_defaults")(&resultErr)

	files := []trustedHostBaseBuildFile{
		{name: "base.pkr.hcl", mode: "0644", data: trustedHostOpenSSHPacker},
		{name: "setup.sh", mode: "0755", data: trustedHostOpenSSHSetup},
		{name: "build.sh", mode: "0755", data: trustedHostOpenSSHBuild},
		{name: "README.md", mode: "0644", data: trustedHostOpenSSHReadme},
	}
	for _, file := range files {
		if err := r.pushTrustedHostBaseBuildFile(ctx, file); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runtime) pushTrustedHostBaseBuildFile(ctx context.Context, file trustedHostBaseBuildFile) error {
	tmp, err := os.CreateTemp("", "haco-base-build-*")
	if err != nil {
		return fmt.Errorf("create temporary base build file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(file.data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temporary base build file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary base build file: %w", err)
	}

	target := trustedHostOpenSSHBaseBuildDir + "/" + file.name
	if _, err := r.runner.Run(ctx, "incus", "file", "push", tmpName, trustedHostName+target,
		"--project", r.project,
		"--create-dirs",
		"--uid", "0",
		"--gid", "0",
		"--mode", file.mode,
	); err != nil {
		return fmt.Errorf("install trusted host base build file %s: %w", file.name, err)
	}
	return nil
}
