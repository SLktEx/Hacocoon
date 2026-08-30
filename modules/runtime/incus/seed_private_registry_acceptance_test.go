package incus

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/internal/seedbuild"
)

const privateRegistryAcceptanceEnv = "HACO_E2E_PRIVATE_REGISTRY"

type acceptanceRegistryImage struct {
	manifest       []byte
	manifestDigest string
	config         []byte
	configDigest   string
	layer          []byte
	layerDigest    string
}

func TestPrivateRegistryHostAcquisitionAcceptance(t *testing.T) {
	if os.Getenv(privateRegistryAcceptanceEnv) != "1" {
		t.Skip("set HACO_E2E_PRIVATE_REGISTRY=1 on a host with containerd + nerdctl")
	}
	if _, err := exec.LookPath("nerdctl"); err != nil {
		t.Fatalf("nerdctl is required: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if result, err := (host.ExecRunner{}).Run(ctx, "nerdctl", "--namespace", seedHostNamespace, "info"); err != nil {
		t.Fatalf("containerd/nerdctl is not usable: %v\nstderr: %s", err, result.Stderr)
	}

	const (
		username = "haco-acceptance"
		password = "haco-private-registry-sentinel-7b1f4c"
	)
	image := buildAcceptanceRegistryImage(t, "host-acquisition-ok")
	server := newAcceptanceRegistryServer(t, username, password, image)
	defer server.Close()

	// nerdctl intentionally treats loopback registries as HTTP. This acceptance
	// test isolates the Host-owned authentication boundary; production registry
	// TLS trust configuration is deployment-specific and is not asserted here.
	hostport := strings.TrimPrefix(server.URL, "http://")
	configureAcceptanceRegistryCredentials(t, hostport, username, password)

	runtime := New(host.ExecRunner{})
	provider, err := NewSandboxProvider(runtime)
	if err != nil {
		t.Fatal(err)
	}
	identity := seedbuild.ImageIdentity{
		Reference: hostport + "/hacocoon/private:stable",
		Digest:    image.manifestDigest,
	}
	archive, cleanup, err := provider.exportSeedImages(ctx, []seedbuild.ImageIdentity{identity})
	if err != nil {
		t.Fatalf("authenticated trusted-Host acquisition failed: %v", err)
	}
	defer cleanup()
	archiveBytes, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	if len(archiveBytes) == 0 {
		t.Fatal("Seed export archive is empty")
	}
	if bytes.Contains(archiveBytes, []byte(password)) || bytes.Contains(archiveBytes, []byte(username+":"+password)) {
		t.Fatal("Host registry credential leaked into Seed export archive")
	}

	inspect, err := (host.ExecRunner{}).Run(ctx, "nerdctl", "--namespace", seedHostNamespace, "image", "inspect", identity.String())
	if err != nil || inspect.ExitCode != 0 {
		t.Fatalf("exact immutable identity is not present in trusted Host Seed namespace: %v\nstderr: %s", err, inspect.Stderr)
	}

	badImage := buildAcceptanceRegistryImage(t, "host-acquisition-auth-failure")
	badServer := newAcceptanceRegistryServer(t, username, password+"-different", badImage)
	defer badServer.Close()
	badHostport := strings.TrimPrefix(badServer.URL, "http://")
	configureAcceptanceRegistryCredentials(t, badHostport, username, "definitely-wrong")
	badIdentity := seedbuild.ImageIdentity{
		Reference: badHostport + "/hacocoon/private:stable",
		Digest:    badImage.manifestDigest,
	}
	if archive, cleanup, err := provider.exportSeedImages(ctx, []seedbuild.ImageIdentity{badIdentity}); err == nil {
		cleanup()
		t.Fatalf("authenticated acquisition unexpectedly succeeded with invalid Host credential; archive=%q", archive)
	}
}

func buildAcceptanceRegistryImage(t *testing.T, marker string) acceptanceRegistryImage {
	t.Helper()
	var tarBytes bytes.Buffer
	tw := tar.NewWriter(&tarBytes)
	payload := []byte(marker + "\n")
	if err := tw.WriteHeader(&tar.Header{Name: "marker.txt", Mode: 0o644, Size: int64(len(payload))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	diffID := digestBytes(tarBytes.Bytes())

	var layer bytes.Buffer
	zw := gzip.NewWriter(&layer)
	if _, err := zw.Write(tarBytes.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	layerDigest := digestBytes(layer.Bytes())

	config, err := json.Marshal(map[string]any{
		"architecture": "amd64",
		"os":           "linux",
		"config":       map[string]any{},
		"rootfs": map[string]any{
			"type":     "layers",
			"diff_ids": []string{diffID},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	configDigest := digestBytes(config)
	manifest, err := json.Marshal(map[string]any{
		"schemaVersion": 2,
		"mediaType":     "application/vnd.oci.image.manifest.v1+json",
		"config": map[string]any{
			"mediaType": "application/vnd.oci.image.config.v1+json",
			"digest":    configDigest,
			"size":      len(config),
		},
		"layers": []map[string]any{{
			"mediaType": "application/vnd.oci.image.layer.v1.tar+gzip",
			"digest":    layerDigest,
			"size":      layer.Len(),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return acceptanceRegistryImage{
		manifest:       manifest,
		manifestDigest: digestBytes(manifest),
		config:         config,
		configDigest:   configDigest,
		layer:          layer.Bytes(),
		layerDigest:    layerDigest,
	}
}

func digestBytes(value []byte) string {
	sum := sha256.Sum256(value)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func newAcceptanceRegistryServer(t *testing.T, username, password string, image acceptanceRegistryImage) *httptest.Server {
	t.Helper()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser, gotPassword, ok := r.BasicAuth()
		if !ok || gotUser != username || gotPassword != password {
			w.Header().Set("WWW-Authenticate", `Basic realm="hacocoon acceptance"`)
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Docker-Distribution-Api-Version", "registry/2.0")
		switch r.URL.Path {
		case "/v2/", "/v2":
			w.WriteHeader(http.StatusOK)
			return
		case "/v2/hacocoon/private/manifests/" + image.manifestDigest:
			serveAcceptanceRegistryObject(w, r, "application/vnd.oci.image.manifest.v1+json", image.manifestDigest, image.manifest)
			return
		case "/v2/hacocoon/private/blobs/" + image.configDigest:
			serveAcceptanceRegistryObject(w, r, "application/vnd.oci.image.config.v1+json", image.configDigest, image.config)
			return
		case "/v2/hacocoon/private/blobs/" + image.layerDigest:
			serveAcceptanceRegistryObject(w, r, "application/vnd.oci.image.layer.v1.tar+gzip", image.layerDigest, image.layer)
			return
		default:
			http.NotFound(w, r)
		}
	})
	return httptest.NewServer(handler)
}

func serveAcceptanceRegistryObject(w http.ResponseWriter, r *http.Request, mediaType, digest string, body []byte) {
	w.Header().Set("Content-Type", mediaType)
	w.Header().Set("Docker-Content-Digest", digest)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(body)
	}
}

func configureAcceptanceRegistryCredentials(t *testing.T, hostport, username, password string) {
	t.Helper()
	dockerConfig := t.TempDir()
	auth := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	config, err := json.Marshal(map[string]any{
		"auths": map[string]any{
			hostport: map[string]string{"auth": auth},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dockerConfig, "config.json"), config, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOCKER_CONFIG", dockerConfig)
}
