#!/usr/bin/env python3
"""Reconcile Windows drive/EXE access for the owned trusted haco-host only.

Installed by the Windows installer and invoked by controller-owned setup. No profiles or
Environment devices are modified. Windows continues to enforce its user's ACLs.
"""
import json
import hashlib
import inspect
import os
from pathlib import Path
import re
import shlex
import stat
import sys
import subprocess
import tempfile
import uuid

PATH_RECORD = Path('/etc/hacocoon/windows-path.json')
DISTRIBUTION_RECORD = Path('/etc/hacocoon/windows-distribution.json')
REGISTRATION_RECORD = Path('/etc/hacocoon/windows-registration.json')
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


def distribution_record(capture=False):
    if capture:
        name = os.environ.get('WSL_DISTRO_NAME', '')
        if not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9_.-]{0,63}', name):
            raise ValueError('invalid WSL distribution identity; rerun Windows installer')
        parent = DISTRIBUTION_RECORD.parent
        if parent.is_symlink() or parent.stat().st_uid != 0 or parent.stat().st_mode & 0o022:
            raise ValueError('unsafe Windows distribution directory')
        if DISTRIBUTION_RECORD.exists() or DISTRIBUTION_RECORD.is_symlink():
            if distribution_record().lower() != name.lower():
                raise ValueError('Windows distribution identity mismatch')
        temporary = None
        try:
            with tempfile.NamedTemporaryFile(mode='w', dir=parent, delete=False) as stream:
                temporary = stream.name
                json.dump(name, stream)
                stream.flush()
                os.fsync(stream.fileno())
            os.replace(temporary, DISTRIBUTION_RECORD)
        finally:
            if temporary and os.path.exists(temporary):
                os.unlink(temporary)
        return name
    info = DISTRIBUTION_RECORD.lstat()
    if not stat.S_ISREG(info.st_mode) or info.st_uid != 0 or info.st_mode & 0o022 or info.st_size > 256:
        raise ValueError('unsafe Windows distribution record; rerun Windows installer')
    name = json.loads(DISTRIBUTION_RECORD.read_text())
    if not isinstance(name, str) or not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9_.-]{0,63}', name):
        raise ValueError('invalid Windows distribution record')
    return name



def canonical_registration_id(value):
    if not isinstance(value, str):
        raise ValueError('invalid Windows registration GUID')
    try:
        parsed = uuid.UUID(value)
    except (ValueError, AttributeError):
        raise ValueError('invalid Windows registration GUID') from None
    if parsed.int == 0 or value != '{' + str(parsed) + '}':
        raise ValueError('require a canonical nonzero Windows registration GUID')
    return value


