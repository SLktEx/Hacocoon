package incus

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestUnchangedDNSSetupDoesNotExhaustServiceStartLimit(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux guest shell contract")
	}
	root := t.TempDir()
	for _, dir := range []string{"bin", "etc/systemd/system", "usr/local/libexec", "usr/local/bin"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(root+"/bin/systemctl", []byte("#!/bin/sh\ncase \"$*\" in\n 'show --property=Version --value') exit 0 ;;\n 'daemon-reload'|'enable hacocoon-dns.service'|'is-active --quiet hacocoon-dns.service') exit 0 ;;\n 'restart hacocoon-dns.service')\n   count=0; if test -f \"$COUNT\"; then count=$(cat \"$COUNT\"); fi\n   count=$((count+1)); printf '%s' \"$count\" > \"$COUNT\"\n   test \"$count\" -le 5 ;;\n 'start hacocoon-dns.service')\n   printf 'start\\n' >> \"$STARTS\"\n   test \"$FAIL_START\" != 1 ;;\n 'show -p ExecMainStatus --value hacocoon-dns.service') printf '1\\n' ;;\n *) exit 99 ;;\nesac\n"), 0700); err != nil {
		t.Fatal(err)
	}
	script := strings.NewReplacer("/etc/", root+"/etc/", "/usr/local/", root+"/usr/local/").Replace(environmentDNSSetup)
	invoke := func(binary, fail string) error {
		t.Helper()
		if err := os.WriteFile(root+"/usr/local/libexec/hacocoon-dns.next", []byte(binary), 0700); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("/bin/sh", "-ec", script)
		cmd.Env = []string{"PATH=" + root + "/bin:/usr/bin:/bin", "COUNT=" + root + "/count", "STARTS=" + root + "/starts", "FAIL_START=" + fail}
		output, err := cmd.CombinedOutput()
		if fail == "1" && dnsSetupFailureStage(string(output)) != "start; service_exit=1" {
			t.Fatal("start failure category lost")
		}
		return err
	}
	for i := 0; i < 12; i++ {
		if err := invoke("companion", "0"); err != nil {
			t.Fatalf("unchanged setup %d failed: %v", i, err)
		}
	}
	count, _ := os.ReadFile(root + "/count")
	starts, _ := os.ReadFile(root + "/starts")
	if string(count) != "1" || strings.Count(string(starts), "start\n") != 11 {
		t.Fatal("unchanged DNS service restarted")
	}
	if err := invoke("updated companion", "0"); err != nil {
		t.Fatal(err)
	}
	count, _ = os.ReadFile(root + "/count")
	if string(count) != "2" {
		t.Fatal("changed companion not restarted")
	}
	unit := root + "/etc/systemd/system/hacocoon-dns.service"
	if err := os.WriteFile(unit, []byte("drift"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := invoke("updated companion", "0"); err != nil {
		t.Fatal(err)
	}
	count, _ = os.ReadFile(root + "/count")
	if string(count) != "3" {
		t.Fatal("changed unit not restarted")
	}
	if err := invoke("updated companion", "1"); err == nil {
		t.Fatal("failed unchanged service start hidden")
	}
}
