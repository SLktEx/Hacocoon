package client

import (
	"crypto/elliptic"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func normalizePublicKey(raw string) (string, error) {
	key := strings.TrimSpace(raw)
	if key == "" || strings.ContainsAny(key, "\r\n") {
		return "", fmt.Errorf("SSH public key: %w", core.ErrInvalidArgument)
	}
	fields := strings.Fields(key)
	if len(fields) < 2 || !supportedPublicKeyType(fields[0]) {
		return "", fmt.Errorf("SSH public key: %w", core.ErrInvalidArgument)
	}

	blob, err := base64.StdEncoding.DecodeString(fields[1])
	if err != nil {
		blob, err = base64.RawStdEncoding.DecodeString(fields[1])
	}
	if err != nil {
		return "", fmt.Errorf("SSH public key payload: %w", core.ErrInvalidArgument)
	}
	if err := validatePublicKeyBlob(fields[0], blob); err != nil {
		return "", fmt.Errorf("SSH public key payload: %w", core.ErrInvalidArgument)
	}
	return fields[0] + " " + fields[1], nil
}

func supportedPublicKeyType(keyType string) bool {
	switch keyType {
	case "ssh-ed25519", "ssh-rsa",
		"ecdsa-sha2-nistp256", "ecdsa-sha2-nistp384", "ecdsa-sha2-nistp521",
		"sk-ssh-ed25519@openssh.com", "sk-ecdsa-sha2-nistp256@openssh.com":
		return true
	default:
		return false
	}
}

func validatePublicKeyBlob(keyType string, blob []byte) error {
	reader := sshWireReader{data: blob}
	algorithm, ok := reader.stringField()
	if !ok || string(algorithm) != keyType {
		return fmt.Errorf("algorithm mismatch")
	}

	switch keyType {
	case "ssh-ed25519":
		key, ok := reader.stringField()
		if !ok || len(key) != 32 {
			return fmt.Errorf("invalid Ed25519 key length")
		}
	case "ssh-rsa":
		exponent, ok := reader.stringField()
		if !ok || !validPositiveMPInt(exponent) {
			return fmt.Errorf("invalid RSA exponent")
		}
		modulus, ok := reader.stringField()
		if !ok || !validPositiveMPInt(modulus) {
			return fmt.Errorf("invalid RSA modulus")
		}
	case "ecdsa-sha2-nistp256":
		if !validateECDSAFields(&reader, "nistp256", elliptic.P256()) {
			return fmt.Errorf("invalid nistp256 key")
		}
	case "ecdsa-sha2-nistp384":
		if !validateECDSAFields(&reader, "nistp384", elliptic.P384()) {
			return fmt.Errorf("invalid nistp384 key")
		}
	case "ecdsa-sha2-nistp521":
		if !validateECDSAFields(&reader, "nistp521", elliptic.P521()) {
			return fmt.Errorf("invalid nistp521 key")
		}
	case "sk-ssh-ed25519@openssh.com":
		key, ok := reader.stringField()
		if !ok || len(key) != 32 {
			return fmt.Errorf("invalid security-key Ed25519 key length")
		}
		application, ok := reader.stringField()
		if !ok || len(application) == 0 {
			return fmt.Errorf("missing security-key application")
		}
	case "sk-ecdsa-sha2-nistp256@openssh.com":
		if !validateECDSAFields(&reader, "nistp256", elliptic.P256()) {
			return fmt.Errorf("invalid security-key ECDSA key")
		}
		application, ok := reader.stringField()
		if !ok || len(application) == 0 {
			return fmt.Errorf("missing security-key application")
		}
	default:
		return fmt.Errorf("unsupported key type")
	}
	if !reader.empty() {
		return fmt.Errorf("trailing key payload")
	}
	return nil
}

type sshWireReader struct {
	data []byte
}

func (r *sshWireReader) stringField() ([]byte, bool) {
	if len(r.data) < 4 {
		return nil, false
	}
	length := uint64(binary.BigEndian.Uint32(r.data[:4]))
	if length > uint64(len(r.data)-4) {
		return nil, false
	}
	end := 4 + int(length)
	value := r.data[4:end]
	r.data = r.data[end:]
	return value, true
}

func (r *sshWireReader) empty() bool { return len(r.data) == 0 }

func validPositiveMPInt(value []byte) bool {
	if len(value) == 0 || value[0]&0x80 != 0 {
		return false
	}
	if len(value) > 1 && value[0] == 0 && value[1]&0x80 == 0 {
		return false
	}
	for _, b := range value {
		if b != 0 {
			return true
		}
	}
	return false
}

func validateECDSAFields(reader *sshWireReader, expectedCurve string, curve elliptic.Curve) bool {
	curveName, ok := reader.stringField()
	if !ok || string(curveName) != expectedCurve {
		return false
	}
	point, ok := reader.stringField()
	if !ok {
		return false
	}
	x, y := elliptic.Unmarshal(curve, point)
	return x != nil && y != nil
}
