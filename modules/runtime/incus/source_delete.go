package incus

import (
	"context"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/modules/standard/gitrepo"
	"reflect"
)

// sourceDevice proves the specific Host mount; it never accepts guest output.
func (b *RepositoryBackend) sourceDevice(ctx context.Context, o gitrepo.Object) (bool, error) {
	pool, name, err := volumeRef(o)
	if err != nil || o.Kind != "repo" {
		return false, core.ErrInvalidArgument
	}
	out, err := b.Runtime.runner.Run(ctx, "incus", "query", "/1.0/instances/"+trustedHostName+"?project="+b.Runtime.project)
	if err != nil || out.ExitCode != 0 || out.StdoutTruncated {
		return false, core.ErrRuntimeUnavailable
	}
	var host struct {
		Name, Type      string
		Config          map[string]string
		Devices         map[string]map[string]string
		ExpandedConfig  map[string]string            `json:"expanded_config"`
		ExpandedDevices map[string]map[string]string `json:"expanded_devices"`
	}
	if json.Unmarshal([]byte(out.Stdout), &host) != nil || host.Name != trustedHostName || host.Type != "container" || host.Config[trustedHostRoleKey] != trustedHostRoleValue || host.Devices == nil || host.ExpandedDevices == nil {
		return false, core.ErrIncompatibleState
	}
	device := "haco-repo-" + o.ID
	expected := map[string]string{"type": "disk", "pool": pool, "source": name, "path": gitrepo.RepositoryRoot + "/" + o.ID}
	actual, present := host.Devices[device]
	if present && (!reflect.DeepEqual(actual, expected) || !reflect.DeepEqual(host.ExpandedDevices[device], expected)) {
		return false, core.ErrCapabilityStale
	}
	for key, d := range host.ExpandedDevices {
		if key == device && !present {
			return false, core.ErrCapabilityStale
		}
		if key != device && d["pool"] == pool && d["source"] == name {
			return false, core.ErrStorageBusy
		}
	}
	return present, nil
}
func (b *RepositoryBackend) CheckSourceDeletion(ctx context.Context, o gitrepo.Object) error {
	if o.Kind != "repo" {
		return core.ErrInvalidArgument
	}
	present, err := b.sourceDevice(ctx, o)
	if err != nil {
		return err
	}
	allowed := ""
	if present {
		allowed = "/1.0/instances/" + trustedHostName + "?project=" + b.Runtime.project
	}
	v, err := b.managedVolumeForDeletion(ctx, o, allowed)
	if err != nil {
		return err
	}
	if v == nil {
		if present {
			return core.ErrIncompatibleState
		}
		return nil
	}
	pool, name, err := volumeRef(o)
	if err != nil {
		return err
	}
	return b.Runtime.checkVolumeSavedObjects(ctx, pool, name, v.Config)
}
func (b *RepositoryBackend) DeleteSourceVolume(ctx context.Context, o gitrepo.Object) error {
	unlock, err := lockHostOperation(ctx, b.Runtime.project)
	if err != nil {
		return err
	}
	defer unlock()
	if err := b.Runtime.rejectPendingHostCopy(ctx); err != nil {
		return err
	}
	if err := b.CheckSourceDeletion(ctx, o); err != nil {
		return err
	}
	present, err := b.sourceDevice(ctx, o)
	if err != nil {
		return err
	}
	if present {
		out, err := b.Runtime.runner.Run(ctx, "incus", "config", "device", "remove", trustedHostName, "haco-repo-"+o.ID, "--project", b.Runtime.project)
		if err != nil || out.ExitCode != 0 {
			return core.ErrRecoveryRequired
		}
	}
	if present, err := b.sourceDevice(ctx, o); err != nil || present {
		return core.ErrRecoveryRequired
	}
	return b.deleteManagedVolume(ctx, o)
}
