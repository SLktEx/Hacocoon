package clientadapter

import (
	"github.com/SLktEx/Hacocoon/internal/core"
	"testing"
)

func TestConnectionHostKeyProjectionRejectsInjection(t *testing.T) {
	raw := core.ClientConnection{ID: "ssh-2222", Kind: "ssh", Host: "127.0.0.1", Port: 2222, TargetPort: 22, User: "root", HostPublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAABAgMEBQYHCAkKCwwNDg8QERITFBUWFxgZGhscHR4f comment"}
	c, err := projectConnection(raw)
	if err != nil || c.HostPublicKey != "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAABAgMEBQYHCAkKCwwNDg8QERITFBUWFxgZGhscHR4f" {
		t.Fatalf("%+v %v", c, err)
	}
	raw.HostPublicKey += "\nHost *\n ProxyCommand evil"
	if _, err := projectConnection(raw); err == nil {
		t.Fatal("accepted host key config injection")
	}
}
