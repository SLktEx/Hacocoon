#!/usr/bin/env python3
"""Fetch pinned runtime distributions into a new E2E-only directory.

Never installs or executes downloaded tools on the Physical Host. The Incus
fixture rechecks hashes before putting them into its disposable instances.
"""
import hashlib
import pathlib
import sys
import urllib.request

ASSETS = (
    ("nerdctl-full.tar.gz", "https://github.com/containerd/nerdctl/releases/download/v2.3.5/nerdctl-full-2.3.5-linux-amd64.tar.gz",
     "b697295c623639734aaab737523c808fd3cc8d3046039fd94fff1744e4c317aa"),
    ("docker.tgz", "https://download.docker.com/linux/static/stable/x86_64/docker-28.5.2.tgz",
     "ea90cfd12e1eeb12aa1c971741adb8bd4ed88e2a574eaac13f5029a1dbc6300d"),
)


def main():
    if len(sys.argv) != 2:
        raise SystemExit("usage: prepare-oci-runtime-fixture.py NEW_DIRECTORY")
    destination = pathlib.Path(sys.argv[1])
    destination.mkdir(mode=0o700, parents=False, exist_ok=False)
    for name, url, expected in ASSETS:
        digest = hashlib.sha256()
        size = 0
        with urllib.request.urlopen(url, timeout=60) as response, (destination / name).open("xb") as output:
            while chunk := response.read(1024 * 1024):
                size += len(chunk)
                if size > 512 * 1024 * 1024:
                    raise RuntimeError("oversized fixture distribution")
                digest.update(chunk)
                output.write(chunk)
        if digest.hexdigest() != expected:
            raise RuntimeError("fixture digest mismatch: " + name)
        print(name + " verified " + expected, flush=True)


if __name__ == "__main__":
    main()
