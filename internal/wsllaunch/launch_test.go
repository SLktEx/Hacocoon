package wsllaunch

import (
	"github.com/SLktEx/Hacocoon/internal/reclamation"
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

func TestRegisteredControlInvocation(t *testing.T) {
	target := reclamation.WSLTarget{RegistrationID: "{11111111-1111-1111-1111-111111111111}", InstallationID: "22222222-2222-2222-2222-222222222222"}
	got, err := RegisteredPlan(target, `C:\Windows`)
	if err != nil {
		t.Fatal(err)
	}
	expected, _ := Plan("display-name", `C:\Windows`, ControlStdio)
	expected.Args[0], expected.Args[1] = "--distribution-id", target.RegistrationID
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("%#v", got)
	}
	for _, id := range []string{"", "--user root", "{00000000-0000-0000-0000-000000000000}", "{11111111-1111-1111-1111-111111111111};exit"} {
		target.RegistrationID = id
		if _, err := RegisteredPlan(target, `C:\Windows`); err == nil {
			t.Fatal(id)
		}
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
