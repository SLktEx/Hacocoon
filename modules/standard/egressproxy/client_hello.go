package egressproxy

import (
	"io"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const maxClientHelloBytes = 128 << 10

func readClientHelloServerName(reader io.Reader) ([]byte, string, error) {
	var raw []byte
	var handshake []byte
	for len(raw) < maxClientHelloBytes {
		header := make([]byte, 5)
		if _, err := io.ReadFull(reader, header); err != nil {
			return nil, "", err
		}
		if header[0] != 22 {
			return nil, "", core.ErrPolicyDenied
		}
		length := int(header[3])<<8 | int(header[4])
		if length < 1 || len(raw)+5+length > maxClientHelloBytes {
			return nil, "", core.ErrPolicyDenied
		}
		payload := make([]byte, length)
		if _, err := io.ReadFull(reader, payload); err != nil {
			return nil, "", err
		}
		raw = append(raw, header...)
		raw = append(raw, payload...)
		handshake = append(handshake, payload...)
		if len(handshake) < 4 {
			continue
		}
		if handshake[0] != 1 {
			return nil, "", core.ErrPolicyDenied
		}
		messageLen := int(handshake[1])<<16 | int(handshake[2])<<8 | int(handshake[3])
		if messageLen < 1 || messageLen+4 > maxClientHelloBytes {
			return nil, "", core.ErrPolicyDenied
		}
		if len(handshake) < messageLen+4 {
			continue
		}
		name, err := parseClientHelloServerName(handshake[4 : messageLen+4])
		return raw, name, err
	}
	return nil, "", core.ErrPolicyDenied
}

func parseClientHelloServerName(body []byte) (string, error) {
	// legacy_version(2), random(32), session id, cipher suites,
	// compression methods, then extensions.
	if len(body) < 35 {
		return "", core.ErrPolicyDenied
	}
	pos := 34
	sessionLen := int(body[pos])
	pos++
	if pos+sessionLen+2 > len(body) {
		return "", core.ErrPolicyDenied
	}
	pos += sessionLen
	cipherLen := int(body[pos])<<8 | int(body[pos+1])
	pos += 2
	if cipherLen < 2 || pos+cipherLen+1 > len(body) {
		return "", core.ErrPolicyDenied
	}
	pos += cipherLen
	compressionLen := int(body[pos])
	pos++
	if pos+compressionLen == len(body) {
		return "", core.ErrPolicyDenied
	}
	if pos+compressionLen+2 > len(body) {
		return "", core.ErrPolicyDenied
	}
	pos += compressionLen
	extensionsLen := int(body[pos])<<8 | int(body[pos+1])
	pos += 2
	if extensionsLen < 0 || pos+extensionsLen != len(body) {
		return "", core.ErrPolicyDenied
	}
	end := pos + extensionsLen
	for pos+4 <= end {
		typeID := int(body[pos])<<8 | int(body[pos+1])
		length := int(body[pos+2])<<8 | int(body[pos+3])
		pos += 4
		if pos+length > end {
			return "", core.ErrPolicyDenied
		}
		if typeID == 0 {
			return parseServerNameExtension(body[pos : pos+length])
		}
		pos += length
	}
	return "", core.ErrPolicyDenied
}

func parseServerNameExtension(value []byte) (string, error) {
	if len(value) < 2 {
		return "", core.ErrPolicyDenied
	}
	listLen := int(value[0])<<8 | int(value[1])
	if listLen+2 != len(value) {
		return "", core.ErrPolicyDenied
	}
	pos := 2
	for pos+3 <= len(value) {
		nameType := value[pos]
		nameLen := int(value[pos+1])<<8 | int(value[pos+2])
		pos += 3
		if nameLen < 1 || pos+nameLen > len(value) {
			return "", core.ErrPolicyDenied
		}
		if nameType == 0 {
			return string(value[pos : pos+nameLen]), nil
		}
		pos += nameLen
	}
	return "", core.ErrPolicyDenied
}
