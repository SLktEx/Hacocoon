package sshkey_test

import (
	"crypto/elliptic"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"strings"
	"testing"

	sshkey "github.com/SLktEx/Hacocoon/internal/client/ssh/key"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func wire(fields ...[]byte) []byte {
	var result []byte
	for _, field := range fields {
		result = binary.BigEndian.AppendUint32(result, uint32(len(field)))
		result = append(result, field...)
	}
	return result
}

func fixtures() map[string][][]byte {
	result := map[string][][]byte{
		"ssh-ed25519":                {make([]byte, 32)},
		"ssh-rsa":                    {{1, 0, 1}, {0, 0x80, 1}},
		"sk-ssh-ed25519@openssh.com": {make([]byte, 32), []byte("ssh:test")},
	}
	for _, curve := range []elliptic.Curve{elliptic.P256(), elliptic.P384(), elliptic.P521()} {
		name := map[int]string{256: "nistp256", 384: "nistp384", 521: "nistp521"}[curve.Params().BitSize]
		point := elliptic.Marshal(curve, curve.Params().Gx, curve.Params().Gy)
		result["ecdsa-sha2-"+name] = [][]byte{[]byte(name), point}
		if name == "nistp256" {
			result["sk-ecdsa-sha2-nistp256@openssh.com"] = [][]byte{[]byte(name), point, []byte("ssh:test")}
		}
	}
	return result
}

func TestNormalizeSupportedWireKeysAndRemoveComments(t *testing.T) {
	for algorithm, fields := range fixtures() {
		t.Run(algorithm, func(t *testing.T) {
			blob := wire(append([][]byte{[]byte(algorithm)}, fields...)...)
			for _, encoding := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding} {
				want := algorithm + " " + encoding.EncodeToString(blob)
				got, err := sshkey.NormalizePublicKey("  " + want + " private-comment@example.invalid  ")
				if err != nil || got != want {
					t.Fatalf("normalized=%q, err=%v", got, err)
				}
			}
		})
	}
}

func TestNormalizeRejectsMalformedWireForEveryAlgorithm(t *testing.T) {
	for algorithm, fields := range fixtures() {
		t.Run(algorithm, func(t *testing.T) {
			blob := wire(append([][]byte{[]byte(algorithm)}, fields...)...)
			mutations := [][]byte{append(append([]byte{}, blob...), 0), wire([]byte("ssh-unknown"))}
			// Every incomplete wire prefix must fail, including truncated lengths.
			for length := 0; length < len(blob); length++ {
				mutations = append(mutations, blob[:length])
			}
			for i := range fields {
				copyFields := append([][]byte{[]byte(algorithm)}, fields...)
				copyFields[i+1] = nil
				mutations = append(mutations, wire(copyFields...))
				// Applications are opaque nonempty strings; no content restriction.
				if strings.HasPrefix(algorithm, "sk-") && i == len(fields)-1 {
					continue
				}
				copyFields[i+1] = []byte("wrong-curve-or-point")
				if algorithm == "ssh-rsa" {
					copyFields[i+1] = []byte{0x80}
				}
				mutations = append(mutations, wire(copyFields...))
			}
			for index, malformed := range mutations {
				raw := algorithm + " " + base64.StdEncoding.EncodeToString(malformed)
				if got, err := sshkey.NormalizePublicKey(raw); got != "" || !errors.Is(err, core.ErrInvalidArgument) {
					t.Fatalf("mutation %d accepted: %q, %v", index, got, err)
				}
			}
		})
	}
}

func TestNormalizeRejectsInvalidTextAndNoncanonicalRSAIntegers(t *testing.T) {
	for _, raw := range []string{"", "ssh-ed25519", "ssh-dss AAAA", "ssh-ed25519 !!!", "ssh-ed25519 AAAA\nHost *", strings.Repeat("a", 16*1024+1)} {
		if got, err := sshkey.NormalizePublicKey(raw); got != "" || !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatalf("accepted invalid text: %q, %v", got, err)
		}
	}
	for _, integer := range [][]byte{nil, {0}, {0, 0}, {0x80}, {0, 1}, {0, 0x7f}} {
		for _, fields := range [][][]byte{{integer, {1}}, {{3}, integer}} {
			raw := "ssh-rsa " + base64.StdEncoding.EncodeToString(wire(append([][]byte{[]byte("ssh-rsa")}, fields...)...))
			if got, err := sshkey.NormalizePublicKey(raw); got != "" || !errors.Is(err, core.ErrInvalidArgument) {
				t.Fatalf("accepted RSA integer %x: %q, %v", integer, got, err)
			}
		}
	}
}
