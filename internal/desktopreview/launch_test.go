package desktopreview

import (
	"reflect"
	"strings"
	"testing"
)

func TestExactLocalReview(t *testing.T) {
	distro := "Hacocoon-Review"
	id := strings.Repeat("a", 32)
	uri, err := URI(distro, id)
	if err != nil {
		t.Fatal(err)
	}
	p, err := Plan(distro, uri, `C:\Windows`)
	if err != nil {
		t.Fatal(err)
	}
	if p.File != `C:\Windows\System32\wsl.exe` || !reflect.DeepEqual(p.Args, []string{"--distribution", distro, "--exec", "/usr/bin/env", "-i", "PATH=/usr/local/bin:/usr/bin:/bin", "LANG=C.UTF-8", "/usr/local/bin/haco", "approve", id}) {
		t.Fatalf("wrong fixed launch: %+v", p)
	}
	if !reflect.DeepEqual(p.Env, []string{`SystemRoot=C:\Windows`, `WINDIR=C:\Windows`}) {
		t.Fatal(p.Env)
	}
}
func TestUntrustedActivationCannotSupplyAuthorityOrCommands(t *testing.T) {
	valid, _ := URI("Hacocoon", strings.Repeat("a", 32))
	other, _ := URI("Other", strings.Repeat("a", 32))
	for _, uri := range []string{"", other, valid + "?answer=yes", valid + "#x", valid + "/", valid + "\n", strings.Replace(valid, "request/", "user@request/", 1), strings.Replace(valid, "request/", "request/%2f", 1), valid + `" --exec sh`, strings.ToUpper(valid)} {
		if _, err := Plan("Hacocoon", uri, `C:\Windows`); err == nil {
			t.Errorf("accepted hostile URI %q", uri)
		}
	}
	for _, distro := range []string{"-root", "a b", "a/b", "a\n", strings.Repeat("a", 65)} {
		if _, err := Scheme(distro); err == nil {
			t.Errorf("accepted %q", distro)
		}
	}
	for _, root := range []string{"Windows", `\\server\share`, "C:\\Windows\n", `C:\Windows" -x`} {
		if _, err := Plan("Hacocoon", valid, root); err == nil {
			t.Errorf("accepted root %q", root)
		}
	}
}
func TestDistributionRegistrationsDoNotReplaceEachOther(t *testing.T) {
	a, _ := Scheme("Hacocoon")
	same, _ := Scheme("hacocoon")
	b, _ := Scheme("Hacocoon-Test")
	if a != same || a == b {
		t.Fatal(a, same, b)
	}
}
