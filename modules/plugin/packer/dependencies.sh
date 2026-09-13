set -eu
# Runs only in the disposable builder, with its ordinary network permissions.
# Custom Bases can provide these tools themselves; apt is needed only if absent.
if [ ! -x /usr/bin/python3 ] || [ ! -x /usr/bin/ssh-keygen ] || [ ! -x /usr/sbin/sshd ]; then
  if [ ! -x /usr/bin/apt-get ]; then
    printf '%s\n' 'Packer requires Python 3 and OpenSSH client/server in the starting Base.' >&2
    exit 1
  fi
  export DEBIAN_FRONTEND=noninteractive
  /usr/bin/apt-get update
  /usr/bin/apt-get install -y --no-install-recommends python3 ca-certificates openssh-server
fi
test -x /usr/bin/python3
test -x /usr/bin/ssh-keygen
test -x /usr/sbin/sshd