def registration_record(value=None, capture=False):
    """Installer-only binding component, not permission to stop or compact WSL.

    Keep the existing name record unchanged. No replacement/migration/recovery is
    inferred from a caller-supplied name or from an unreadable binding.
    """
    import fcntl
    if capture:
        value = canonical_registration_id(value)
    if os.geteuid() != 0:
        raise ValueError('registration access requires Physical Host root')
    parent = REGISTRATION_RECORD.parent
    if not parent.is_absolute():
        raise ValueError('registration directory must be absolute')
    directory = os.open('/', os.O_RDONLY | os.O_DIRECTORY | os.O_CLOEXEC)
    try:
        # Walk using held directories, never follow an ancestor symlink.
        for part in parent.parts[1:]:
            child = os.open(part, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC, dir_fd=directory)
            os.close(directory)
            directory = child
        info = os.fstat(directory)
        if info.st_uid != 0 or info.st_mode & 0o022:
            raise ValueError('unsafe registration directory')
        fcntl.flock(directory, fcntl.LOCK_EX)
        name = REGISTRATION_RECORD.name
        try:
            fd = os.open(name, os.O_RDONLY | os.O_NONBLOCK | os.O_NOFOLLOW | os.O_CLOEXEC, dir_fd=directory)
        except FileNotFoundError:
            fd = None
        if fd is not None:
            with os.fdopen(fd, 'rb') as stream:
                info = os.fstat(stream.fileno())
                if not stat.S_ISREG(info.st_mode) or info.st_uid != 0 or info.st_mode & 0o077 or info.st_nlink != 1 or info.st_size > 4096:
                    raise ValueError('unsafe registration record')
                raw = stream.read(4097)
            try:
                record = json.loads(raw)
                if (set(record) != {'schema_version', 'registration_id', 'installation_id'} or
                        type(record['schema_version']) is not int or record['schema_version'] != 1 or
                        (canonical_registration_id(record['registration_id']) != value and value is not None) or
                        str(uuid.UUID(record['installation_id'])) != record['installation_id'] or
                        uuid.UUID(record['installation_id']).int == 0 or
                        raw != (json.dumps(record, sort_keys=True, separators=(',', ':')) + '\n').encode()):
                    raise ValueError('registration binding mismatch or unsupported format')
            except (KeyError, TypeError, AttributeError, json.JSONDecodeError):
                raise ValueError('invalid registration record; retain it for review') from None
            return record
        if not capture:
            raise FileNotFoundError('managed Windows registration is not enrolled')
        record = {'schema_version': 1, 'registration_id': value, 'installation_id': str(uuid.uuid4())}
        raw = (json.dumps(record, sort_keys=True, separators=(',', ':')) + '\n').encode()
        temporary = '.windows-registration-' + uuid.uuid4().hex
        fd = os.open(temporary, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW | os.O_CLOEXEC, 0o600, dir_fd=directory)
        try:
            with os.fdopen(fd, 'wb') as stream:
                stream.write(raw)
                stream.flush()
                os.fsync(stream.fileno())
            # Publish without replacing a name even if another root process races.
            os.link(temporary, name, src_dir_fd=directory, dst_dir_fd=directory, follow_symlinks=False)
        finally:
            os.unlink(temporary, dir_fd=directory)
        os.fsync(directory)
        return record
    finally:
        os.close(directory)


def capture_registration(value):
    return registration_record(value, capture=True)


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


def plan(config, devices, distribution=None):
    if config.get("config", {}).get("user.hacocoon.role") != "trusted-host":
        raise ValueError("refusing an unowned haco-host")
    if config.get("profiles"):
        raise ValueError("haco-host must use explicit devices without profiles; run haco setup")
    current_distribution = config.get("config", {}).get("environment.WSL_DISTRO_NAME", "")
    if distribution and current_distribution and current_distribution.lower() != distribution.lower():
        raise ValueError("incompatible WSL distribution identity")
    current = config.get("devices", {})
    for name, desired in devices.items():
        if name in current and current[name] != desired:
            raise ValueError("incompatible device: " + name)
        for other, present in current.items():
            if other != name and present.get("path") == desired["path"]:
                raise ValueError("mount target already used: " + desired["path"])
    current_interop = config.get("config", {}).get("environment.WSL_INTEROP", "")
    if current_interop and not re.fullmatch(r"/run/WSL/[0-9]+_interop", current_interop):
        raise ValueError("incompatible WSL_INTEROP setting")
    return [name for name in devices if name not in current]


class NativeBinfmtIncompatible(ValueError):
    """Fixed setup protocol exit 42; contains no guest output."""


