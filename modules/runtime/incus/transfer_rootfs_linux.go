//go:build linux

package incus

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
	incusclient "github.com/lxc/incus/v6/client"
	"github.com/lxc/incus/v6/shared/api"
	"github.com/lxc/incus/v6/shared/cliconfig"
	"golang.org/x/sys/unix"
)

const exportImageOwnerKey = "user.hacocoon.export-owner"

type imageExportConnect func(context.Context) (incusclient.InstanceServer, error)

// localImageExportConnect resolves the same Incus CLI configuration once. Remote
// HTTPS export is explicitly unsupported in this initial local/WSL implementation.
// Never silently fall back from a configured remote to the local daemon.
func localImageExportConnect(project string) (imageExportConnect, error) {
	config, err := cliconfig.LoadConfig("")
	if err != nil {
		return nil, err
	}
	remote, ok := config.Remotes[config.DefaultRemote]
	socket, unixRemote := strings.CutPrefix(remote.Addr, "unix:")
	if !ok || remote.Public || remote.Protocol != "incus" || !unixRemote {
		return nil, core.ErrUnsupported
	}
	socket = strings.TrimPrefix(socket, "//")
	return func(ctx context.Context) (incusclient.InstanceServer, error) {
		server, err := incusclient.ConnectIncusUnixWithContext(ctx, socket, &incusclient.ConnectionArgs{SkipGetEvents: true, TransportWrapper: func(base *http.Transport) incusclient.HTTPTransporter { return &imageExportTransport{base: base} }})
		if err != nil {
			return nil, err
		}
		if server.IsClustered() {
			server.Disconnect()
			return nil, core.ErrUnsupported
		}
		return server.UseProject(project), nil
	}, nil
}

// ExportSnapshotRootfs publishes the independent saved rootfs, streams its native
// unified image archive, and removes only this invocation's transport image.
// The caller must hold ReadSnapshot's source-use boundary until it returns.
func (r *Runtime) ExportSnapshotRootfs(ctx context.Context, c core.SnapshotComponent, root string, limit int64) (*NativeArchive, error) {
	connect, err := localImageExportConnect(r.project)
	if err != nil {
		return nil, err
	}
	return r.exportSnapshotRootfs(ctx, c, root, limit, connect)
}

func (r *Runtime) exportSnapshotRootfs(ctx context.Context, c core.SnapshotComponent, root string, limit int64, connect imageExportConnect) (archive *NativeArchive, err error) {
	binding, err := r.decodeSnapshotComponent(c)
	if err != nil {
		return nil, err
	}
	if binding.Rootfs == nil || c.State != "verified" || limit <= 0 {
		return nil, core.ErrInvalidArgument
	}
	if err := r.verifySnapshotRootfs(ctx, *binding.Rootfs); err != nil {
		return nil, err
	}
	server, err := connect(ctx)
	if err != nil {
		return nil, err
	}
	defer server.Disconnect()
	connection, err := server.GetConnectionInfo()
	if err != nil {
		return nil, err
	}
	if connection.Project != r.project || !filepath.IsAbs(connection.SocketPath) {
		return nil, core.ErrUnsupported
	}
	journal, err := newImageExportJournal(root, r.project, c.NativeRef, connection.SocketPath)
	if err != nil {
		return nil, err
	}
	defer journal.close()
	// Once publication is attempted, a failed/unfinished operation can still
	// create an image. Keep the exact owner receipt instead of guessing cleanup.
	published, fingerprint := false, ""
	defer func() {
		if !published {
			return
		}
		cleanupCtx, cancel := context.WithTimeout(context.Background(), r.cleanupTimeout)
		defer cancel()
		cleanupServer, cleanupErr := connect(cleanupCtx)
		if cleanupErr == nil {
			cleanupErr = removeExportImage(cleanupCtx, cleanupServer, fingerprint, journal.owner)
			cleanupServer.Disconnect()
		}
		if cleanupErr == nil {
			cleanupErr = journal.finish()
		}
		if cleanupErr != nil {
			if archive != nil {
				cleanupErr = errors.Join(cleanupErr, archive.Close())
				archive = nil
			}
			err = errors.Join(err, fmt.Errorf("rootfs export cleanup unconfirmed; receipt %s: %w", journal.path(), core.ErrRecoveryRequired), cleanupErr)
		}
	}()
	request := api.ImagesPost{Source: &api.ImagesPostSource{Type: "instance", Name: binding.Rootfs.target()}, CompressionAlgorithm: "none", Format: "unified"}
	request.Properties = map[string]string{exportImageOwnerKey: journal.owner}
	operation, err := server.CreateImage(request, nil)
	if err != nil {
		return nil, journal.unconfirmed(err)
	}
	if err := journal.record(map[string]string{"operation": operation.Get().ID}); err != nil {
		return nil, journal.unconfirmed(err)
	}
	if err := waitExportOperation(ctx, operation); err != nil {
		return nil, journal.unconfirmed(err)
	}
	fingerprint, _ = operation.Get().Metadata["fingerprint"].(string)
	if err := journal.record(map[string]string{"fingerprint": fingerprint}); err != nil {
		return nil, journal.unconfirmed(err)
	}
	if !baseFingerprintPattern.MatchString(fingerprint) {
		return nil, journal.unconfirmed(core.ErrIncompatibleState)
	}
	published = true
	if err := verifyExportImage(server, fingerprint, journal.owner); err != nil {
		return nil, err
	}
	archive, err = downloadExportImage(ctx, server, fingerprint, root, limit)
	if err != nil {
		return nil, err
	}
	if err := r.verifySnapshotRootfs(ctx, *binding.Rootfs); err != nil {
		err = errors.Join(err, archive.Close())
		archive = nil
		return nil, err
	}
	return archive, nil
}

