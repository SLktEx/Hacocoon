package client

import (
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
	if err != nil || len(blob) < 8 {
		return "", fmt.Errorf("SSH public key payload: %w", core.ErrInvalidArgument)
	}

	algorithmLength := int(binary.BigEndian.Uint32(blob[:4]))
	if algorithmLength <= 0 || 4+algorithmLength+4 > len(blob) {
		return "", fmt.Errorf("SSH public key payload: %w", core.ErrInvalidArgument)
	}
	if string(blob[4:4+algorithmLength]) != fields[0] {
		return "", fmt.Errorf("SSH public key algorithm mismatch: %w", core.ErrInvalidArgument)
	}

	return fields[0] + " " + fields[1], nil
}

func supportedPublicKeyType(keyType string) bool {
	return strings.HasPrefix(keyType, "ssh-") ||
		strings.HasPrefix(keyType, "ecdsa-") ||
		strings.HasPrefix(keyType, "sk-ssh-") ||
		strings.HasPrefix(keyType, "sk-ecdsa-")
}
