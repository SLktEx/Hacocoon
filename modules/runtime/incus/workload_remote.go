package incus

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const (
	defaultWorkloadOCIRemote = "oci-docker"
	defaultWorkloadOCIURL    = "https://docker.io"
)

func (r *Runtime) ensureOCIImageRemote(ctx context.Context, image string) error {
	remote, err := workloadImageRemote(image)
	if err != nil {
		return err
	}
	protocol, exists, err := r.ociRemoteProtocol(ctx, remote)
	if err != nil {
		return err
	}
	if exists {
		if protocol != "oci" {
			return fmt.Errorf("Incus remote %q uses protocol %q instead of OCI: %w", remote, protocol, core.ErrIncompatibleState)
		}
		return nil
	}
	if remote != defaultWorkloadOCIRemote {
		return fmt.Errorf("Incus OCI remote %q is not configured: %w", remote, core.ErrNotFound)
	}

	if _, addErr := r.runner.Run(ctx, "incus", "remote", "add", defaultWorkloadOCIRemote, defaultWorkloadOCIURL, "--protocol=oci"); addErr != nil {
		// Another reconciler may have created it. Only accept that race if the
		// final remote exists with exactly the OCI protocol.
		protocol, exists, inspectErr := r.ociRemoteProtocol(ctx, remote)
		if inspectErr != nil || !exists || protocol != "oci" {
			return errors.Join(fmt.Errorf("add Docker Hub OCI remote: %w", addErr), inspectErr)
		}
		return nil
	}
	protocol, exists, err = r.ociRemoteProtocol(ctx, remote)
	if err != nil {
		return fmt.Errorf("verify Docker Hub OCI remote: %w", err)
	}
	if !exists || protocol != "oci" {
		return fmt.Errorf("Docker Hub OCI remote did not converge: %w", core.ErrIncompatibleState)
	}
	return nil
}

func workloadImageRemote(image string) (string, error) {
	if err := validateWorkloadImage(image); err != nil {
		return "", err
	}
	cut := strings.IndexByte(image, ':')
	if cut <= 0 {
		return "", fmt.Errorf("OCI workload image %q must include an Incus remote: %w", image, core.ErrInvalidArgument)
	}
	remote := image[:cut]
	if strings.ContainsAny(remote, "/ \t") {
		return "", fmt.Errorf("invalid Incus OCI remote in %q: %w", image, core.ErrInvalidArgument)
	}
	return remote, nil
}

func (r *Runtime) ociRemoteProtocol(ctx context.Context, name string) (string, bool, error) {
	result, err := r.runner.Run(ctx, "incus", "remote", "list", "--format", "csv,noheader", "--columns", "np")
	if err != nil {
		return "", false, fmt.Errorf("list Incus remotes: %w", err)
	}
	reader := csv.NewReader(strings.NewReader(result.Stdout))
	var protocol string
	found := false
	for {
		record, readErr := reader.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil || len(record) != 2 {
			return "", false, fmt.Errorf("decode Incus remote inventory: %w", core.ErrIncompatibleState)
		}
		if record[0] != name {
			continue
		}
		if found {
			return "", false, fmt.Errorf("Incus remote inventory repeats %q: %w", name, core.ErrIncompatibleState)
		}
		found = true
		protocol = strings.TrimSpace(record[1])
	}
	return protocol, found, nil
}
