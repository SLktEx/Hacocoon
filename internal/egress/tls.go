package egress

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const maxClientHelloBytes = 64 << 10
const echExtension uint16 = 0xfe0d

func readClientHello(r *bufio.Reader) ([]byte, string, bool, error) {
	var raw, handshake []byte
	need := -1
	for len(raw) < maxClientHelloBytes {
		header := make([]byte, 5)
		if _, err := io.ReadFull(r, header); err != nil {
			return nil, "", false, err
		}
		if header[0] != 22 {
			return nil, "", false, core.ErrInvalidArgument
		}
		recordLen := int(binary.BigEndian.Uint16(header[3:5]))
		if recordLen <= 0 || len(raw)+5+recordLen > maxClientHelloBytes {
			return nil, "", false, core.ErrInvalidArgument
		}
		body := make([]byte, recordLen)
		if _, err := io.ReadFull(r, body); err != nil {
			return nil, "", false, err
		}
		raw = append(raw, header...)
		raw = append(raw, body...)
		handshake = append(handshake, body...)
		if len(handshake) >= 4 && need < 0 {
			if handshake[0] != 1 {
				return nil, "", false, core.ErrInvalidArgument
			}
			need = 4 + int(handshake[1])<<16 + int(handshake[2])<<8 + int(handshake[3])
			if need > maxClientHelloBytes {
				return nil, "", false, core.ErrInvalidArgument
			}
		}
		if need >= 0 && len(handshake) >= need {
			if len(handshake) != need {
				return nil, "", false, core.ErrInvalidArgument
			}
			sni, ech, err := parseClientHelloBody(handshake[4:need])
			return raw, sni, ech, err
		}
	}
	return nil, "", false, core.ErrInvalidArgument
}

func parseClientHelloBody(body []byte) (string, bool, error) {
	pos := 0
	take := func(n int) ([]byte, error) {
		if n < 0 || pos+n > len(body) {
			return nil, io.ErrUnexpectedEOF
		}
		out := body[pos : pos+n]
		pos += n
		return out, nil
	}
	if _, err := take(2 + 32); err != nil {
		return "", false, err
	}
	sessionLenBytes, err := take(1)
	if err != nil {
		return "", false, err
	}
	if _, err := take(int(sessionLenBytes[0])); err != nil {
		return "", false, err
	}
	cipherLenBytes, err := take(2)
	if err != nil {
		return "", false, err
	}
	cipherLen := int(binary.BigEndian.Uint16(cipherLenBytes))
	if cipherLen == 0 || cipherLen%2 != 0 {
		return "", false, core.ErrInvalidArgument
	}
	if _, err := take(cipherLen); err != nil {
		return "", false, err
	}
	compLenBytes, err := take(1)
	if err != nil {
		return "", false, err
	}
	if _, err := take(int(compLenBytes[0])); err != nil {
		return "", false, err
	}
	extLenBytes, err := take(2)
	if err != nil {
		return "", false, err
	}
	extLen := int(binary.BigEndian.Uint16(extLenBytes))
	extensions, err := take(extLen)
	if err != nil || pos != len(body) {
		if err == nil {
			err = core.ErrInvalidArgument
		}
		return "", false, err
	}
	var sni string
	ech := false
	for len(extensions) > 0 {
		if len(extensions) < 4 {
			return "", false, io.ErrUnexpectedEOF
		}
		typ := binary.BigEndian.Uint16(extensions[:2])
		n := int(binary.BigEndian.Uint16(extensions[2:4]))
		extensions = extensions[4:]
		if n > len(extensions) {
			return "", false, io.ErrUnexpectedEOF
		}
		value := extensions[:n]
		extensions = extensions[n:]
		if typ == echExtension {
			ech = true
		}
		if typ == 0 {
			parsed, err := parseSNI(value)
			if err != nil {
				return "", false, err
			}
			if sni != "" && sni != parsed {
				return "", false, core.ErrInvalidArgument
			}
			sni = parsed
		}
	}
	if sni == "" {
		return "", ech, errors.New("TLS ClientHello has no SNI")
	}
	canonical, err := canonicalHostname(sni)
	if err != nil {
		return "", ech, fmt.Errorf("invalid TLS SNI: %w", err)
	}
	return canonical, ech, nil
}

func parseSNI(value []byte) (string, error) {
	if len(value) < 2 {
		return "", io.ErrUnexpectedEOF
	}
	total := int(binary.BigEndian.Uint16(value[:2]))
	value = value[2:]
	if total != len(value) {
		return "", core.ErrInvalidArgument
	}
	var host string
	for len(value) > 0 {
		if len(value) < 3 {
			return "", io.ErrUnexpectedEOF
		}
		typ := value[0]
		n := int(binary.BigEndian.Uint16(value[1:3]))
		value = value[3:]
		if n == 0 || n > len(value) {
			return "", core.ErrInvalidArgument
		}
		name := string(value[:n])
		value = value[n:]
		if typ == 0 {
			if host != "" {
				return "", core.ErrInvalidArgument
			}
			host = name
		}
	}
	if host == "" {
		return "", core.ErrInvalidArgument
	}
	return host, nil
}
