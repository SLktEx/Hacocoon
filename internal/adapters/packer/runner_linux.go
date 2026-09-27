//go:build linux

package packer

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	basebuild "github.com/SLktEx/Hacocoon/internal/base/build"
	"github.com/SLktEx/Hacocoon/internal/core"
)

const root = "/var/lib/hacocoon-packer/builds"

// Build is a local trusted-host operation. The only controller operation is the
// existing bounded archive upload. A lost reply is never retried.
func Build(ctx context.Context, template basebuild.PackerTemplate, request basebuild.ImportRequest, client Importer) (result basebuild.Result, err error) {
	name := request.Name
	if template.Validate() != nil || request.Validate() != nil || request.Artifact != nil || client == nil {
		return result, core.ErrInvalidArgument
	}
	var nonce [16]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return result, err
	}
	id := hex.EncodeToString(nonce[:])
	directory := filepath.Join(root, id)
	unit := "haco-packer-" + id
	result = basebuild.Result{Base: core.BaseInfo{Name: name}, Builder: unit, State: "recovery-required"}
	input, _ := json.Marshal(template)
	fileLimit := "infinity"
	if request.MaxBytes != 0 {
		fileLimit = strconv.FormatInt(request.ArchiveLimit(), 10)
	}
	command := exec.CommandContext(ctx, "/usr/bin/systemd-run", "--unit="+unit, "--collect", "--wait", "--pipe", "--quiet",
		"--expand-environment=no", "--service-type=exec", "--property=KillMode=control-group",
		"--property=RuntimeMaxSec=infinity", "--property=TimeoutStopSec=10s",
		"--property=LimitFSIZE="+fileLimit, "/usr/bin/python3", "-I", "-c", program, id, string(name), strconv.FormatInt(request.ArchiveLimit(), 10))
	command.Stdin = bytes.NewReader(input)
	command.WaitDelay = 15 * time.Second
	// Never forward arbitrary Packer/child output into errors, stdout or logs.
	runErr := command.Run()
	if ctx.Err() != nil {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 20*time.Second)
		defer cancel()
		_ = exec.CommandContext(cleanup, "/usr/bin/systemctl", "stop", unit+".service").Run()
		return result, fmt.Errorf("packer canceled; inspect receipt %s: %w", directory, errors.Join(ctx.Err(), core.ErrRecoveryRequired))
	}
	record, e := readReceipt(directory)
	if e != nil {
		return result, fmt.Errorf("packer did not produce a confirmed receipt %s: %w", directory, core.ErrRecoveryRequired)
	}
	result.Stage, _ = record["stage"].(string)
	if runErr != nil {
		if value, ok := record["execution"]; ok {
			data, _ := json.Marshal(value)
			var execution core.ExecutionResult
			if json.Unmarshal(data, &execution) == nil {
				if len(execution.Stdout) > 16384 {
					execution.Stdout = execution.Stdout[:16384]
					execution.StdoutTruncated = true
				}
				if len(execution.Stderr) > 16384 {
					execution.Stderr = execution.Stderr[:16384]
					execution.StderrTruncated = true
				}
				result.Execution = &execution
			}
		}
		if record["state"] == "failed" {
			result.State = "failed"
			return result, fmt.Errorf("packer %s failed; inspect receipt %s: %w", result.Stage, directory, core.ErrRuntimeUnavailable)
		}
		return result, fmt.Errorf("packer %s failed; inspect receipt %s: %w", result.Stage, directory, core.ErrRecoveryRequired)
	}
	return importArtifact(ctx, directory, id, request, client, record)
}

