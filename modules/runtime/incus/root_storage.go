package incus

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// ConfigureStorageProvider installs a lazy Host-side source for the storage
// attachment used by Hacocoon-owned Incus rootfs volumes. Configuration itself
// performs no loop attach, mount, or Incus storage mutation; the first rootfs
// operation resolves and ensures the selected pool.
func (r *Runtime) ConfigureStorageProvider(provider func(context.Context) (map[string]string, error)) error {
	if r == nil || provider == nil {
		return core.ErrInvalidArgument
	}
	if r.storage == nil {
		r.storage = &runtimeStorageState{}
	}
	r.storage.mu.Lock()
	defer r.storage.mu.Unlock()
	r.storage.provider = provider
	r.storage.rootPool = ""
	return nil
}

func (r *Runtime) setRootPool(pool string) {
	if r.storage == nil {
		r.storage = &runtimeStorageState{}
	}
	r.storage.mu.Lock()
	defer r.storage.mu.Unlock()
	r.storage.rootPool = pool
}

// defaultRootPool prefers the Hacocoon-managed pool selected by Prepare or by
// the lazy storage provider configured by the local composition. The Incus
// default-profile lookup is retained only for low-level callers that bypass the
// normal Hacocoon local composition.
func (r *Runtime) defaultRootPool(ctx context.Context) (string, error) {
	if r.storage != nil {
		r.storage.mu.Lock()
		if pool := strings.TrimSpace(r.storage.rootPool); pool != "" {
			r.storage.mu.Unlock()
			if _, err := r.runner.Run(ctx, "incus", "storage", "show", pool, "--project", r.project); err != nil {
				return "", fmt.Errorf("Hacocoon root storage pool %q is unavailable: %w", pool, err)
			}
			return pool, nil
		}
		provider := r.storage.provider
		if provider != nil {
			attachment, err := provider(ctx)
			if err != nil {
				r.storage.mu.Unlock()
				return "", fmt.Errorf("ensure Hacocoon root storage: %w", err)
			}
			pool, err := r.ensureStoragePool(ctx, attachment)
			if err != nil {
				r.storage.mu.Unlock()
				return "", fmt.Errorf("ensure Hacocoon Incus storage pool: %w", err)
			}
			if strings.TrimSpace(pool) == "" {
				r.storage.mu.Unlock()
				return "", fmt.Errorf("Hacocoon storage provider returned no incus_pool: %w", core.ErrIncompatibleState)
			}
			r.storage.rootPool = pool
			r.storage.mu.Unlock()
			return pool, nil
		}
		r.storage.mu.Unlock()
	}

	result, err := r.runner.Run(ctx, "incus", "profile", "show", "default", "--project", "default", "--format", "json")
	if err != nil {
		return "", err
	}
	var profile struct {
		Devices map[string]map[string]string `json:"devices"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &profile); err != nil {
		return "", fmt.Errorf("decode default profile: %w", err)
	}
	for _, device := range profile.Devices {
		if device["type"] == "disk" && device["path"] == "/" && device["pool"] != "" {
			return device["pool"], nil
		}
	}
	return "", fmt.Errorf("default profile has no root disk pool: %w", core.ErrUnsupported)
}

func (r *Runtime) ensureStoragePool(ctx context.Context, attachment map[string]string) (string, error) {
	if len(attachment) == 0 {
		return "", nil
	}
	pool := attachment["incus_pool"]
	// Creation belongs to the Incus-owned storage provider. Do not accept the
	// removed external driver/source attachment, even when the pool exists.
	if len(attachment) != 1 || pool == "" || strings.TrimSpace(pool) != pool || strings.HasPrefix(pool, "-") || strings.ContainsAny(pool, ":/\\\x00\r\n\t ") {
		return "", fmt.Errorf("storage attachment requires only a local incus_pool identity: %w", core.ErrInvalidArgument)
	}
	if _, err := r.runner.Run(ctx, "incus", "storage", "show", pool, "--project", r.project); err != nil {
		return "", fmt.Errorf("Incus-owned storage pool %q is unavailable: %w", pool, err)
	}
	return pool, nil
}
