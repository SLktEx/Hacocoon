// installed-egress-check is a CI acceptance client, not a shipped command. It
// uses the installed controller as an ordinary Physical Host user and runs its
// network probe as a normal read-only Workspace workload inside one Environment.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
)

const passed = "allowed_https=verified denied_proxy=403 direct_tcp=blocked management_sockets=absent"

func main() {
	var err error
	if len(os.Args) != 3 {
		err = errors.New("usage: installed-egress-check check <environment> | probe <public-ipv4>")
	} else if os.Args[1] == "check" {
		err = check(os.Args[2])
	} else if os.Args[1] == "probe" {
		err = probe(os.Args[2])
	} else {
		err = errors.New("unknown check mode")
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func check(name string) (result error) {
	if os.Geteuid() == 0 || !regexp.MustCompile(`^m1-egress-[a-f0-9]{16}$`).MatchString(name) {
		return errors.New("check requires the ordinary WSL user and a unique acceptance name")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	client, err := controlapi.NewClient(control.DefaultSocketPath)
	if err != nil {
		return err
	}
	// Positive control: the exact public endpoint must be reachable from the
	// Physical Host before its denied direct path is tested in the Environment.
	addresses, err := net.DefaultResolver.LookupIP(ctx, "ip4", "github.com")
	if err != nil || len(addresses) == 0 || !publicIPv4(addresses[0]) {
		return errors.New("Physical Host github.com IPv4 lookup failed")
	}
	address := addresses[0].String()
	connection, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp4", net.JoinHostPort(address, "443"))
	if err != nil {
		return errors.New("Physical Host positive TCP control failed")
	}
	connection.Close()
	workspace, err := os.MkdirTemp("", name+"-")
	if err != nil {
		return err
	}
	// Read-only workspaces retain normal file modes rather than granting a UID
	// mapping. This directory contains only this public, executable probe.
	if err := os.Chmod(workspace, 0o755); err != nil {
		return err
	}
	if err := copyProbe(filepath.Join(workspace, "probe")); err != nil {
		return err
	}
	created, err := client.CreateEnvironment(ctx, controlapi.EnvironmentCreateRequest{
		Name: name, WorkspacePath: workspace, AccessMode: core.WorkspaceReadOnly,
	})
	if err != nil {
		return fmt.Errorf("controller create failed; retain workspace %s: %w", workspace, err)
	}
	if created.Name != name || created.RuntimeRef == "" || created.Workspace.Path != workspace {
		return errors.New("unexpected created identity; retaining resources for inspection")
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), time.Minute)
		defer stop()
		if err := client.DeleteEnvironment(cleanup, name); err != nil {
			result = errors.Join(result, fmt.Errorf("controller cleanup failed; retain workspace %s: %w", workspace, err))
			return
		}
		remaining, err := client.ListEnvironments(cleanup)
		if err != nil {
			result = errors.Join(result, errors.New("cleanup inventory unavailable; retaining workspace"))
			return
		}
		for _, environment := range remaining {
			if environment.Name == name {
				result = errors.Join(result, errors.New("deleted Environment still present; retaining workspace"))
				return
			}
		}
		// Remove only known files after successful controller cleanup. Never
		// recursively delete a Workspace after an uncertain lifecycle outcome.
		result = errors.Join(result, os.Remove(filepath.Join(workspace, "probe")))
		result = errors.Join(result, os.Remove(workspace))
	}()
	executed, err := client.ExecEnvironment(ctx, name, []string{"/workspace/probe", "probe", address})
	if err != nil {
		return fmt.Errorf("controller exec failed: %w", err)
	}
	if executed.ExitCode != 0 || executed.StdoutTruncated || executed.StderrTruncated || strings.TrimSpace(executed.Stdout) != passed {
		// Do not copy arbitrary guest output into acceptance logs.
		phase := "unknown"
		for fragment, label := range map[string]string{
			"management endpoint": "management_boundary", "guest DHCP/default route": "network_startup",
			"allowed HTTPS": "allowed_https", "unapproved proxy": "denied_proxy", "direct Environment TCP": "direct_tcp",
		} {
			if strings.Contains(executed.Stderr, fragment) {
				phase = label
			}
		}
		proxyStatus := "unknown"
		if match := regexp.MustCompile(`proxy status ([0-9]{1,3})\)`).FindStringSubmatch(executed.Stderr); len(match) == 2 {
			proxyStatus = match[1]
		}
		return fmt.Errorf("Environment egress probe failed (phase %s, proxy status %s, exit %d)", phase, proxyStatus, executed.ExitCode)
	}
	fmt.Println(passed)
	return nil
}

func copyProbe(destination string) error {
	source, err := os.Executable()
	if err != nil {
		return err
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o755)
	if err != nil {
		return err
	}
	if err := output.Chmod(0o755); err != nil {
		output.Close()
		return err
	}
	_, copyErr := io.Copy(output, input)
	return errors.Join(copyErr, output.Close())
}

func publicIPv4(ip net.IP) bool {
	if ip.To4() == nil || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	address, err := netip.ParseAddr(ip.String())
	if err != nil {
		return false
	}
	for _, prefix := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "192.88.99.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4"} {
		if netip.MustParsePrefix(prefix).Contains(address) {
			return false
		}
	}
	return true
}

