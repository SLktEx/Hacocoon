//go:build windows

package wslreclaim

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func TestRegistrationRefusesAmbiguousIdentityAndPaths(t *testing.T) {
	for _, id := range []string{"", "default", "Hacocoon", "--shutdown", `{00000000-0000-0000-0000-000000000000}`, `..\other`} {
		if _, err := registrationKey(id); err == nil {
			t.Fatalf("accepted registration %q", id)
		}
	}
	id, err := windows.GenerateGUID()
	if err != nil {
		t.Fatal(err)
	}
	valid := registration{ID: id, Name: "Hacocoon-Test", BasePath: `C:\owned`, VHDFileName: "ext4.vhdx"}
	path, err := valid.diskPath()
	if err != nil || path != `C:\owned\ext4.vhdx` {
		t.Fatal(path, err)
	}
	for _, mutate := range []func(*registration){
		func(r *registration) { r.ID = windows.GUID{} },
		func(r *registration) { r.Name = "--shutdown" },
		func(r *registration) { r.Name = "x\ncommand" },
		func(r *registration) { r.BasePath = "" },
		func(r *registration) { r.BasePath = `C:\owned\..\other` },
		func(r *registration) { r.BasePath = `\\server\share` },
		func(r *registration) { r.VHDFileName = `..\other.vhdx` },
		func(r *registration) { r.VHDFileName = `C:\other.vhdx` },
		func(r *registration) { r.VHDFileName = "ext4.vhdx:stream" },
		func(r *registration) { r.VHDFileName = "ext4.vhdx " },
	} {
		r := valid
		mutate(&r)
		if _, err := r.diskPath(); err == nil {
			t.Fatalf("accepted %+v", r)
		}
	}
}

func TestRegistrationReadsLiteralNativeValues(t *testing.T) {
	id, err := windows.GenerateGUID()
	if err != nil {
		t.Fatal(err)
	}
	// Never create or modify a real WSL registration, including for negative tests.
	testKey := `Software\Hacocoon\Tests\` + id.String()
	key, existed, err := registry.CreateKey(registry.CURRENT_USER, testKey, registry.ALL_ACCESS)
	if err != nil {
		t.Fatal(err)
	}
	if existed {
		key.Close()
		t.Fatal("test key unexpectedly exists")
	}
	defer func() {
		key.Close()
		if err := registry.DeleteKey(registry.CURRENT_USER, testKey); err != nil {
			t.Error(err)
		}
	}()
	base := t.TempDir()
	for name, value := range map[string]string{"DistributionName": "Hacocoon-Test", "BasePath": base, "VhdFileName": "ext4.vhdx"} {
		if err := key.SetStringValue(name, value); err != nil {
			t.Fatal(err)
		}
	}
	if err := key.SetDWordValue("Version", 2); err != nil {
		t.Fatal(err)
	}
	r, err := readRegistrationValues(key, id.String())
	if err != nil {
		t.Fatal(err)
	}
	if path, err := r.diskPath(); err != nil || path != filepath.Join(base, "ext4.vhdx") {
		t.Fatal(path, err)
	}
	if err := key.SetExpandStringValue("BasePath", base); err != nil {
		t.Fatal(err)
	}
	if _, err := readRegistrationValues(key, id.String()); err == nil {
		t.Fatal("accepted expandable path")
	}
	if err := key.SetStringValue("BasePath", base); err != nil {
		t.Fatal(err)
	}
	if err := key.SetDWordValue("Version", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := readRegistrationValues(key, id.String()); err == nil {
		t.Fatal("accepted WSL1")
	}
	if err := key.SetDWordValue("Version", 2); err != nil {
		t.Fatal(err)
	}
	if err := key.DeleteValue("VhdFileName"); err != nil {
		t.Fatal(err)
	}
	if _, err := readRegistrationValues(key, id.String()); err == nil {
		t.Fatal("guessed missing VHD filename")
	}
}

func TestDedicatedWSLRegistration(t *testing.T) {
	id := os.Getenv("HACO_E2E_RECLAIM_REGISTRATION")
	if id == "" {
		t.Skip("requires exact dedicated registration ID and expected VHD path")
	}
	r, err := readRegistration(id)
	if err != nil {
		t.Fatal(err)
	}
	path, err := r.diskPath()
	if err != nil || path != os.Getenv("HACO_E2E_RECLAIM_VHD") {
		t.Fatal("registered VHD path mismatch", err)
	}
	pin, err := pinDisk(path)
	if err != nil {
		t.Fatal(err)
	}
	defer pin.Close()
	if err := r.revalidate(); err != nil {
		t.Fatal(err)
	}
	if _, err := pin.Allocation(); err != nil {
		t.Fatal(err)
	}
	t.Log("exact registration and pinned allocation revalidated; no mutation")
}
