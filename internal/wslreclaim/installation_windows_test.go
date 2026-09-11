//go:build windows && (amd64 || arm64)

package wslreclaim

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestInstallationObservationRejectsUnknownAndReusedIdentity(t *testing.T) {
	id, err := windows.GenerateGUID()
	if err != nil {
		t.Fatal(err)
	}
	nonce, err := windows.GenerateGUID()
	if err != nil {
		t.Fatal(err)
	}
	identity := installationIdentity{InstallationID: strings.Trim(strings.ToLower(nonce.String()), "{}"), RegistrationID: strings.ToLower(id.String()), SchemaVersion: 1}
	raw, _ := json.Marshal(identity)
	raw = append(raw, '\n')
	if got, err := decodeInstallation(raw, id); err != nil || got != identity {
		t.Fatal(got, err)
	}
	for _, data := range [][]byte{nil, raw[:len(raw)-1], append([]byte(" "), raw...), bytes.Replace(raw, []byte(`"schema_version":1`), []byte(`"schema_version":2`), 1), bytes.Replace(raw, []byte(`"schema_version":1`), []byte(`"schema_version":1,"future":true`), 1), bytes.Replace(raw, []byte(`"schema_version":1`), []byte(`"schema_version":1,"schema_version":1`), 1), bytes.Replace(raw, []byte(identity.InstallationID), []byte("00000000-0000-0000-0000-000000000000"), 1), make([]byte, 4097)} {
		if _, err := decodeInstallation(data, id); err == nil {
			t.Fatal("invalid observation accepted")
		}
	}
	other, err := windows.GenerateGUID()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeInstallation(raw, other); err == nil {
		t.Fatal("different registration accepted")
	}
	var output registrationOutput
	if n, err := output.Write(make([]byte, 4096)); err != nil || n != 4096 {
		t.Fatal(n, err)
	}
	if _, err := output.Write([]byte{1}); err == nil || !output.overflow || len(output.data) != 4096 {
		t.Fatal("output limit not enforced")
	}
	if _, err := output.Write(nil); err == nil {
		t.Fatal("overflow cleared")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (registration{}).observeInstallation(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestDedicatedWSLInstallationObservation(t *testing.T) {
	if os.Getenv("HACO_E2E_RECLAIM_OBSERVE_INSTALLATION") != "1" {
		t.Skip("requires exact dedicated WSL with installed read-only helper")
	}
	r, err := readRegistration(os.Getenv("HACO_E2E_RECLAIM_REGISTRATION"))
	if err != nil {
		t.Fatal(err)
	}
	path, err := r.diskPath()
	if err != nil || path != os.Getenv("HACO_E2E_RECLAIM_VHD") {
		t.Fatal("unexpected VHD", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	result, err := r.observeInstallation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result.Registration != r || result.WindowsOwner == "" || result.Disk == (diskIdentity{}) {
		t.Fatal("missing identity fields")
	}
	t.Logf("PASS observed installed Host with pinned file %+v and nonempty Windows owner; no enrollment or compaction", result.Disk)
}
