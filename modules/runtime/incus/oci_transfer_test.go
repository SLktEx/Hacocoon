package incus

import (
	"github.com/SLktEx/Hacocoon/modules/plugin/oci"
	"strings"
	"testing"
)

func TestOCICommandsUseOnlyInstanceLocalSocketsAndNoShell(t *testing.T) {
	for _, driver := range []oci.Driver{oci.DriverDocker, oci.DriverNerdctl} {
		for _, op := range []string{"save", "load"} {
			args, err := localOCIArgs(driver, op, "sample:test")
			if err != nil {
				t.Fatal(err)
			}
			text := strings.Join(args, " ")
			if !strings.HasPrefix(text, "/usr/bin/env -i ") || strings.Contains(text, "sh -c") || strings.Contains(text, "tcp:") {
				t.Fatal(args)
			}
			if op == "save" && !strings.HasSuffix(text, "save -- sample:test") {
				t.Fatal(args)
			}
			if op == "load" && !strings.HasSuffix(text, "load") {
				t.Fatal(args)
			}
		}
	}
	if _, err := localOCIArgs("docker;evil", "save", "image"); err == nil {
		t.Fatal("invalid driver")
	}
}
