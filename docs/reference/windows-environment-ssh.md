# Windows OpenSSH access to an Environment

Status: automatic SSH setup is implemented; commit-bound Windows/editor acceptance
belongs in [implementation status](../IMPLEMENTATION_STATUS.md).

From trusted haco-host:

```bash
haco ssh setup my-dev
haco open my-dev
# Optional terminal client:
haco open --client ssh my-dev
```

A single Environment needs no name. With several, run `haco open` or
`haco ssh setup` in a terminal and select a number from the Environment/Workspace
list. Blank input cancels. Scripts should supply the name explicitly. Setup manages Windows-owned keys, host-key
pins and the SSH include. ProxyCommand resumes stopped Environments and reuses matching
connections. See the [client contract](../design/client-adapters-and-vscode-integration.md#desktop-ssh-setup-and-vs-code-opening)
for ownership and recovery.

The installed Windows acceptance fixture records editor, project setup, preview
and Environment doctor failures separately and continues the independent probes.
Native SSH failures record only allowlisted client progress, stream failure reasons
and fixed fixture markers: connection, authentication, session, received
exit status and command progress. Raw verbose SSH output and key/peer details
are not emitted. These observations diagnose a failure and never replace pinned
host-key checks or successful completion. The five-minute deadline is unchanged.
Any recorded failure still fails the job after host-key refusal checks and
cleanup. A later PASS marker never erases an earlier failure.

## Reconnect from VS Code after reboot

After successful setup and opening `/workspace`, select `haco-my-dev` from
Remote Explorer → SSH Targets, or open its recent remote folder. Standard
Remote-SSH reads the managed Include; no Hacocoon extension is required.

A generated fragment has this shape (the encoded target is intentionally abbreviated):

```sshconfig
Host haco-my-dev
  HostName haco-my-dev
  User root
  IdentityFile ~/.ssh/hacocoon/identity
  StrictHostKeyChecking yes
  HostKeyAlias haco-my-dev
  UserKnownHostsFile ~/.ssh/hacocoon/known-<pin-id>
  ProxyCommand C:/Windows/System32/wsl.exe --distribution Hacocoon --exec /usr/local/bin/haco stream <encoded-target>
```

WSL starts normally and the client waits up to two minutes for the enabled
controller service. Only the same stopped Environment may resume. A deleted,
replaced, revoked or recovery-required target fails; no install/repair, new
Environment or Workspace adoption occurs. `haco ssh cleanup` removes positively
stale managed fragments without deleting unrelated targets or the shared key.

This is the implemented reconnect route. Actual reboot, GUI selection and supported
Incus-version acceptance must be read from the commit-bound evidence, not inferred
from a successful OpenSSH config parse or editor launch.

## Advanced manual configuration

The existing explicit public-key/configuration flow remains available:

Use Windows standard `%WINDIR%\System32\OpenSSH\ssh.exe` and `ssh-keygen.exe`.
Create a dedicated keypair on Windows. The private key stays in that Windows
directory; pass only the public-key path to trusted `haco-host` through Windows
drive projection. To obtain its Linux path, use `wsl -d Hacocoon --exec wslpath
-u -a <Windows-public-key-path>` (on one command line). This relies on WSL's path
conversion and does not assume a particular drive letter.

From the trusted Host, prepare access and generate configuration:

```bash
haco env ssh --key <projected-public-key-path> my-dev
haco env ssh-config my-dev
```

Save the generated text as a UTF-8 file on Windows. It contains a stable alias, root user, strict checking and a creation-bound
ProxyCommand through the saved WSL distribution. It does not embed a private key
or edit the user's `.ssh/config`.

The `haco env ssh` JSON response includes `host_public_key`. Hacocoon reads only
the Environment's public Ed25519 host key through the trusted provider channel,
validates its SSH wire structure and drops its comment. Use that value for
pinning; ordinary users no longer need to invoke Incus as an administrator.
Malformed keys fail preparation and trigger managed key/grant cleanup.

Put `haco-my-dev ssh-ed25519 <public-key-data>` into a dedicated Windows
`known_hosts` file. An unauthenticated `ssh-keyscan` result alone is not trusted
identity. With the generated configuration, run Windows native OpenSSH:

```powershell
& "$env:WINDIR\System32\OpenSSH\ssh.exe" `
  -F <generated-config> -i <windows-private-key> `
  -o UserKnownHostsFile=<dedicated-known-hosts> `
  haco-my-dev "pwd && test -d /workspace"
```

The path is Windows OpenSSH stdio through ProxyCommand, `wsl.exe`, the controller
UDS and the generic byte session to Environment sshd port 22. No raw Incus socket is projected to
the guest. Windows drives, `/init`, WSL sockets and Windows executable authority
remain exclusive to trusted `haco-host`. Host-key mismatch fails before remote
execution; after intentionally recreating an Environment, verify the new public
key through the trusted channel before updating the pin.

Use `haco env disconnect my-dev <connection-id>` to revoke this access. Normal
Environment deletion removes its grants and retains Workspace/OCI Store data.
Keep the client private key on Windows throughout. The maintained
[`tools/test_windows_environment_ssh.ps1`](../../tools/test_windows_environment_ssh.ps1)
uses an isolated Windows key directory, a dedicated pin, a mismatch rejection
probe, `/workspace` verification and cleanup. On the disposable GitHub Actions user,
it also installs the managed Include and tests parallel cold reconnect and standard
VS Code Remote-SSH. Local manual runs preserve the operator's SSH configuration.

After a fresh candidate ZIP passes the ordinary Windows installer gate, run
`python test/e2e/windows/access.py --require-non-c` on a machine with
a writable additional drive. The maintained ConPTY driver checks native interop
in ordinary trusted Host terminals before and after the complete SSH lifecycle.
It closes the first terminal before intentional WSL shutdown and opens a new one
after cold reconnect. Local manual runs leave the user's SSH configuration untouched.

## Package access from SSH sessions

SSH preparation installs current managed HTTP(S)/NO_PROXY session settings through
an OpenSSH SetEnv drop-in, validates sshd configuration and reloads it. Interactive
and command sessions can use the same policy-controlled proxy as Incus exec.
This does not permit a domain or inherit an old Env grant; configure current
network Policy as usual. See [ADR 0058](../adr/0058-ssh-session-egress-environment.md).
Installed SSH package and transfer acceptance passed at `684e411`; `7517c27` failed before this fix. See [the bounded evidence](../status/acceptance-evidence.md#transfer).


## Host-key refusal evidence

The maintained native-client check requires a nonzero exit, no forbidden command
marker and the original host-key refusal diagnostic. Its outcome distinguishes
`exit_zero`, `command_marker` and `refusal_unconfirmed`; all fail the check.
An unrelated connection failure is not proof that the client accepted a changed key.
Verbose native SSH output is reduced through the existing fixed progress allowlist;
raw peer/key/output content is not printed by the new failure path. A recorded WSL
`E_UNEXPECTED` is a diagnostic observation only, never an alternate PASS or retry.
The same component test runs locally and in the existing Windows installer workflow.
