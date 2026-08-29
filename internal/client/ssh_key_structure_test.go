package client

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestNormalizePublicKeyRejectsEd25519WithInvalidKeyLength(t *testing.T) {
	invalid := encodedWireKey("ssh-ed25519", make([]byte, 31))
	if _, err := normalizePublicKey("ssh-ed25519 " + invalid); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("invalid Ed25519 key length accepted: %v", err)
	}
}

func TestNormalizePublicKeyRejectsTrailingWirePayload(t *testing.T) {
	blob := append(wireString([]byte("ssh-ed25519")), wireString(make([]byte, 32))...)
	blob = append(blob, 0)
	encoded := base64.StdEncoding.EncodeToString(blob)
	if _, err := normalizePublicKey("ssh-ed25519 " + encoded); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("trailing wire payload accepted: %v", err)
	}
}

func TestNormalizePublicKeyAcceptsValidEd25519AndDropsComment(t *testing.T) {
	got, err := normalizePublicKey(validEd25519Key + " laptop-key")
	if err != nil {
		t.Fatal(err)
	}
	if got != validEd25519Key {
		t.Fatalf("normalized key=%q want=%q", got, validEd25519Key)
	}
}

func encodedWireKey(keyType string, fields ...[]byte) string {
	blob := wireString([]byte(keyType))
	for _, field := range fields {
		blob = append(blob, wireString(field)...)
	}
	return base64.StdEncoding.EncodeToString(blob)
}

func wireString(value []byte) []byte {
	encoded := make([]byte, 4+len(value))
	binary.BigEndian.PutUint32(encoded[:4], uint32(len(value)))
	copy(encoded[4:], value)
	return encoded
}
