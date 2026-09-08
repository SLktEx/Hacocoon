//go:build linux

package sshclient

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const testKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAABAgMEBQYHCAkKCwwNDg8QERITFBUWFxgZGhscHR4f"

type fakeController struct {
	state         core.EnvironmentState
	count, starts int
	connection    core.ClientConnection
	prepare       func()
}

func (f *fakeController) EnvironmentStatus(context.Context, string) (core.EnvironmentStatus, error) {
	return core.EnvironmentStatus{Environment: core.Environment{RuntimeRef: "runtime-owned"}, State: f.state}, nil
}
func (f *fakeController) EnvironmentConnections(context.Context, string) ([]core.ClientConnection, error) {
	return []core.ClientConnection{f.connection}, nil
}
func (f *fakeController) StartEnvironment(context.Context, string) error {
	f.starts++
	f.state = core.EnvironmentRunning
	return nil
}
func (f *fakeController) PrepareEnvironmentSSH(_ context.Context, _ string, r core.SSHAccessRequest) (core.ClientConnection, error) {
	f.count++
	if r.HostPort != 0 || !strings.HasPrefix(r.PublicKey, "ssh-ed25519 ") {
		return core.ClientConnection{}, core.ErrInvalidArgument
	}
	f.connection = core.ClientConnection{ID: "ssh-23001", Kind: "ssh", Host: "127.0.0.1", Port: 23001, TargetPort: 22, User: "root", HostPublicKey: testKey}
	if f.prepare != nil {
		f.prepare()
	}
	return f.connection, nil
}
func (f *fakeController) UnforwardEnvironment(context.Context, string, string) error { return nil }
func TestSetupCreatesClientKeyPinsHostAndReusesResumedConnection(t *testing.T) {
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen unavailable")
	}
	home := t.TempDir()
	if err := os.Mkdir(filepath.Join(home, ".ssh"), 0700); err != nil {
		t.Fatal(err)
	}
	original := "Host personal\n  HostName personal.example\n"
	if err := os.WriteFile(filepath.Join(home, ".ssh/config"), []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	c := &fakeController{state: core.EnvironmentStopped}
	alias, err := Setup(context.Background(), c, Desktop{Home: home}, "dev")
	if err != nil || alias != "haco-dev" || c.starts != 1 || c.count != 1 {
		t.Fatalf("%s %v %+v", alias, err, c)
	}
	main, err := os.ReadFile(filepath.Join(home, ".ssh/config"))
	if err != nil || string(main) != include+original {
		t.Fatalf("%s %v", main, err)
	}
	managed, err := os.ReadFile(filepath.Join(home, ".ssh/hacocoon/dev.conf"))
	if err != nil || !strings.Contains(string(managed), "StrictHostKeyChecking yes") {
		t.Fatalf("%s %v", managed, err)
	}
	keyBefore, err := os.ReadFile(filepath.Join(home, ".ssh/hacocoon/identity"))
	if err != nil {
		t.Fatal(err)
	}
	c.state = core.EnvironmentStopped
	if _, err = Setup(context.Background(), c, Desktop{Home: home}, "dev"); err != nil {
		t.Fatal(err)
	}
	keyAfter, _ := os.ReadFile(filepath.Join(home, ".ssh/hacocoon/identity"))
	if c.count != 1 || c.starts != 2 || string(keyBefore) != string(keyAfter) {
		t.Fatal("setup rotated credentials or failed resume")
	}
	configs, _ := filepath.Glob(filepath.Join(home, ".ssh/hacocoon/known-*"))
	if len(configs) != 1 {
		t.Fatalf("known_hosts=%v", configs)
	}
	known, _ := os.ReadFile(configs[0])
	if string(known) != "haco-dev "+testKey+"\n" {
		t.Fatalf("%s", known)
	}
}
func TestFilesRejectSymlinkAndHardlinkInputs(t *testing.T) {
	home := t.TempDir()
	f, err := openFiles(context.Background(), home, false)
	if err != nil {
		t.Fatal(err)
	}
	defer f.close()
	victim := filepath.Join(home, "victim")
	if err = os.WriteFile(victim, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, link := range []func(string, string) error{os.Symlink, os.Link} {
		target := filepath.Join(home, ".ssh/config")
		if err = link(victim, target); err != nil {
			t.Fatal(err)
		}
		if err = f.replace("config", []byte("replace")); err == nil {
			t.Fatal("accepted linked SSH config")
		}
		if err = os.Remove(target); err != nil {
			t.Fatal(err)
		}
	}
	b, _ := os.ReadFile(victim)
	if string(b) != "preserve" {
		t.Fatal("overwrote unrelated file")
	}
}
func TestInvalidHostResponseRetainsRecoveryEvidence(t *testing.T) {
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen unavailable")
	}
	c := &fakeController{state: core.EnvironmentRunning}
	c.prepare = func() { c.connection.Host = "192.0.2.1" }
	home := t.TempDir()
	_, err := Setup(context.Background(), c, Desktop{Home: home}, "dev")
	if !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("lost recovery state: %v", err)
	}
	if _, err = os.Stat(filepath.Join(home, ".ssh/hacocoon/dev.conf")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("published invalid connection")
	}
}
func TestRenderRejectsSSHConfigInjection(t *testing.T) {
	valid := saved{Runtime: "owned", Connection: core.ClientConnection{Kind: "ssh", Host: "127.0.0.1", Port: 23000, TargetPort: 22, User: "root", HostPublicKey: testKey}}
	for _, name := range []string{"bad\nHost *", "../other", "-option"} {
		if _, _, err := render(name, valid); err == nil {
			t.Fatal("accepted injected alias")
		}
	}
	valid.Connection.HostPublicKey += "\nProxyCommand evil"
	if _, _, err := render("dev", valid); err == nil {
		t.Fatal("accepted injected key")
	}
}

func TestManagedConfigPreservesVSCodeDynamicForward(t *testing.T) {
	ssh, err := exec.LookPath("ssh")
	if err != nil {
		t.Skip("OpenSSH client unavailable")
	}
	config, _, err := render("dev", saved{Runtime: "owned", Connection: core.ClientConnection{Kind: "ssh", Host: "127.0.0.1", Port: 23000, TargetPort: 22, User: "root", HostPublicKey: testKey}})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config")
	if err = os.WriteFile(path, []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(ssh, "-G", "-D", "127.0.0.1:49101", "-F", path, "haco-dev").Output()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "dynamicforward [127.0.0.1]:49101") {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(line, "forward") {
				t.Log(line)
			}
		}
		t.Fatal("managed SSH config discarded the VS Code forwarding request")
	}
}