def ensure_native_binfmt(directory=Path('/proc/sys/fs/binfmt_misc'),
                         generated=Path('/run/systemd/generator/systemd-binfmt.service.d/override.conf'),
                         run=subprocess.run):
    # A healthy native registration is never rewritten. Respect explicit disable
    # or foreign interpreter settings rather than silently replacing them.
    if (directory / 'status').read_text().strip() != 'enabled':
        raise ValueError('WSL native binfmt is disabled; enable WSL interop explicitly')

    def check():
        entries = list(directory.glob('WSLInterop*'))
        for entry in entries:
            lines = set(entry.read_text().strip().splitlines())
            expected = {'enabled', 'interpreter /init', 'offset 0', 'magic 4d5a'}
            # Allow only known native WSL variants; every other field stays exact.
            if lines not in (expected | {'flags: P'}, expected | {'flags: PF'}):
                raise NativeBinfmtIncompatible('incompatible or disabled native WSL binfmt registration')
        return bool(entries)

    if check():
        return
    # Let WSL's own generated systemd integration restore its native handler.
    # Never write a Hacocoon registration string into binfmt_misc/register.
    info = generated.lstat()
    if not stat.S_ISREG(info.st_mode) or info.st_uid != 0 or info.st_mode & 0o022:
        raise ValueError('untrusted WSL systemd binfmt integration')
    if ':WSLInterop:M::MZ::/init:P' not in generated.read_text():
        raise ValueError('WSL native binfmt integration is unavailable')
    run(['systemctl', 'restart', 'systemd-binfmt.service'], check=True)
    if not check():
        raise ValueError('WSL native binfmt registration was not restored')


# No launcher or socket relay: this only restores WSL's native pathname after
# the guest recreates /run. Refuse a pre-existing path owned by another setup.
SOCKET_LAYOUT = r"""set -eu
if test -e /run/WSL || test -L /run/WSL; then
    if ! test -L /run/WSL || test "$(readlink /run/WSL)" != /var/lib/hacocoon-wsl; then exit 1; fi
fi
mkdir -p /etc/tmpfiles.d
printf '%s\n' 'L /run/WSL - - - - /var/lib/hacocoon-wsl' > /etc/tmpfiles.d/hacocoon-wsl.conf
systemd-tmpfiles --create /etc/tmpfiles.d/hacocoon-wsl.conf
test -S /run/WSL/1_interop
"""


def notification_executable_revision(path=Path('/usr/local/bin/haco-notify')):
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    with os.fdopen(fd, 'rb') as stream:
        info = os.fstat(stream.fileno())
        if not stat.S_ISREG(info.st_mode) or info.st_uid != 0 or info.st_nlink != 1 or info.st_mode & 0o022 or not 0 < info.st_size <= 128 * 1024 * 1024:
            raise ValueError('unsafe notification executable')
        digest = hashlib.sha256()
        remaining = info.st_size
        while remaining:
            chunk = stream.read(min(remaining, 1024 * 1024))
            if not chunk: raise ValueError('notification executable changed')
            digest.update(chunk)
            remaining -= len(chunk)
        final = os.fstat(stream.fileno())
        if stream.read(1) or final.st_size != info.st_size or final.st_mtime_ns != info.st_mtime_ns:
            raise ValueError('notification executable changed')
        return digest.hexdigest()


def notification_unit(paths, distribution, revision):
    if not re.fullmatch(r'[0-9a-f]{64}', revision):
        raise ValueError('invalid notification executable revision')
    if not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9_.-]{0,63}', distribution):
        raise ValueError('invalid notification distribution')
    def setting(value):
        if any(ord(c) < 32 or ord(c) == 127 for c in value):
            raise ValueError('invalid notification environment')
        return '"' + value.replace('\\', '\\\\').replace('"', '\\"').replace('%', '%%') + '"'
    environment = [
        'HOME=/root', 'HACO_CLIENT_MODE=controller', 'HACO_CONTROL_SOCKET=/var/lib/hacocoon-control.sock',
        'WSL_INTEROP=/run/WSL/1_interop', 'WSL_DISTRO_NAME=' + distribution,
        'PATH=' + GUEST_LINUX_PATH + ':' + ':'.join(paths),
    ]
    return ('# Hacocoon managed native notifications\n# Executable SHA256: ' + revision + '\n[Unit]\n'
            'Description=Hacocoon desktop notifications\nAfter=systemd-tmpfiles-setup.service\n'
            'StartLimitIntervalSec=60\nStartLimitBurst=5\n[Service]\nType=simple\n'
            'ExecStart=/usr/local/bin/haco-notify native --from-now\n'
            'User=root\nUMask=0077\n'
            'Environment=' + ' '.join(setting(value) for value in environment) + '\n'
            'Restart=on-failure\nRestartSec=5\nStandardOutput=null\nStandardError=journal\n'
            '[Install]\nWantedBy=multi-user.target\n')