func importArtifact(ctx context.Context, directory, id string, request basebuild.ImportRequest, client Importer, record map[string]any) (result basebuild.Result, err error) {
	name := request.Name
	result = basebuild.Result{Base: core.BaseInfo{Name: name}, Builder: "haco-packer-" + id, State: "recovery-required", Stage: "import"}
	if err := transportDirectory(directory); err != nil {
		return result, err
	}
	digest, _ := record["sha256"].(string)
	number, ok := record["size"].(json.Number)
	size, sizeErr := number.Int64()
	arch, _ := record["architecture"].(string)
	if record["id"] != id || record["base_name"] != string(name) || record["state"] != "artifact-ready" || !ok || size <= 0 || size > request.ArchiveLimit() || sizeErr != nil || request.Validate() != nil {
		return result, core.ErrRecoveryRequired
	}
	fd, e := syscall.Open(filepath.Join(directory, "export", "image.tar"), syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if e != nil {
		return result, core.ErrRecoveryRequired
	}
	file := os.NewFile(uintptr(fd), "Packer image artifact")
	defer func() { _ = file.Close() }()
	info, e := file.Stat()
	if e != nil || !info.Mode().IsRegular() || info.Size() != int64(size) || info.Sys().(*syscall.Stat_t).Nlink != 1 {
		return result, core.ErrRecoveryRequired
	}
	request.Artifact = &basebuild.Artifact{ID: id, SHA256: digest, Size: size, Architecture: arch}
	if request.Validate() != nil {
		return result, core.ErrRecoveryRequired
	}
	record["state"] = "import-pending"
	record["import_builder"] = "build-" + id
	if e = saveReceipt(directory, record); e != nil {
		return result, core.ErrRecoveryRequired
	}
	imported, importErr := client.ImportBase(ctx, io.NewSectionReader(file, 0, info.Size()), request)
	record["import_result"] = imported
	// Unknown responses retain both the exact import builder identity and artifact.
	if importErr != nil || imported.State != "ready" || imported.Base.Name != name || imported.Base.Revision == "" {
		record["state"] = "recovery-required"
		persistErr := saveReceipt(directory, record)
		if imported.Base.Name == name {
			result.Base = imported.Base
		}
		return result, fmt.Errorf("base import unconfirmed; inspect receipt %s: %w", directory, errors.Join(core.ErrRecoveryRequired, importErr, persistErr))
	}
	result.Base = imported.Base
	record["state"] = "imported"
	if e = saveReceipt(directory, record); e != nil {
		return result, core.ErrRecoveryRequired
	}
	if e = file.Close(); e != nil {
		return result, core.ErrRecoveryRequired
	}
	if e = transportDirectory(directory); e != nil {
		result.Base = imported.Base
		return result, e
	}
	// The directory was created for this exact random operation. Remove only the
	// two remaining fixed files after confirmed import. rmdir refuses extra data.
	for _, path := range []string{"export/image.tar", "export", "receipt.json", ""} {
		if e = os.Remove(filepath.Join(directory, path)); e != nil {
			result.Base = imported.Base
			persistErr := saveReceipt(directory, record)
			return result, fmt.Errorf("base imported; transport cleanup required for %s: %w", id, errors.Join(core.ErrRecoveryRequired, e, persistErr))
		}
	}
	imported.Stage = ""
	return imported, nil
}

func readReceipt(directory string) (map[string]any, error) {
	fd, err := syscall.Open(filepath.Join(directory, "receipt.json"), syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), "Packer receipt")
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 262144 {
		return nil, core.ErrRecoveryRequired
	}
	var value map[string]any
	decoder := json.NewDecoder(io.LimitReader(f, 262145))
	decoder.UseNumber()
	err = decoder.Decode(&value)
	return value, err
}
func saveReceipt(directory string, value map[string]any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	temporary := filepath.Join(directory, "receipt.next")
	f, err := os.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(data)
	syncErr := f.Sync()
	closeErr := f.Close()
	if err = errors.Join(writeErr, syncErr, closeErr); err != nil {
		return err
	}
	if err = os.Rename(temporary, filepath.Join(directory, "receipt.json")); err != nil {
		return err
	}
	dir, err := os.Open(directory)
	if err != nil {
		return err
	}
	syncErr = dir.Sync()
	return errors.Join(syncErr, dir.Close())
}

func transportDirectory(directory string) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return core.ErrRecoveryRequired
	}
	if len(entries) != 2 || entries[0].Name() != "export" || entries[1].Name() != "receipt.json" {
		return core.ErrRecoveryRequired
	}
	exported, err := os.ReadDir(filepath.Join(directory, "export"))
	if err != nil || len(exported) != 1 || exported[0].Name() != "image.tar" {
		return core.ErrRecoveryRequired
	}
	return nil
}