// Incus wait can return an unfinished operation when its server-side timeout
// elapses without an API error. Only a terminal success permits the next step.
func waitExportOperation(ctx context.Context, operation incusclient.Operation) error {
	if err := operation.WaitContext(ctx); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if operation.Get().StatusCode != api.Success {
		return core.ErrRecoveryRequired
	}
	return nil
}

// The native SDK handles metadata decoding. Bound its input; only the separate
// image export endpoint uses the caller's larger streaming archive budget.
type imageExportTransport struct{ base *http.Transport }

func (t *imageExportTransport) Transport() *http.Transport { return t.base }
func (t *imageExportTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := t.base.RoundTrip(request)
	if err != nil {
		return response, err
	}
	if request.Method != http.MethodGet || !strings.HasSuffix(request.URL.Path, "/export") {
		response.Body = http.MaxBytesReader(nil, response.Body, 1<<20)
	}
	return response, nil
}

func verifyExportImage(server incusclient.InstanceServer, fingerprint, owner string) error {
	image, _, err := server.GetImage(fingerprint)
	if err != nil {
		return err
	}
	if image.Fingerprint != fingerprint || image.Properties[exportImageOwnerKey] != owner || image.Public || image.Type != "container" || len(image.Aliases) != 0 {
		return core.ErrCapabilityStale
	}
	return nil
}

func removeExportImage(ctx context.Context, server incusclient.InstanceServer, fingerprint, owner string) error {
	if err := verifyExportImage(server, fingerprint, owner); err != nil {
		return err
	}
	operation, err := server.DeleteImage(fingerprint)
	if err != nil {
		return err
	}
	if err := waitExportOperation(ctx, operation); err != nil {
		return err
	}
	_, _, err = server.GetImage(fingerprint)
	if !api.StatusErrorCheck(err, 404) {
		return errors.Join(core.ErrRecoveryRequired, err)
	}
	return nil
}

// Stream the native endpoint using the official client's selected Unix transport.
// SDK 6.0.5 GetImageFile does not bind downloads to the connection context and
// opportunistically tries /dev/incus/sock. An explicit request preserves both the
// selected daemon and cancellation. Never open response-selected filenames.
func downloadExportImage(ctx context.Context, server incusclient.InstanceServer, fingerprint, root string, limit int64) (*NativeArchive, error) {
	if !baseFingerprintPattern.MatchString(fingerprint) {
		return nil, core.ErrInvalidArgument
	}
	info, err := server.GetConnectionInfo()
	if err != nil {
		return nil, err
	}
	endpoint, err := url.Parse(info.URL)
	if err != nil || endpoint.Host == "" {
		return nil, core.ErrUnsupported
	}
	endpoint.Path = "/1.0/images/" + fingerprint + "/export"
	query := url.Values{}
	query.Set("project", info.Project)
	endpoint.RawQuery = query.Encode()
	client, err := server.GetHTTPClient()
	if err != nil {
		return nil, err
	}
	transportClient := *client
	transportClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	archive, err := captureNativeArchive(ctx, root, limit, func(output string) error {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
		if err != nil {
			return err
		}
		request.Header.Set("Accept-Encoding", "identity")
		response, err := transportClient.Do(request)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		media, _, _ := mime.ParseMediaType(response.Header.Get("Content-Type"))
		if response.StatusCode != http.StatusOK || strings.HasPrefix(media, "multipart/") || response.ContentLength > limit {
			return core.ErrIncompatibleState
		}
		file, err := os.OpenFile(output, os.O_WRONLY, 0)
		if err != nil {
			return err
		}
		writer := &boundedImageWriter{ctx: ctx, file: file, remaining: limit}
		_, downloadErr := io.Copy(writer, response.Body)
		return errors.Join(downloadErr, file.Close())
	})
	if err != nil {
		return nil, err
	}
	if archive.Digest() != fingerprint {
		return nil, errors.Join(core.ErrIncompatibleState, archive.Close())
	}
	return archive, nil
}

