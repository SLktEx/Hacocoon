package wsllaunch

import (
	"reflect"
	"testing"
)

func TestFixedControlInvocation(t *testing.T) {
	got, err := Plan("hacocoon-second", `C:\Windows`, ControlStdio)
	if err != nil {
		t.Fatal(err)
	}
	expected := Invocation{File: `C:\Windows\System32\wsl.exe`, Args: []string{"--distribution", "hacocoon-second", "--exec", "/usr/bin/env", "-i", "PATH=/usr/local/bin:/usr/bin:/bin", "LANG=C.UTF-8", "/usr/local/bin/haco", "_control-stdio"}, Env: []string{`SystemRoot=C:\Windows`, `WINDIR=C:\Windows`}}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("unexpected plan: %#v", got)
	}
}

func TestRejectsCallerSelectedCommandsAndPaths(t *testing.T) {
	for _, distro := range []string{"", "-u root", "x y", "x\n--exec", "../x", "x;id", "$(id)", "x\\y"} {
		if _, err := Plan(distro, `C:\Windows`, ControlStdio); err == nil {
			t.Fatal(distro)
		}
	}
	for _, root := range []string{"", `Windows`, `\\server\Windows`, `C:\Windows\..\Temp`, `C:\Windows\tool:stream`, `C:\Windows\x"`, "C:\\Windows\n"} {
		if _, err := Plan("hacocoon", root, ControlStdio); err == nil {
			t.Fatal(root)
		}
	}
	if _, err := Plan("hacocoon", `C:\Windows`, Operation("sh")); err == nil {
		t.Fatal("arbitrary operation accepted")
	}
}
