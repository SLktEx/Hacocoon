package reclaimnative

import (
	"bytes"
	"strings"
	"testing"
)

func TestPreparedReceiptRejectsAmbiguityAndReplayFields(t *testing.T) {
	for _, raw := range []string{"", `null`, `[]`, prepared + prepared, prepared + "x", strings.Replace(prepared, "pending", "complete", 1), strings.Replace(prepared, operationID, "{00000000-0000-0000-0000-000000000000}", 1), strings.Replace(prepared, operationID, operationID[1:37], 1), `{"operation":"` + operationID + `","operation":"` + registrationID + `","state":"pending"}`, `{"operation":"` + operationID + `","\u006fperation":"` + registrationID + `","state":"pending"}`, `{"Operation":"` + operationID + `","state":"pending"}`, `{"operation":"` + operationID + `","state":"pending","registration":"` + registrationID + `"}`, `{"operation":"` + operationID + `","state":null}`, `{"operation":"` + operationID + `"}`, strings.Repeat(" ", receiptLimit) + prepared} {
		if _, err := decodePrepared([]byte(raw)); err == nil {
			t.Errorf("accepted %q", raw)
		}
	}
	for _, raw := range []string{prepared, "\n " + prepared + "\n", strings.Replace(prepared, operationID, strings.ToUpper(operationID), 1)} {
		if _, err := decodePrepared([]byte(raw)); err != nil {
			t.Errorf("refused %q: %v", raw, err)
		}
	}
}
func TestLaunchReceiptExactBoundedFrame(t *testing.T) {
	if pid, err := decodeLaunch([]byte(launched)); err != nil || pid != 42 {
		t.Fatal(pid, err)
	}
	for _, raw := range []string{"", launched + launched, launched + " ", strings.Replace(launched, "42", "0", 1), strings.Replace(launched, "42", "042", 1), strings.Replace(launched, "42", "-1", 1), strings.Replace(launched, "42", "4294967296", 1), strings.Replace(launched, "42", "4 2", 1), strings.TrimSuffix(launched, "\n"), strings.Repeat("a", receiptLimit+1)} {
		if _, err := decodeLaunch([]byte(raw)); err == nil {
			t.Errorf("accepted %q", raw)
		}
	}
}
func TestInstallerOwnershipMatchesExactRegistrationSchema(t *testing.T) {
	raw := `{"registration_id":"` + registrationID + `","schema_version":1}`
	if err := validateOwner([]byte(raw), registrationID); err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]byte{[]byte(strings.Replace(raw, registrationID, operationID, 1)), []byte(strings.Replace(raw, ":1", ":2", 1)), []byte(strings.Replace(raw, ":1", `:"1"`, 1)), []byte(`{"registration_id":"` + registrationID + `","schema_version":1,"schema_version":1}`), []byte(raw + "{}"), bytes.Repeat([]byte("x"), receiptLimit+1)} {
		if err := validateOwner(bad, registrationID); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}
