//go:build linux

package incus

import (
	"context"
	"net/netip"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/host"
)

// Run only through tools/test_trusted_host_forwarding.sh. Both a root check and
// a distinct netns are required before any rule or link is changed. The test
// never changes the host firewall and does not prepare a Windows BAT fixture.
func TestTrustedHostForwardingKernel(t *testing.T) {
	if os.Getenv("HACO_FORWARDING_KERNEL_TEST") != "isolated" {
		t.Skip("explicit isolated kernel regression only")
	}
	if os.Geteuid() != 0 {
		t.Skip("requires isolated root network namespace")
	}
	self, err := os.Readlink("/proc/self/ns/net")
	if err != nil {
		t.Fatal(err)
	}
	initNS, err := os.Readlink("/proc/1/ns/net")
	if err != nil {
		t.Fatal(err)
	}
	if self == initNS {
		t.Skip("refusing to modify the host network namespace")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	run := func(name string, args ...string) string {
		t.Helper()
		out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
		if err != nil {
			t.Fatalf("%s %v: %v: %s", name, args, err, out)
		}
		return string(out)
	}
	guest := func(hostIF, peerIF, address, gateway string) string {
		t.Helper()
		cmd := exec.CommandContext(ctx, "unshare", "--net", "sleep", "60")
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
		pid := strconv.Itoa(cmd.Process.Pid)
		ready := false
		for range 100 {
			ns, err := os.Readlink("/proc/" + pid + "/ns/net")
			if err == nil && ns != self {
				ready = true
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		if !ready {
			t.Fatal("child network namespace did not become ready")
		}
		run("ip", "link", "add", hostIF, "type", "veth", "peer", "name", peerIF)
		run("ip", "link", "set", peerIF, "netns", pid)
		run("ip", "address", "add", gateway+"/24", "dev", hostIF)
		run("ip", "link", "set", hostIF, "up")
		run("nsenter", "-t", pid, "-n", "ip", "link", "set", "lo", "up")
		run("nsenter", "-t", pid, "-n", "ip", "address", "add", address+"/24", "dev", peerIF)
		run("nsenter", "-t", pid, "-n", "ip", "link", "set", peerIF, "up")
		run("nsenter", "-t", pid, "-n", "ip", "route", "add", "default", "via", gateway)
		return pid
	}
	trusted := guest(trustedHostNetwork, "trusted0", "10.70.80.2", "10.70.80.1")
	untrusted := guest("hbr-test-env", "env0", "10.71.80.2", "10.71.80.1")
	external := guest("test-uplink", "external0", "203.0.113.2", "203.0.113.1")
	run("sysctl", "-w", "net.ipv4.ip_forward=1")
	run("iptables", "-P", "FORWARD", "DROP")
	run("iptables", "-N", "DOCKER-USER")
	run("iptables", "-A", "FORWARD", "-j", "DOCKER-USER")
	run("iptables", "-A", "DOCKER-USER", "-j", "RETURN")
	ping := func(pid, address string, allowed bool) {
		t.Helper()
		out, err := exec.CommandContext(ctx, "nsenter", "-t", pid, "-n", "ping", "-c", "1", "-W", "1", address).CombinedOutput()
		if (err == nil) != allowed {
			t.Fatalf("ping %s -> %s allowed=%v err=%v output=%s", pid, address, allowed, err, out)
		}
	}
	ping(trusted, "203.0.113.2", false)
	runtime := New(host.ExecRunner{})
	if err := runtime.ensureTrustedHostForwarding(ctx, netip.MustParsePrefix("10.70.80.0/24")); err != nil {
		t.Fatal(err)
	}
	ping(trusted, "203.0.113.2", true)
	ping(external, "10.70.80.2", false)
	ping(untrusted, "203.0.113.2", false)
	before := run("iptables", "-S")
	if !strings.Contains(before, "-P FORWARD DROP\n") {
		t.Fatal("global DROP policy changed")
	}
	if err := runtime.ensureTrustedHostForwarding(ctx, netip.MustParsePrefix("10.70.80.0/24")); err != nil {
		t.Fatal(err)
	}
	if after := run("iptables", "-S"); after != before {
		t.Fatalf("non-idempotent rules: %s", after)
	}
}
