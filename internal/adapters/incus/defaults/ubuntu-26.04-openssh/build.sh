#!/bin/sh
set -eu

here="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
exec haco base build --name ubuntu-26.04-openssh "$here"
