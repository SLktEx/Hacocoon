#!/usr/bin/env python3
"""Manual real-Incus OCI acceptance; run as root on the installed Physical Host.
Creates only UUID-scoped test resources and exact test-Environment Policy rules.
Failures retain resources and print their names for inspection; no recovery bypass.
"""
import json
import os
import pathlib
import pwd
import stat
import subprocess
import tempfile
import uuid

PREFIX = 'oci-check-' + uuid.uuid4().hex[:12]
POLICY = pathlib.Path('/var/lib/hacocoon/policy.json')
HOSTS = ('archive.ubuntu.com', 'security.ubuntu.com', 'github.com',
         'release-assets.githubusercontent.com', 'objects.githubusercontent.com',
         'auth.docker.io', 'registry-1.docker.io', 'production.cloudflare.docker.com')
INSTALL = r'''
set -eu
export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y -o Dpkg::Options::=--force-confold containerd runc curl ca-certificates busybox-static
cd /tmp
curl -fL --retry 2 -o nerdctl.tar.gz https://github.com/containerd/nerdctl/releases/download/v2.3.5/nerdctl-2.3.5-linux-amd64.tar.gz
printf '%s  %s\n' de3206aeb7cbd5f20f5fb1f55c1e3bf2db1be567812a8a3f5e65eba2488347ee nerdctl.tar.gz | sha256sum -c -
tar -xzf nerdctl.tar.gz -C /usr/local/bin nerdctl
curl -fL --retry 2 -o buildkit.tar.gz https://github.com/moby/buildkit/releases/download/v0.33.0/buildkit-v0.33.0.linux-amd64.tar.gz
printf '%s  %s\n' b6242896d343100808dcbe37565caf381e0a444a6a83d7255926bb1519248ead buildkit.tar.gz | sha256sum -c -
tar -xzf buildkit.tar.gz -C /usr/local bin/buildkitd bin/buildctl
systemctl start containerd buildkit
containerd --version
nerdctl --version
buildkitd --version
test ! -e /init
test ! -e /run/WSL
test ! -e /var/lib/hacocoon-control.sock
test -z "$(find /mnt -mindepth 1 -maxdepth 1 -print -quit)"
test -z "$WSL_INTEROP"
'''

