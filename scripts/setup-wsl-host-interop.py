#!/usr/bin/env python3
"""Opt in to Windows drive/EXE access for the owned trusted haco-host only.

Run as root on the WSL Physical Host after `haco setup`. No profiles or
Environment devices are modified. Windows continues to enforce its user's ACLs.
"""
import json
import os
from pathlib import Path
import re
import shlex
import sys
import subprocess
import tempfile

PATH_RECORD = Path('/etc/hacocoon/windows-path.json')
GUEST_LINUX_PATH = '/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin'


def windows_paths(value, drives):
    result = []
    for entry in value.split(':'):
        if (entry not in result and entry.startswith('/') and
                not any(ord(c) < 32 or ord(c) == 127 for c in entry) and
                '..' not in entry.split('/') and
                any(entry == drive or entry.startswith(drive + '/') for drive in drives)):
            result.append(entry)
    return result


def path_record(drives, capture=False):
    if capture:
        paths = windows_paths(os.environ.get('PATH', ''), drives)
        if not paths:
            raise ValueError('no WSL-converted Windows PATH; enable appendWindowsPath and rerun Windows installer')
        parent = PATH_RECORD.parent
        parent.mkdir(mode=0o755, exist_ok=True)
        if parent.is_symlink() or parent.stat().st_uid != 0 or parent.stat().st_mode & 0o022:
            raise ValueError('unsafe Windows PATH record directory')
        with tempfile.NamedTemporaryFile(mode='w', dir=parent, delete=False) as stream:
            json.dump(paths, stream)
            stream.flush()
            os.fsync(stream.fileno())
            temporary = stream.name
        os.replace(temporary, PATH_RECORD)
        return paths
    if PATH_RECORD.is_symlink() or PATH_RECORD.stat().st_uid != 0 or PATH_RECORD.stat().st_mode & 0o022:
        raise ValueError('unsafe Windows PATH record; rerun Windows installer')
    paths = json.loads(PATH_RECORD.read_text())
    if not isinstance(paths, list) or not all(isinstance(p, str) and ':' not in p for p in paths):
        raise ValueError('invalid Windows PATH record')
    return windows_paths(':'.join(paths), drives)


def drive_mounts(mounts):
    """Only actual WSL drive roots, never an arbitrary /mnt directory."""
    result = []
    for entry in mounts:
        target = entry.get("target", "")
        if not re.fullmatch(r"/mnt/[a-z]", target):
            continue
        kind, options = entry.get("fstype"), entry.get("options", "")
        if kind == "drvfs" or (kind == "9p" and "aname=drvfs;" in options):
            result.append(target)
    return sorted(set(result))


def desired_devices(drives):
    devices = {
        "haco-wsl-init": {"type": "disk", "source": "/init", "path": "/init", "readonly": "true"},
        "haco-wsl-interop": {"type": "disk", "source": "/run/WSL", "path": "/var/lib/hacocoon-wsl", "readonly": "true"},
    }
    for drive in drives:
        devices["haco-wsl-drive-" + drive[-1]] = {"type": "disk", "source": drive, "path": drive}
    return devices


def plan(config, devices):
    if config.get("config", {}).get("user.hacocoon.role") != "trusted-host":
        raise ValueError("refusing an unowned haco-host")
    if config.get("profiles"):
        raise ValueError("haco-host must use explicit devices without profiles; run haco setup")
    current = config.get("devices", {})
    for name, desired in devices.items():
        if name in current and current[name] != desired:
            raise ValueError("incompatible device: " + name)
        for other, present in current.items():
            if other != name and present.get("path") == desired["path"]:
                raise ValueError("mount target already used: " + desired["path"])
    current_interop = config.get("config", {}).get("environment.WSL_INTEROP", "")
    if current_interop and not re.fullmatch(r"/var/lib/hacocoon-wsl/[0-9]+_interop", current_interop):
        raise ValueError("incompatible WSL_INTEROP setting")
    return [name for name in devices if name not in current]


def main():
    if os.geteuid() != 0 or "microsoft" not in Path("/proc/sys/kernel/osrelease").read_text().lower():
        raise ValueError("run as root on the WSL Physical Host")
    if not Path("/init").is_file() or not Path("/run/WSL/1_interop").is_socket():
        raise ValueError("WSL interop is unavailable; enable Windows interop and enter WSL again")
    mounts = json.loads(subprocess.check_output(["findmnt", "--json", "--list", "-o", "TARGET,FSTYPE,OPTIONS"]))["filesystems"]
    drives = drive_mounts(mounts)
    if not drives:
        raise ValueError("no mounted Windows drives found under /mnt/<letter>")
    for drive in drives:
        if Path(drive).is_symlink() or str(Path(drive).resolve()) != drive:
            raise ValueError("refusing a redirected drive root")
    if sys.argv[1:] == ['--capture-path']:
        path_record(drives, capture=True)
        print('Recorded only WSL-converted Windows PATH entries')
        return
    if sys.argv[1:]:
        raise ValueError('usage: setup-wsl-host-interop.py [--capture-path]')
    paths = path_record(drives)
    incus = ["incus", "--project", "hacocoon"]
    inspect = ["incus", "query", "/1.0/instances/haco-host?project=hacocoon"]
    config = json.loads(subprocess.check_output(inspect))
    devices = desired_devices(drives)
    missing = plan(config, devices)
    for name in missing:
        device = devices[name]
        subprocess.run(incus + ["config", "device", "add", "haco-host", name, device["type"]] + [k + "=" + v for k, v in device.items() if k != "type"], check=True)
    # Keep WSL's stable init socket path rather than persisting a session PID.
    subprocess.run(incus + ["config", "set", "haco-host", "environment.WSL_INTEROP=/var/lib/hacocoon-wsl/1_interop"], check=True)
    subprocess.run(incus + ["config", "set", "haco-host", "environment.PATH=" + GUEST_LINUX_PATH + ':' + ':'.join(paths)], check=True)
    profile = '# Hacocoon managed Windows PATH; WSL already converted these entries.\n'
    profile += 'export WSL_INTEROP=/var/lib/hacocoon-wsl/1_interop\n'
    profile += 'export PATH="$PATH":' + shlex.quote(':'.join(paths)) + '\n'
    subprocess.run(incus + ['exec', 'haco-host', '--disable-stdin=false', '--', '/bin/sh', '-c',
                   'umask 022; cat > /etc/profile.d/hacocoon-windows.sh'], input=profile, text=True, check=True)
    verified = json.loads(subprocess.check_output(inspect))
    if plan(verified, devices):
        raise ValueError("trusted Host interop devices did not converge")
    print("Trusted haco-host Windows access enabled: " + ", ".join(drives))
    print("Open a new haco-host shell; existing WSLInterop supports direct tool.exe execution.")


if __name__ == "__main__":
    try:
        main()
    except (ValueError, OSError, subprocess.CalledProcessError) as error:
        raise SystemExit("WSL Host setup: " + str(error))
