package main

import (
	"bytes"
	"github.com/SLktEx/Hacocoon/internal/core"
	"strings"
	"testing"
)

func TestSSHConfigRefusesUnsafeTargetsAndAmbiguousConnections(t *testing.T) {
	good := core.ClientConnection{Kind: "ssh", Host: "127.0.0.1", Port: 2223, User: "root"}
	var out bytes.Buffer
	if err := writeSSHConfig(&out, "dev", []core.ClientConnection{good}); err != nil || !strings.Contains(out.String(), "Host haco-dev\n  HostName 127.0.0.1\n  Port 2223") {
		t.Fatalf("%s %v", &out, err)
	}
	if writeSSHConfig(&out, "dev\nProxyCommand evil", []core.ClientConnection{good}) == nil {
		t.Fatal("config injection")
	}
	for _, list := range [][]core.ClientConnection{nil, {good, good}, {{Kind: "ssh", Host: "evil.example", Port: 22, User: "root"}}, {{Kind: "ssh", Host: "127.0.0.1", Port: 22, User: "root\nProxyCommand evil"}}} {
		if writeSSHConfig(&out, "dev", list) == nil {
			t.Fatal("unsafe/ambiguous connection")
		}
	}
}
