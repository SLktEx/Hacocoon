package egress

import (
	"encoding/binary"
	"testing"
)

func clientHelloBody(host string, ech bool) []byte {
	body := make([]byte, 0, 128)
	body = append(body, 0x03, 0x03)
	body = append(body, make([]byte, 32)...)
	body = append(body, 0)
	body = append(body, 0, 2, 0x13, 0x01)
	body = append(body, 1, 0)

	name := []byte(host)
	sniValue := make([]byte, 2+1+2+len(name))
	binary.BigEndian.PutUint16(sniValue[:2], uint16(1+2+len(name)))
	sniValue[2] = 0
	binary.BigEndian.PutUint16(sniValue[3:5], uint16(len(name)))
	copy(sniValue[5:], name)

	extensions := make([]byte, 4+len(sniValue))
	binary.BigEndian.PutUint16(extensions[:2], 0)
	binary.BigEndian.PutUint16(extensions[2:4], uint16(len(sniValue)))
	copy(extensions[4:], sniValue)
	if ech {
		extra := make([]byte, 4)
		binary.BigEndian.PutUint16(extra[:2], echExtension)
		binary.BigEndian.PutUint16(extra[2:4], 0)
		extensions = append(extensions, extra...)
	}
	length := make([]byte, 2)
	binary.BigEndian.PutUint16(length, uint16(len(extensions)))
	body = append(body, length...)
	body = append(body, extensions...)
	return body
}

func TestClientHelloSNIIsCanonicalAndExact(t *testing.T) {
	sni, ech, err := parseClientHelloBody(clientHelloBody("API.Example.COM", false))
	if err != nil {
		t.Fatal(err)
	}
	if sni != "api.example.com" || ech {
		t.Fatalf("sni=%q ech=%t", sni, ech)
	}
	if sni == "other.example.com" {
		t.Fatal("mismatched SNI unexpectedly matched another CONNECT authority")
	}
}

func TestClientHelloECHIsDetectedFailClosed(t *testing.T) {
	sni, ech, err := parseClientHelloBody(clientHelloBody("api.example.com", true))
	if err != nil {
		t.Fatal(err)
	}
	if sni != "api.example.com" || !ech {
		t.Fatalf("sni=%q ech=%t", sni, ech)
	}
}

func TestClientHelloRejectsMissingSNI(t *testing.T) {
	body := clientHelloBody("api.example.com", false)
	// Remove all extensions while preserving a syntactically complete prefix.
	body = body[:2+32+1+2+2+1+1]
	body = append(body, 0, 0)
	if _, _, err := parseClientHelloBody(body); err == nil {
		t.Fatal("missing SNI was accepted")
	}
}
