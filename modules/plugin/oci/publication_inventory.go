package oci

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

// PublicationImage keeps the engine's local ID separate from a registry digest.
// In particular, locally built Docker images need not have a registry digest.
type PublicationImage struct {
	Reference string `json:"reference"`
	ID        string `json:"id"`
	Digest    string `json:"digest,omitempty"`
}

type PublicationInventory struct {
	Driver    Driver             `json:"driver"`
	Namespace string             `json:"namespace,omitempty"`
	Images    []PublicationImage `json:"images"`
	Revision  string             `json:"revision"`
}

// LocalImageSource must read the trusted logical Host, never an Environment or
// ambient Physical Host Docker context. Missing/broken tooling returns an error.
type LocalImageSource interface {
	LocalOCIImages(context.Context, string) (host.Result, error)
}

// ReadPublicationInventory prepares an immutable selection for the publisher.
// This is inventory only: it does not pull, export, publish or mutate a Store.
func ReadPublicationInventory(ctx context.Context, source LocalImageSource, driver Driver) (PublicationInventory, error) {
	if source == nil || (driver != DriverDocker && driver != DriverNerdctl) {
		return PublicationInventory{}, core.ErrInvalidArgument
	}
	result, err := source.LocalOCIImages(ctx, string(driver))
	if err != nil || result.ExitCode != 0 {
		if ctx.Err() != nil {
			return PublicationInventory{}, ctx.Err()
		}
		return PublicationInventory{}, fmt.Errorf("trusted Host image inventory failed: %w", core.ErrRuntimeUnavailable)
	}
	if result.StdoutTruncated || len(result.Stdout) > 4<<20 {
		return PublicationInventory{}, core.ErrIncompatibleState
	}
	inventory, err := parsePublicationInventory(result.Stdout, driver)
	if err != nil {
		return PublicationInventory{}, err
	}
	return inventory, nil
}

var publicationRepository = regexp.MustCompile(`^[a-z0-9][a-z0-9._:/-]{0,254}$`)
var publicationTag = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$`)

func parsePublicationInventory(output string, driver Driver) (PublicationInventory, error) {
	inventory := PublicationInventory{Driver: driver, Images: []PublicationImage{}}
	if driver == DriverNerdctl {
		inventory.Namespace = "default"
	}
	seen := map[string]PublicationImage{}
	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) != 4 || len(seen) >= 10000 {
			return PublicationInventory{}, core.ErrIncompatibleState
		}
		repository, tag, id, digest := parts[0], parts[1], parts[2], parts[3]
		if !strings.HasPrefix(id, "sha256:") {
			id = "sha256:" + id
		}
		if !digestPattern.MatchString(id) {
			return PublicationInventory{}, core.ErrIncompatibleState
		}
		if digest == "<none>" {
			digest = ""
		}
		if digest != "" && !digestPattern.MatchString(digest) {
			return PublicationInventory{}, core.ErrIncompatibleState
		}
		reference := ""
		if repository == "<none>" {
			if tag != "<none>" {
				return PublicationInventory{}, core.ErrIncompatibleState
			}
		} else {
			if !publicationRepository.MatchString(repository) {
				return PublicationInventory{}, core.ErrIncompatibleState
			}
			if tag == "<none>" {
				if digest == "" {
					return PublicationInventory{}, core.ErrIncompatibleState
				}
				reference = repository + "@" + digest
			} else {
				if !publicationTag.MatchString(tag) {
					return PublicationInventory{}, core.ErrIncompatibleState
				}
				reference = repository + ":" + tag
			}
		}
		image := PublicationImage{Reference: reference, ID: id, Digest: digest}
		key := reference
		if key == "" {
			key = id
		}
		if previous, exists := seen[key]; exists && previous != image {
			return PublicationInventory{}, core.ErrIncompatibleState
		}
		seen[key] = image
	}
	for _, image := range seen {
		inventory.Images = append(inventory.Images, image)
	}
	sort.Slice(inventory.Images, func(i, j int) bool {
		if inventory.Images[i].Reference != inventory.Images[j].Reference {
			return inventory.Images[i].Reference < inventory.Images[j].Reference
		}
		return inventory.Images[i].ID < inventory.Images[j].ID
	})
	encoded, err := json.Marshal(inventory)
	if err != nil {
		return PublicationInventory{}, err
	}
	sum := sha256.Sum256(encoded)
	inventory.Revision = "sha256:" + hex.EncodeToString(sum[:])
	return inventory, nil
}