type boundedImageWriter struct {
	ctx       context.Context
	file      *os.File
	remaining int64
}

func (w *boundedImageWriter) Write(p []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	if int64(len(p)) > w.remaining {
		return 0, core.ErrInvalidArgument
	}
	n, err := w.file.Write(p)
	w.remaining -= int64(n)
	return n, err
}

// The append-only receipt identifies a unique native owner before publication,
// then the exact returned operation and image. Uncertain cleanup keeps it for
// explicit inspection; there is no automatic replay or snapshot backup.
type imageExportJournal struct {
	dir               int
	file              *os.File
	root, name, owner string
}

func newImageExportJournal(root, project, source, socket string) (*imageExportJournal, error) {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || len(root) > 4096 {
		return nil, core.ErrInvalidArgument
	}
	dir, err := unix.Openat2(unix.AT_FDCWD, root, &unix.OpenHow{Flags: unix.O_RDONLY | unix.O_DIRECTORY | unix.O_CLOEXEC, Resolve: unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_NO_MAGICLINKS})
	if err != nil {
		return nil, err
	}
	keep := false
	defer func() {
		if !keep {
			unix.Close(dir)
		}
	}()
	var info unix.Stat_t
	if err := unix.Fstat(dir, &info); err != nil {
		return nil, err
	}
	if info.Uid != uint32(os.Geteuid()) || info.Mode&0077 != 0 {
		return nil, core.ErrInvalidArgument
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return nil, err
	}
	owner := hex.EncodeToString(random[:])
	name := "rootfs-export-" + owner + ".jsonl"
	fd, err := unix.Openat(dir, name, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if err != nil {
		return nil, err
	}
	journal := &imageExportJournal{dir: dir, file: os.NewFile(uintptr(fd), name), root: root, name: name, owner: owner}
	if err := journal.record(map[string]string{"owner": owner, "project": project, "source": source, "socket": socket}); err != nil {
		journal.file.Close()
		return nil, err
	}
	if err := unix.Fsync(dir); err != nil {
		journal.file.Close()
		return nil, err
	}
	keep = true
	return journal, nil
}
func (j *imageExportJournal) path() string { return filepath.Join(j.root, j.name) }
func (j *imageExportJournal) record(value map[string]string) error {
	if err := json.NewEncoder(j.file).Encode(value); err != nil {
		return err
	}
	return j.file.Sync()
}
func (j *imageExportJournal) unconfirmed(err error) error {
	return errors.Join(fmt.Errorf("rootfs publication unconfirmed; receipt %s: %w", j.path(), core.ErrRecoveryRequired), err)
}
func (j *imageExportJournal) close() { j.file.Close(); unix.Close(j.dir) }
func (j *imageExportJournal) finish() error {
	var open, named unix.Stat_t
	if err := unix.Fstat(int(j.file.Fd()), &open); err != nil {
		return err
	}
	if err := unix.Fstatat(j.dir, j.name, &named, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return err
	}
	if open.Ino != named.Ino || open.Dev != named.Dev || named.Nlink != 1 || named.Mode&unix.S_IFMT != unix.S_IFREG || named.Uid != uint32(os.Geteuid()) || named.Mode&0077 != 0 {
		return core.ErrCapabilityStale
	}
	if err := unix.Unlinkat(j.dir, j.name, 0); err != nil {
		return err
	}
	return unix.Fsync(j.dir)
}
