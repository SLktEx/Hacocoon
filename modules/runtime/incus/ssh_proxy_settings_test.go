package incus

import (
	"strings"
	"testing"
)

func TestSSHSessionProxySettingsAreCurrentAndLimited(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://private-credential@untrusted.invalid")
	t.Setenv("LD_PRELOAD", "/untrusted.so")
	words := strings.Fields(managedSSHProxySettings())
	if len(words) != 7 || words[0] != "SetEnv" {
		t.Fatal("unexpected SSH settings")
	}
	expected := map[string]bool{"HTTP_PROXY": false, "HTTPS_PROXY": false, "NO_PROXY": false, "http_proxy": false, "https_proxy": false, "no_proxy": false}
	for _, word := range words[1:] {
		key, value, ok := strings.Cut(word, "=")
		seen, allowed := expected[key]
		if !ok || !allowed || seen || strings.ContainsAny(value, "\r\n\"'@") {
			t.Fatal("unmanaged SSH environment")
		}
		expected[key] = true
		if key == "NO_PROXY" || key == "no_proxy" {
			if value != "localhost,127.0.0.1,::1" {
				t.Fatal("broadened bypass")
			}
		} else if !strings.HasPrefix(value, "http://"+sandboxRoutedHostIPv4+":") {
			t.Fatal("caller environment reached SSH")
		}
	}
}