def install_notification_unit(unit, enabled, directory=Path('/etc/systemd/system'), run=subprocess.run):
    marker = '# Hacocoon managed native notifications\n'
    if not unit.startswith(marker) or enabled is not None and not isinstance(enabled, bool):
        raise ValueError('invalid notification service request')
    info = directory.lstat()
    if not stat.S_ISDIR(info.st_mode) or info.st_uid != 0 or info.st_mode & 0o022:
        raise ValueError('unsafe notification service directory')
    target = directory / 'hacocoon-notify.service'
    exists = target.exists() or target.is_symlink()
    previous = None
    if exists:
        info = target.lstat()
        if not stat.S_ISREG(info.st_mode) or info.st_uid != 0 or info.st_nlink != 1 or info.st_mode & 0o022 or info.st_size > 32768:
            raise ValueError('unsafe existing notification service')
        previous = target.read_text()
        if not previous.startswith(marker):
            raise ValueError('notification service is owned by another configuration')
    if enabled is None:
        if not exists:
            return
        state = run(['systemctl', 'is-enabled', '--quiet', 'hacocoon-notify.service'], check=False)
        if state.returncode == 1:
            return
        if state.returncode != 0:
            raise subprocess.CalledProcessError(state.returncode, 'inspect notification service')
        enabled = True
    if not enabled:
        if exists:
            run(['systemctl', 'disable', '--now', 'hacocoon-notify.service'], check=True)
        return
    if previous == unit:
        active = run(['systemctl', 'is-active', '--quiet', 'hacocoon-notify.service'], check=False)
        if active.returncode == 0:
            run(['systemctl', 'enable', 'hacocoon-notify.service'], check=True)
            return
        if active.returncode != 3:
            raise subprocess.CalledProcessError(active.returncode, 'inspect notification activity')
    temporary = None
    try:
        with tempfile.NamedTemporaryFile(mode='w', dir=directory, delete=False) as stream:
            temporary = stream.name
            os.fchmod(stream.fileno(), 0o644)
            stream.write(unit)
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(temporary, target)
    finally:
        if temporary and os.path.exists(temporary):
            os.unlink(temporary)
    run(['systemctl', 'daemon-reload'], check=True)
    # A new/inactive unit can be garbage-collected between systemctl calls.
    # Only a retained failed unit has failure state to reset.
    failed = run(['systemctl', 'is-failed', '--quiet', 'hacocoon-notify.service'], check=False)
    if failed.returncode == 0:
        run(['systemctl', 'reset-failed', 'hacocoon-notify.service'], check=True)
    elif failed.returncode != 1:
        raise subprocess.CalledProcessError(failed.returncode, 'inspect notification failure state')
    run(['systemctl', 'enable', 'hacocoon-notify.service'], check=True)
    run(['systemctl', 'restart', 'hacocoon-notify.service'], check=True)
    run(['systemctl', 'is-active', '--quiet', 'hacocoon-notify.service'], check=True)


def configure_notifications(incus, paths, distribution, enabled):
    unit = notification_unit(paths, distribution, notification_executable_revision())
    program = ('from pathlib import Path\nimport os, stat, tempfile, subprocess\n' +
               inspect.getsource(install_notification_unit) +
               '\ninstall_notification_unit(' + repr(unit) + ', ' + repr(enabled) + ')\n')
    subprocess.run(incus + ['exec', 'haco-host', '--disable-stdin=false', '--', 'python3', '-I', '-'],
                   input=program, text=True, check=True)