func probe(address string) error {
	if !publicIPv4(net.ParseIP(address)) {
		return errors.New("invalid public IPv4 probe target")
	}
	for _, path := range []string{control.DefaultSocketPath, "/var/lib/hacocoon-control.sock", "/var/lib/incus/unix.socket"} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			return errors.New("unexpected management endpoint in Environment")
		}
	}
	// Observe initial DHCP/route readiness, without repairing it or retrying
	// failed external requests. The workload needs no curl or extra packages.
	ready := false
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		routes, err := os.ReadFile("/proc/net/route")
		if err != nil {
			return errors.New("guest route observation failed")
		}
		for _, line := range strings.Split(string(routes), "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 4 && fields[1] == "00000000" && fields[2] != "00000000" {
				ready = true
			}
		}
		if ready {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	if !ready {
		return errors.New("guest DHCP/default route did not become ready")
	}
	if err := proxyRequest("github.com", true); err != nil {
		return err
	}
	if err := proxyRequest("example.com", false); err != nil {
		return err
	}
	connection, err := net.DialTimeout("tcp4", net.JoinHostPort(address, "443"), 3*time.Second)
	if err == nil {
		connection.Close()
		return errors.New("direct Environment TCP egress unexpectedly succeeded")
	}
	var operation *net.OpError
	if !errors.As(err, &operation) || operation.Op != "dial" {
		return errors.New("direct egress was not tested by a TCP dial")
	}
	fmt.Println(passed)
	return nil
}

func proxyRequest(host string, allowed bool) error {
	proxyStatus := 0
	transport := &http.Transport{
		Proxy:               http.ProxyURL(&url.URL{Scheme: "http", Host: "169.254.254.1:18080"}),
		DialContext:         (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
		TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: 10 * time.Second,
		DisableKeepAlives: true,
		OnProxyConnectResponse: func(_ context.Context, _ *url.URL, _ *http.Request, response *http.Response) error {
			proxyStatus = response.StatusCode
			return nil
		},
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 25 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Get("https://" + host + "/")
	if response != nil {
		defer response.Body.Close()
	}
	if !allowed {
		if err == nil || proxyStatus != http.StatusForbidden {
			return errors.New("unapproved proxy destination was not refused with 403")
		}
		return nil
	}
	if err != nil || proxyStatus != http.StatusOK || response.StatusCode < 200 || response.StatusCode >= 400 || response.TLS == nil || len(response.TLS.VerifiedChains) == 0 {
		return fmt.Errorf("allowed HTTPS did not complete with verified TLS (proxy status %d)", proxyStatus)
	}
	return nil
}