def run(args, *, check=True):
    print('+', ' '.join(args[:8]), flush=True)
    result = subprocess.run(args, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    if result.stdout: print(result.stdout, end='', flush=True)
    if result.stderr: print(result.stderr, end='', flush=True)
    if check and result.returncode: raise RuntimeError(f'command failed ({result.returncode}): {args[:6]}')
    return result

def haco(*args, check=True):
    return run(['incus','exec','haco-host','--project','hacocoon','--','haco',*args], check=check)

def guest(env, script):
    return run(['incus','exec','haco-'+env,'--project','hacocoon','--','/bin/bash','-c',script])

def policy(env, registry=True, remove=False):
    if POLICY.exists():
        info = POLICY.lstat()
        if not stat.S_ISREG(info.st_mode) or info.st_uid != 0 or info.st_mode & 0o022:
            raise RuntimeError('unsafe administrator Policy')
        data = json.loads(POLICY.read_text())
    else: data = {'default':'deny','rules':[]}
    reason = 'Persistent OCI acceptance ' + PREFIX
    data['rules'] = [r for r in data['rules'] if not (r.get('environment') == env and r.get('reason') == reason)]
    if not remove:
        hosts = HOSTS if registry else HOSTS[:5]
        data['rules'] += [dict(capability='network.egress', action='connect', resource=h,
            environment=env, attributes=dict(protocol=p,port=port), decision='allow',reason=reason)
            for h in hosts for p,port in (('http','80'),('https','443'))]
    with tempfile.NamedTemporaryFile(mode='w',dir=POLICY.parent,delete=False) as f:
        json.dump(data,f); f.flush(); os.fsync(f.fileno()); temporary=f.name
    os.replace(temporary,POLICY)

if os.geteuid() != 0: raise SystemExit('Run on the installed WSL Physical Host as root')
user = pwd.getpwnam('hacocoon')
work = pathlib.Path(tempfile.mkdtemp(prefix=PREFIX+'-',dir=user.pw_dir))
os.chown(work,user.pw_uid,user.pw_gid)
store_a, store_b = PREFIX+'-a', PREFIX+'-b'
first, second, separate = PREFIX+'-first', PREFIX+'-second', PREFIX+'-separate'
print(json.dumps(dict(workspace=str(work),stores=[store_a,store_b],environments=[first,second,separate])),flush=True)
try:
    a = json.loads(haco('plugin','oci','store','create',store_a).stdout)['resources'][0]
    b = json.loads(haco('plugin','oci','store','create',store_b).stdout)['resources'][0]
    assert a['native_ref'] != b['native_ref']
    haco('env','create','--workspace',str(work),'--resource','oci:'+store_a,first)
    assert haco('plugin','oci','store','delete',store_a,check=False).returncode != 0
    policy(first)
    guest(first,INSTALL)
    guest(first,r'''set -eu
nerdctl --snapshotter native pull docker.io/library/busybox:latest
nerdctl --snapshotter native run --rm --network none docker.io/library/busybox:latest echo pulled-image-ok
mkdir -p /workspace/build-context
cp /bin/busybox /workspace/build-context/busybox
printf 'FROM scratch\nCOPY busybox /busybox\nCOPY marker /marker\nENTRYPOINT ["/busybox", "cat", "/marker"]\n' > /workspace/build-context/Dockerfile
printf persistent-built-image-ok > /workspace/build-context/marker
cd /workspace/build-context
nerdctl --snapshotter native build --progress plain --network none -t hacocoon-test:local .
nerdctl --snapshotter native run --rm --network none hacocoon-test:local
nerdctl images --digests
buildctl du
''')
    before = guest(first,'nerdctl image inspect --format "{{.Id}}" docker.io/library/busybox:latest hacocoon-test:local').stdout
    haco('env','delete',first)
    retained = json.loads(haco('plugin','oci','store','inspect',store_a).stdout)['resources'][0]
    assert retained == a
    policy(first,remove=True)
    haco('env','create','--workspace',str(work),'--resource','oci:'+store_a,second)
    policy(second,registry=False)
    guest(second,INSTALL)
    after = guest(second,'nerdctl image inspect --format "{{.Id}}" docker.io/library/busybox:latest hacocoon-test:local').stdout
    assert before == after, (before,after)
    reused = guest(second,r'''set -eu
nerdctl --snapshotter native run --rm --pull never --network none docker.io/library/busybox:latest echo pulled-image-reused
nerdctl --snapshotter native run --rm --pull never --network none hacocoon-test:local
cd /workspace/build-context
nerdctl --snapshotter native build --progress plain --network none -t hacocoon-test:local .
buildctl du
''')
    assert 'CACHED' in reused.stderr + reused.stdout
    haco('env','delete',second)
    policy(second,remove=True)
    haco('env','create','--workspace',str(work),'--resource','oci:'+store_b,separate)
    guest(separate,'test ! -d /var/lib/hacocoon-oci/containerd && test ! -d /var/lib/hacocoon-oci/buildkit && test -f /workspace/build-context/marker')
    haco('env','delete',separate)
    haco('plugin','oci','store','delete',store_a)
    haco('plugin','oci','store','delete',store_b)
    assert haco('plugin','oci','store','inspect',store_a,check=False).returncode != 0
    assert haco('plugin','oci','store','inspect',store_b,check=False).returncode != 0
    # Only remove our known files and empty directories, never recursively.
    for name in ('Dockerfile','marker','busybox'): (work/'build-context'/name).unlink()
    (work/'build-context').rmdir()
    work.rmdir()
    print('PERSISTENT OCI STORE: PASS',flush=True)
except Exception:
    print('Acceptance failed; retained test resources above require inspection.',flush=True)
    raise
finally:
    for env in (first,second,separate): policy(env,remove=True)