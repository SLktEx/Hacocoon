package incus

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/host"
)

type publishedBaseBuildFile struct {
	content string
	mode    string
}

func TestProvisionTrustedHostBaseBuildDefaultsPublishesOpenSSHAndNerdctlBuild(t *testing.T) {
	published := map[string]publishedBaseBuildFile{}
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if len(args) >= 4 && args[0] == "file" && args[1] == "push" {
			payload, err := os.ReadFile(args[2])
			if err != nil {
				return host.Result{}, err
			}
			mode := ""
			for i := 0; i+1 < len(args); i++ {
				if args[i] == "--mode" {
					mode = args[i+1]
					break
				}
			}
			published[strings.TrimPrefix(args[3], trustedHostName)] = publishedBaseBuildFile{content: string(payload), mode: mode}
		}
		return host.Result{}, nil
	}}

	if err := New(runner).provisionTrustedHostBaseBuildDefaults(context.Background()); err != nil {
		t.Fatal(err)
	}

	packer := published[trustedHostOpenSSHBaseBuildDir+"/base.pkr.hcl"]
	if packer.mode != "0644" || !strings.Contains(packer.content, `source "null" "base"`) || !strings.Contains(packer.content, `script = "setup.sh"`) {
		t.Fatalf("unexpected Packer template: mode=%q content=%q", packer.mode, packer.content)
	}
	setup := published[trustedHostOpenSSHBaseBuildDir+"/setup.sh"]
	if setup.mode != "0755" ||
		!strings.Contains(setup.content, "apt-get update") ||
		!strings.Contains(setup.content, "apt-get install -y --no-install-recommends ca-certificates curl iptables openssh-server") ||
		!strings.Contains(setup.content, `NERDCTL_VERSION="2.3.5"`) ||
		!strings.Contains(setup.content, "b697295c623639734aaab737523c808fd3cc8d3046039fd94fff1744e4c317aa") ||
		!strings.Contains(setup.content, "6e4b687f1d138e750a3c8372abc0f81d3d7490b6359c48c0562fc7dfe98859b2") ||
		!strings.Contains(setup.content, "nerdctl-full-${NERDCTL_VERSION}-linux-${NERDCTL_ARCH}.tar.gz") ||
		!strings.Contains(setup.content, "sha256sum -c -") ||
		!strings.Contains(setup.content, `tar -xzf "$archive" -C /usr/local`) ||
		!strings.Contains(setup.content, "/usr/local/bin/nerdctl --version") ||
		!strings.Contains(setup.content, "/usr/local/bin/containerd --version") ||
		!strings.Contains(setup.content, "/usr/local/bin/runc --version") ||
		!strings.Contains(setup.content, "/usr/local/bin/buildkitd --version") ||
		!strings.Contains(setup.content, "systemctl enable containerd.service buildkit.service") ||
		!strings.Contains(setup.content, "rm -rf /var/lib/apt/lists/*") {
		t.Fatalf("unexpected setup script: mode=%q content=%q", setup.mode, setup.content)
	}
	build := published[trustedHostOpenSSHBaseBuildDir+"/build.sh"]
	if build.mode != "0755" || !strings.Contains(build.content, "haco base build --name ubuntu-26.04-openssh --from haco/ubuntu-26.04") {
		t.Fatalf("unexpected build helper: mode=%q content=%q", build.mode, build.content)
	}
	readme := published[trustedHostOpenSSHBaseBuildDir+"/README.md"]
	if readme.mode != "0644" || !strings.Contains(readme.content, "~/base-builds/ubuntu-26.04-openssh/build.sh") {
		t.Fatalf("unexpected README: mode=%q content=%q", readme.mode, readme.content)
	}
	if len(published) != 4 {
		t.Fatalf("published files=%d, want 4: %#v", len(published), published)
	}
}
