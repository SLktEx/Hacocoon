package incus

// Configure only the managed data/transient roots. Existing unrelated Docker
// options survive; conflicting roots, active daemons and existing default data
// require explicit migration instead of silently hiding local images.
const persistentDockerConfiguration = `python3 -I - <<'HACO_DOCKER_CONFIG'
import json, os, stat, subprocess

def main():
    parent = os.open('/etc/docker', os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    temporary = None
    try:
        info = os.fstat(parent)
        if info.st_uid != 0 or info.st_mode & 0o022: raise ValueError()
        config = {}
        try:
            fd = os.open('daemon.json', os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=parent)
        except FileNotFoundError:
            pass
        else:
            with os.fdopen(fd, 'rb') as source:
                info = os.fstat(source.fileno())
                if not stat.S_ISREG(info.st_mode) or info.st_uid != 0 or info.st_nlink != 1 or info.st_mode & 0o022: raise ValueError()
                raw = source.read(8193)
            if len(raw) > 8192: raise ValueError()
            config = json.loads(raw)
            if not isinstance(config, dict): raise ValueError()
        desired = {'data-root': '/var/lib/hacocoon-oci/docker', 'exec-root': '/run/docker'}
        for key, value in desired.items():
            if key in config and config[key] != value: raise ValueError()
        if all(config.get(key) == value for key, value in desired.items()): return
        result = subprocess.run(['systemctl', 'is-active', '--quiet', 'docker.service', 'docker.socket', 'hacocoon-docker.service', 'hacocoon-docker.socket'], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        if result.returncode not in (3, 4): raise ValueError()
        try:
            data = os.open('/var/lib/docker', os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
        except FileNotFoundError:
            pass
        else:
            try:
                with os.scandir(data) as entries:
                    if next(entries, None) is not None: raise ValueError()
            finally:
                os.close(data)
        config.update(desired)
        temporary = '.haco-' + os.urandom(16).hex()
        fd = os.open(temporary, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600, dir_fd=parent)
        with os.fdopen(fd, 'w') as output:
            json.dump(config, output, separators=(',', ':'))
            output.write('\n')
            output.flush()
            os.fsync(output.fileno())
        os.replace(temporary, 'daemon.json', src_dir_fd=parent, dst_dir_fd=parent)
        temporary = None
        os.fsync(parent)
    finally:
        if temporary is not None: os.unlink(temporary, dir_fd=parent)
        os.close(parent)

try:
    os.makedirs('/etc/docker', mode=0o755, exist_ok=True)
    main()
except Exception:
    raise SystemExit('Docker storage configuration requires stopped runtimes and an empty or already managed layout')
HACO_DOCKER_CONFIG
`
