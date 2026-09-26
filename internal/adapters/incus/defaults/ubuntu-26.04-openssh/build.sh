#!/bin/sh
set -eu

here="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
exec haco base build --name ubuntu-26.04-openssh --from haco/ubuntu-26.04 "$here"
