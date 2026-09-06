#!/usr/bin/python3
"""Retire Incus network/proxy PID records across proven PID-namespace boots.

Installed as an Incus ExecStartPre, not imported by Hacocoon Core.
The CLI has fixed root-owned paths. Tests construct Guard with a private tree.
"""
import contextlib
import fcntl
import json
import os
from pathlib import Path
import socket
import stat
import struct
import sys
import uuid


class Refused(Exception):
    pass


def namespace_identity():
    boot = str(uuid.UUID(Path('/proc/sys/kernel/random/boot_id').read_text().strip()))
    # comm may contain spaces and ')'; fields after its final ')' start at 3.
    fields = Path('/proc/1/stat').read_text().rsplit(')', 1)[1].split()
    started = int(fields[19])
    ns = os.stat('/proc/self/ns/pid')
    if started < 0:
        raise Refused('invalid PID namespace identity')
    return {'boot': boot, 'init_start': started, 'pidns': ns.st_ino}


def daemon_available(root):
    with socket.socket(socket.AF_UNIX) as conn:
        conn.settimeout(2)
        try:
            conn.connect(str(root / 'unix.socket'))
        except (FileNotFoundError, ConnectionRefusedError):
            return False
        pid, uid, _ = struct.unpack('3i', conn.getsockopt(socket.SOL_SOCKET, socket.SO_PEERCRED, 12))
        if uid != 0 or os.readlink(f'/proc/{pid}/exe') != '/usr/libexec/incus/incusd':
            raise Refused('unexpected Incus socket peer')
        if os.stat(f'/proc/{pid}/ns/pid').st_ino != os.stat('/proc/self/ns/pid').st_ino:
            raise Refused('unexpected Incus PID namespace')
        return True


