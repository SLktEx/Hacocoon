//go:build windows && (amd64 || arm64)

package wslreclaim

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"os"
	"path/filepath"
	"testing"
)

func TestPreparedWorkerRejectsInvalidRequestBeforeLaunch(t *testing.T) {
	id := "{11111111-1111-4111-8111-111111111111}"
	for _, args := range [][2]string{{"", id}, {id, ""}, {"--shutdown", id}, {id, "{00000000-0000-0000-0000-000000000000}"}} {
		if pid, err := LaunchPreparedWorker(context.Background(), args[0], args[1]); err == nil || pid != 0 {
			t.Fatal(pid, err)
		}
		if err := ExecutePreparedWorker(context.Background(), args[0], args[1]); err == nil {
			t.Fatal("invalid worker request accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if pid, err := LaunchPreparedWorker(ctx, id, id); !errors.Is(err, context.Canceled) || pid != 0 {
		t.Fatal(pid, err)
	}
	if err := ExecutePreparedWorker(ctx, id, id); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestPinnedExecutableExcludesWriteAndRename(t *testing.T) {
	var empty pinnedDisk
	if err := empty.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := empty.Allocation(); err == nil {
		t.Fatal("empty disk has allocation")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "worker.exe")
	if err := os.WriteFile(path, []byte("isolated fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	pin, err := pinLocalFile(path, ".exe", false)
	if err != nil {
		t.Fatal(err)
	}
	defer pin.Close()
	if file, err := os.OpenFile(path, os.O_WRONLY, 0); err == nil {
		file.Close()
		t.Fatal("pinned executable writable")
	}
	if err := os.Rename(path, path+".moved"); err == nil {
		t.Fatal("pinned executable renamed")
	}
	if _, err := pinDisk(path); err == nil {
		t.Fatal("executable accepted as disk")
	}
	if err := pin.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path, path+".moved"); err != nil {
		t.Fatal("executable pin leaked", err)
	}
}

func TestPreparedStatusIsReadOnlyAndPendingIsUnknown(t *testing.T) {
	id, err := windows.GenerateGUID()
	if err != nil {
		t.Fatal(err)
	}
	operation, err := windows.GenerateGUID()
	if err != nil {
		t.Fatal(err)
	}
	path := `Software\Hacocoon\Reclamation\` + id.String()
	if _, err := ReadPreparedStatus(context.Background(), id.String(), operation.String()); err == nil {
		t.Fatal("missing result accepted")
	}
	if key, err := registry.OpenKey(registry.CURRENT_USER, path, registry.QUERY_VALUE); err == nil {
		key.Close()
		t.Fatal("status created a key")
	}
	key, existed, err := registry.CreateKey(registry.CURRENT_USER, path, registry.ALL_ACCESS)
	if err != nil {
		t.Fatal(err)
	}
	if existed {
		key.Close()
		t.Fatal("fixture collision")
	}
	defer func() {
		key.Close()
		if err := registry.DeleteKey(registry.CURRENT_USER, path); err != nil {
			t.Error(err)
		}
	}()
	record := operationRecord{Version: 1, Operation: operation, Registration: registration{ID: id, Name: "Hacocoon-Test", BasePath: `C:\owned`, VHDFileName: "ext4.vhdx"}, Disk: diskIdentity{Volume: 1, Low: 2}, State: "pending"}
	store := &operationStore{key: key}
	if err := store.write(record); err != nil {
		t.Fatal(err)
	}
	before, _, err := key.GetBinaryValue("Operation")
	if err != nil {
		t.Fatal(err)
	}
	status, err := ReadPreparedStatus(context.Background(), id.String(), operation.String())
	if err != nil || status.State != "pending" || status.Observation != nil {
		t.Fatal("pending claimed native results", status, err)
	}
	raw, err := json.Marshal(status)
	if err != nil || bytes.Contains(raw, []byte("observation")) {
		t.Fatal(string(raw), err)
	}
	if _, err := ReadPreparedStatus(context.Background(), id.String(), id.String()); err == nil {
		t.Fatal("foreign operation accepted")
	}
	after, _, err := key.GetBinaryValue("Operation")
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("read changed saved bytes", err)
	}
	if err := store.finish(record, continuationObservation{StopAttempted: true}, errors.New("failed")); err != nil {
		t.Fatal(err)
	}
	status, err = ReadPreparedStatus(context.Background(), id.String(), operation.String())
	if err != nil || status.State != "failed" || status.Observation == nil || !status.Observation.StopAttempted || status.Observation.Resumed {
		t.Fatal("failed result changed", status, err)
	}
}

func TestWorkerConsoleBoundary(t *testing.T) {
	console, _, _ := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetConsoleWindow").Call()
	err := requireIndependentWorker()
	if (console != 0) != (err != nil) {
		t.Fatalf("console boundary mismatch: console=%v error=%v", console != 0, err)
	}
}

func TestWorkerReadinessRequiresExactFrameAndEOF(t *testing.T) {
	for _, frame := range []string{"RDY\n", "", "RDY", "RDY\nextra", "FAIL"} {
		read, write, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := write.Write([]byte(frame)); err != nil {
			t.Fatal(err)
		}
		write.Close()
		err = readWorkerReady(context.Background(), read)
		read.Close()
		if (err == nil) != (frame == "RDY\n") {
			t.Fatalf("readiness frame rejected/accepted incorrectly: %v", err)
		}
	}
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer write.Close()
	defer read.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := readWorkerReady(ctx, read); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