def main():
    if os.geteuid() != 0 or "microsoft" not in Path("/proc/sys/kernel/osrelease").read_text().lower():
        raise ValueError("run as root on the WSL Physical Host")
    if not Path("/init").is_file() or not Path("/run/WSL/1_interop").is_socket():
        raise ValueError("WSL interop is unavailable; enable Windows interop and enter WSL again")
    if sys.argv[1:] == ['--read-registration']:
        print(json.dumps(registration_record(), sort_keys=True, separators=(',', ':')))
        return
    if len(sys.argv) == 3 and sys.argv[1] == '--capture-registration':
        capture_registration(sys.argv[2])
        print('Recorded managed WSL registration identity')
        return
    mounts = json.loads(subprocess.check_output(["findmnt", "--json", "--list", "-o", "TARGET,FSTYPE,OPTIONS"]))["filesystems"]
    drives = drive_mounts(mounts)
    if not drives:
        raise ValueError("no mounted Windows drives found under /mnt/<letter>")
    for drive in drives:
        if Path(drive).is_symlink() or str(Path(drive).resolve()) != drive:
            raise ValueError("refusing a redirected drive root")
    if sys.argv[1:] == ['--capture-path']:
        path_record(drives, capture=True)
        distribution_record(capture=True)
        print('Recorded only WSL-converted Windows PATH entries')
        return
    notification_mode = sys.argv[1:]
    if notification_mode not in ([], ['--notifications=on'], ['--notifications=off'], ['--notifications=refresh']):
        raise ValueError('invalid WSL setup mode')
    paths = path_record(drives)
    distribution = distribution_record()
    ensure_native_binfmt()
    incus = ["incus", "--project", "hacocoon"]
    inspect = ["incus", "query", "/1.0/instances/haco-host?project=hacocoon"]
    config = json.loads(subprocess.check_output(inspect))
    devices = desired_devices(drives)
    missing = plan(config, devices, distribution)
    for name in missing:
        device = devices[name]
        subprocess.run(incus + ["config", "device", "add", "haco-host", name, device["type"]] + [k + "=" + v for k, v in device.items() if k != "type"], check=True)
    # /run is replaced by guest systemd's tmpfs at boot. Keep the Incus disk
    # outside it and use standard tmpfiles to restore the native absolute path.
    subprocess.run(incus + ['exec', 'haco-host', '--disable-stdin=false', '--',
                           '/bin/sh', '-s'], input=SOCKET_LAYOUT, text=True, check=True)
    subprocess.run(incus + ["config", "set", "haco-host", "environment.WSL_INTEROP=/run/WSL/1_interop"], check=True)
    subprocess.run(incus + ["config", "set", "haco-host", "environment.PATH=" + GUEST_LINUX_PATH + ':' + ':'.join(paths)], check=True)
    subprocess.run(incus + ["config", "set", "haco-host", "environment.WSL_DISTRO_NAME=" + distribution], check=True)
    profile = '# Hacocoon managed Windows PATH; WSL already converted these entries.\n'
    profile += 'export WSL_DISTRO_NAME=' + shlex.quote(distribution) + '\n'
    profile += 'export WSL_INTEROP=/run/WSL/1_interop\n'
    profile += 'export PATH="$PATH":' + shlex.quote(':'.join(paths)) + '\n'
    subprocess.run(incus + ['exec', 'haco-host', '--disable-stdin=false', '--', '/bin/sh', '-c',
                   'umask 022; cat > /etc/profile.d/hacocoon-windows.sh'], input=profile, text=True, check=True)
    verified = json.loads(subprocess.check_output(inspect))
    if plan(verified, devices, distribution) or verified.get("config", {}).get("environment.WSL_DISTRO_NAME") != distribution:
        raise ValueError("trusted Host interop devices did not converge")
    if notification_mode:
        configure_notifications(incus, paths, distribution, None if notification_mode == ['--notifications=refresh'] else notification_mode == ['--notifications=on'])
    print("Trusted haco-host Windows access enabled: " + ", ".join(drives))
    print("Open a new haco-host shell; existing WSLInterop supports direct tool.exe execution.")


def cli_main():
    try:
        main()
    except NativeBinfmtIncompatible:
        raise SystemExit(42)
    except (ValueError, OSError, subprocess.CalledProcessError) as error:
        raise SystemExit("WSL Host setup: " + str(error))


if __name__ == "__main__":
    cli_main()