class Guard:
    def __init__(self, root, identity, uid=0, available=daemon_available):
        self.root = Path(root)
        self.identity = identity
        self.uid = uid
        self.available = available

    def checked(self, fd, directory=False):
        info = os.fstat(fd)
        kind = stat.S_ISDIR if directory else stat.S_ISREG
        if not kind(info.st_mode) or info.st_uid != self.uid or info.st_mode & 0o022:
            raise Refused('unsafe provider metadata')
        if not directory and info.st_nlink != 1:
            raise Refused('linked provider metadata')
        return info

    @contextlib.contextmanager
    def directory(self, path, parent=None):
        fd = os.open(path, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=parent)
        try:
            self.checked(fd, True)
            yield fd
        finally:
            os.close(fd)

    def read_marker(self, fd):
        try:
            marker = os.open('namespace.json', os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=fd)
        except FileNotFoundError:
            return None
        with os.fdopen(marker) as stream:
            if self.checked(stream.fileno()).st_size > 1024:
                raise Refused('invalid namespace marker')
            data = json.load(stream)
        if (not isinstance(data, dict) or set(data) != {'version', 'namespace'}
                or type(data['version']) is not int or data['version'] != 1
                or not isinstance(data['namespace'], dict)
                or set(data['namespace']) != {'boot', 'init_start', 'pidns'}):
            raise Refused('invalid namespace marker')
        ns = data['namespace']
        if (str(uuid.UUID(ns['boot'])) != ns['boot']
                or type(ns['init_start']) is not int or ns['init_start'] < 0
                or type(ns['pidns']) is not int or ns['pidns'] <= 0):
            raise Refused('invalid namespace marker')
        return ns

    def write_marker(self, fd):
        name = '.namespace-' + uuid.uuid4().hex
        out = os.open(name, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600, dir_fd=fd)
        try:
            with os.fdopen(out, 'w') as stream:
                json.dump({'version': 1, 'namespace': self.identity}, stream)
                stream.flush()
                os.fsync(stream.fileno())
            os.replace(name, 'namespace.json', src_dir_fd=fd, dst_dir_fd=fd)
            os.fsync(fd)
        finally:
            with contextlib.suppress(FileNotFoundError):
                os.unlink(name, dir_fd=fd)

    def run(self, initialize=False, *, root_fd=None):
        # Walk every parent without following symlinks, including /var/lib.
        with contextlib.ExitStack() as stack:
            if root_fd is None:
                parent = None
                for part in (self.root.anchor, *self.root.parts[1:]):
                    parent = stack.enter_context(self.directory(part, parent))
                rootfd = parent
            else:
                # Component tests supply an already opened private fixture.
                # The root-only CLI never accepts a path or file descriptor.
                self.checked(root_fd, True)
                rootfd = root_fd
            with contextlib.suppress(FileExistsError):
                os.mkdir('.hacocoon-boot-guard', 0o700, dir_fd=rootfd)
            os.fsync(rootfd)
            statefd = stack.enter_context(self.directory('.hacocoon-boot-guard', rootfd))
            lockfd = os.open('lock', os.O_RDWR | os.O_CREAT | os.O_NOFOLLOW, 0o600, dir_fd=statefd)
            stack.callback(os.close, lockfd)
            self.checked(lockfd)
            fcntl.flock(lockfd, fcntl.LOCK_EX)
            previous = self.read_marker(statefd)
            if previous == self.identity:
                return
            if initialize:
                if previous is not None or not self.available(self.root):
                    raise Refused('initialization requires the existing Incus daemon')
                self.write_marker(statefd)
                return
            if self.available(self.root):
                raise Refused('Incus is already running')
            records = []
            for kind in ('networks', 'devices'):
                try:
                    entities = stack.enter_context(self.directory(kind, rootfd))
                except FileNotFoundError:
                    continue
                for entity in sorted(os.listdir(entities)):
                    entityfd = stack.enter_context(self.directory(entity, entities))
                    for name in sorted(os.listdir(entityfd)):
                        selected = (name == 'dnsmasq.pid' if kind == 'networks'
                                    else name.startswith('proxy.') and len(name) > len('proxy.'))
                        if not selected:
                            continue
                        record = os.open(name, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=entityfd)
                        stack.callback(os.close, record)
                        info = self.checked(record)
                        records.append((kind, entity, name, entityfd, info))
            if previous is None and records:
                raise Refused('uninitialized PID records; rerun the installer while Incus is ready')
            # Preflight completes before moving anything. No record is parsed,
            # executed, signalled, or deleted. Interrupted moves remain archived
            # and are safe to resume while the previous marker remains.
            if records:
                archive = 'retired-' + uuid.uuid4().hex
                os.mkdir(archive, 0o700, dir_fd=statefd)
                os.fsync(statefd)
                archivefd = stack.enter_context(self.directory(archive, statefd))
                for kind, entity, name, entityfd, before in records:
                    with contextlib.suppress(FileExistsError):
                        os.mkdir(kind, 0o700, dir_fd=archivefd)
                    kindfd = stack.enter_context(self.directory(kind, archivefd))
                    with contextlib.suppress(FileExistsError):
                        os.mkdir(entity, 0o700, dir_fd=kindfd)
                    destination = stack.enter_context(self.directory(entity, kindfd))
                    os.fsync(archivefd)
                    os.fsync(kindfd)
                    after = os.stat(name, dir_fd=entityfd, follow_symlinks=False)
                    fields = ('st_dev', 'st_ino', 'st_nlink', 'st_uid', 'st_mode', 'st_size', 'st_mtime_ns', 'st_ctime_ns')
                    if any(getattr(before, field) != getattr(after, field) for field in fields):
                        raise Refused('provider PID record changed')
                    os.rename(name, name, src_dir_fd=entityfd, dst_dir_fd=destination)
                    os.fsync(entityfd)
                    os.fsync(destination)
                os.fsync(statefd)
            self.write_marker(statefd)


def main():
    if os.geteuid() != 0 or sys.argv[1:] not in ([], ['--initialize']):
        print('haco-incus-boot-guard: requires root; usage: [--initialize]', file=sys.stderr)
        return 2
    try:
        Guard('/var/lib/incus', namespace_identity()).run(bool(sys.argv[1:]))
    except (Refused, OSError, ValueError, KeyError, TypeError):
        # Never emit arbitrary provider contents or subprocess output.
        print('haco-incus-boot-guard: cannot safely prepare PID records; check provider state and rerun the installer', file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    sys.exit(main())
