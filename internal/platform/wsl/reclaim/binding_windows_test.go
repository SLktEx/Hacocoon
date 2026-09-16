//go:build windows && (amd64 || arm64)

package wslreclaim

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func TestNativeInstallationBindingPreservesEnrollmentAndRejectsReplacement(t *testing.T) {
	id, err := windows.GenerateGUID()
	if err != nil {
		t.Fatal(err)
	}
	nonce, err := windows.GenerateGUID()
	if err != nil {
		t.Fatal(err)
	}
	path := `Software\Hacocoon\Tests\` + id.String()
	key, existed, err := registry.CreateKey(registry.CURRENT_USER, path, registry.ALL_ACCESS)
	if err != nil {
		t.Fatal(err)
	}
	if existed {
		key.Close()
		t.Fatal("test key exists")
	}
	defer func() {
		key.Close()
		if err := registry.DeleteKey(registry.CURRENT_USER, path); err != nil {
			t.Error(err)
		}
	}()
	store := &operationStore{key: key}
	target := installationObservation{Registration: registration{ID: id, Name: "Hacocoon-Test", BasePath: `C:\owned`, VHDFileName: "ext4.vhdx"}, Installation: installationIdentity{InstallationID: strings.Trim(strings.ToLower(nonce.String()), "{}"), RegistrationID: strings.ToLower(id.String()), SchemaVersion: 1}, Disk: diskIdentity{Volume: 1, Low: 2}, WindowsOwner: "S-1-5-18"}
	if err := store.requireBinding(target); err == nil {
		t.Fatal("missing enrollment accepted")
	}
	if _, _, err := key.GetBinaryValue("Installation"); err == nil {
		t.Fatal("lookup enrolled implicitly")
	}
	if err := store.enrollBinding(target); err != nil {
		t.Fatal(err)
	}
	before, _, err := key.GetBinaryValue("Installation")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.enrollBinding(target); err != nil {
		t.Fatal(err)
	}
	if err := store.requireBinding(target); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*installationObservation){
		func(v *installationObservation) { v.Disk.Low++ },
		func(v *installationObservation) { v.WindowsOwner = "S-1-5-19" },
		func(v *installationObservation) { v.Registration.BasePath = `C:\replacement` },
		func(v *installationObservation) {
			v.Installation.InstallationID = "00000000-0000-4000-8000-000000000001"
		},
		func(v *installationObservation) {
			v.Registration.ID = nonce
			v.Installation.RegistrationID = strings.ToLower(nonce.String())
		},
	} {
		changed := target
		mutate(&changed)
		if err := store.requireBinding(changed); err == nil {
			t.Fatal("replacement accepted")
		}
		if err := store.enrollBinding(changed); err == nil {
			t.Fatal("replacement re-enrolled")
		}
		after, _, err := key.GetBinaryValue("Installation")
		if err != nil || !bytes.Equal(before, after) {
			t.Fatal("binding changed", err)
		}
	}
	record, _ := json.Marshal(installationBinding{Version: 1, Target: target})
	for _, bad := range [][]byte{bytes.Replace(record, []byte(`"Version":1`), []byte(`"Version":2`), 1), bytes.Replace(record, []byte(`"Version":1`), []byte(`"Version":1,"Version":1`), 1), bytes.Replace(record, []byte(`"Version":1`), []byte(`"Version":1,"Future":true`), 1), make([]byte, 4097)} {
		if err := key.SetBinaryValue("Installation", bad); err != nil {
			t.Fatal(err)
		}
		if _, err := store.readBinding(); err == nil {
			t.Fatal("malformed binding readable")
		}
		if err := store.enrollBinding(target); err == nil {
			t.Fatal("malformed binding replaced")
		}
		after, _, err := key.GetBinaryValue("Installation")
		if err != nil || !bytes.Equal(bad, after) {
			t.Fatal("unknown data lost", err)
		}
	}
}

func TestDedicatedWSLInstallationEnrollment(t *testing.T) {
	if os.Getenv("HACO_E2E_RECLAIM_ENROLL_INSTALLATION") != "1" {
		t.Skip("requires explicit enrollment of exact dedicated WSL")
	}
	r, err := readRegistration(os.Getenv("HACO_E2E_RECLAIM_REGISTRATION"))
	if err != nil {
		t.Fatal(err)
	}
	path, err := r.diskPath()
	if err != nil || path != os.Getenv("HACO_E2E_RECLAIM_VHD") {
		t.Fatal("unexpected VHD", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	if err := r.enrollInstallation(ctx); err != nil {
		t.Fatal(err)
	}
	if err := r.enrollInstallation(ctx); err != nil {
		t.Fatal("repeat enrollment", err)
	}
	t.Log("PASS exact dedicated registration/installation/file/owner enrollment and unchanged repeat; no compaction")
}
