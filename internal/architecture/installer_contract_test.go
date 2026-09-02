package architecture

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestUbuntuInstallerKeepsIncusUserNamespaceCompatibilityContract(t *testing.T) {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve installer contract test source path")
	}

	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", ".."))
	installerPath := filepath.Join(repoRoot, "scripts", "install.sh")
	content, err := os.ReadFile(installerPath)
	if err != nil {
		t.Fatalf("read Ubuntu installer: %v", err)
	}
	installer := string(content)

	// Ubuntu 26.04 enables an AppArmor extension that blocks the user namespace
	// systemd 259 uses for service sandboxing inside otherwise-confined Incus
	// containers. If this compatibility reconciliation disappears, networkd can
	// remain stuck before DHCP and a created Environment is not actually Ready.
	for _, required := range []string{
		"ensure_incus_userns_compatibility",
		"/etc/sysctl.d/90-hacocoon-incus-userns.conf",
		"kernel.apparmor_restrict_unprivileged_unconfined = 0",
		"sysctl -q -w kernel.apparmor_restrict_unprivileged_unconfined=0",
		"Ubuntu AppArmor user-namespace restriction remained enabled after configuration",
	} {
		if !strings.Contains(installer, required) {
			t.Errorf("Ubuntu installer lost Incus userns compatibility contract %q", required)
		}
	}

	// Do not replace the narrow host compatibility setting with an unconfined
	// Incus instance or by disabling AppArmor integration wholesale.
	for _, forbidden := range []string{
		"lxc.apparmor.profile=unconfined",
		"INCUS_SECURITY_APPARMOR=false",
		"INCUS_SECURITY_APPARMOR=0",
	} {
		if strings.Contains(installer, forbidden) {
			t.Errorf("Ubuntu installer weakens Incus AppArmor confinement with %q", forbidden)
		}
	}
}
