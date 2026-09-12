package incus

import (
	"context"
	"encoding/json"
	"path"
	"regexp"
	"strconv"
	"strings"
)

var diagnosticPoolName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,62}$`)
var diagnosticLoopDevice = regexp.MustCompile(`^/dev/loop[0-9]+$`)

type backingIdentity struct {
	major, minor, inode, mode uint64
}

type diagnosticLoop struct {
	Name      string  `json:"name"`
	Inode     *uint64 `json:"back-ino"`
	Device    string  `json:"back-maj:min"`
	Offset    *uint64 `json:"offset"`
	SizeLimit *uint64 `json:"sizelimit"`
}

// inspectLiveStorage observes only the Incus-owned image association and pool
// mount. It never creates a device or mounts, remounts, or detaches a filesystem.
// Source comes from the trusted local Incus inventory, never an RPC parameter.
// The identity checks also distinguish equal path strings in different WSL
// distributions sharing a kernel. A filename-only loop match is insufficient.
func (r *Runtime) inspectLiveStorage(ctx context.Context, pool, source string) (matches, known bool) {
	if !diagnosticPoolName.MatchString(pool) || len(source) > 4096 || !path.IsAbs(source) || path.Clean(source) != source ||
		!strings.HasSuffix(source, "/disks/"+pool+".img") {
		return false, false
	}
	for _, c := range source {
		if c < 32 || c == 127 {
			return false, false
		}
	}
	read := func(name string, args ...string) (string, bool) {
		if ctx.Err() != nil {
			return "", false
		}
		result, err := r.runner.Run(ctx, name, args...)
		return result.Stdout, err == nil && result.ExitCode == 0 && !result.StdoutTruncated && ctx.Err() == nil
	}
	statImage := func() (backingIdentity, bool) {
		// No dereference: the backing image itself must be a regular file.
		raw, ok := read("stat", "--printf=%Hd:%Ld:%i:%f", "--", source)
		if !ok {
			return backingIdentity{}, false
		}
		return parseBackingIdentity(raw)
	}
	image, ok := statImage()
	if !ok {
		return false, false
	}
	loopImage := func() (diagnosticLoop, bool) {
		raw, ok := read("losetup", "--list", "--json", "--associated", source, "--output", "NAME,BACK-INO,BACK-MAJ:MIN,OFFSET,SIZELIMIT")
		var listing struct {
			Devices []diagnosticLoop `json:"loopdevices"`
		}
		if !ok || json.Unmarshal([]byte(raw), &listing) != nil || len(listing.Devices) != 1 {
			return diagnosticLoop{}, false
		}
		device := listing.Devices[0]
		expected := strconv.FormatUint(image.major, 10) + ":" + strconv.FormatUint(image.minor, 10)
		valid := diagnosticLoopDevice.MatchString(device.Name) && device.Inode != nil && *device.Inode == image.inode &&
			device.Device == expected && device.Offset != nil && *device.Offset == 0 && device.SizeLimit != nil && *device.SizeLimit == 0
		return device, valid
	}
	device, ok := loopImage()
	if !ok {
		return false, false
	}
	mountpoint := path.Join(path.Dir(path.Dir(source)), "storage-pools", pool)
	raw, ok := read("findmnt", "--kernel", "--list", "--json", "--nofsroot", "--output", "SOURCE,FSTYPE,OPTIONS,FSROOT", "--mountpoint", mountpoint)
	var listing struct {
		Filesystems []struct {
			Source  string `json:"source"`
			Type    string `json:"fstype"`
			Options string `json:"options"`
			Root    string `json:"fsroot"`
		} `json:"filesystems"`
	}
	if !ok || json.Unmarshal([]byte(raw), &listing) != nil || len(listing.Filesystems) != 1 {
		return false, false
	}
	mount := listing.Filesystems[0]
	if mount.Source != device.Name || mount.Type != "btrfs" || mount.Root != "/" {
		return false, false
	}
	// Detect observed replacement/detachment while collecting the snapshot.
	// This is a read-only observation, not a lease or a mutation precondition.
	lastImage, ok := statImage()
	if !ok || lastImage != image {
		return false, false
	}
	lastDevice, ok := loopImage()
	if !ok || lastDevice.Name != device.Name {
		return false, false
	}
	return liveMountPolicy(mount.Options)
}

func parseBackingIdentity(raw string) (backingIdentity, bool) {
	parts := strings.Split(raw, ":")
	if len(parts) != 4 {
		return backingIdentity{}, false
	}
	var values [4]uint64
	for i, part := range parts {
		base := 10
		if i == 3 {
			base = 16
		}
		var err error
		values[i], err = strconv.ParseUint(part, base, 64)
		if err != nil {
			return backingIdentity{}, false
		}
	}
	identity := backingIdentity{values[0], values[1], values[2], values[3]}
	return identity, identity.inode != 0 && identity.mode&0170000 == 0100000
}

func liveMountPolicy(options string) (matches, known bool) {
	if len(options) == 0 || len(options) > 16384 {
		return false, false
	}
	for _, c := range options {
		if c < 33 || c > 126 {
			return false, false
		}
	}
	values := map[string]string{}
	for _, option := range strings.Split(options, ",") {
		key, value, _ := strings.Cut(option, "=")
		if key == "" {
			return false, false
		}
		if _, exists := values[key]; exists {
			return false, false
		}
		values[key] = value
	}
	_, rw := values["rw"]
	_, noatime := values["noatime"]
	_, compressed := values["compress"]
	matches = rw && values["rw"] == "" && noatime && values["noatime"] == "" && compressed && values["compress"] == "zstd:3"
	for _, key := range []string{"ro", "atime", "relatime", "strictatime", "discard", "autodefrag", "compress-force"} {
		if _, present := values[key]; present {
			matches = false
		}
	}
	// findmnt may omit negative/default nodiscard; positive discard is rejected.
	return matches, true
}
