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

A single Environment needs no name. Setup manages Windows-owned keys, host-key
pins and the SSH include, resumes stopped Environments and reuses matching
connections. See the [client contract](../design/client-adapters-and-vscode-integration.md#desktop-ssh-setup-and-vs-code-opening)
for ownership and recovery.

The installed Windows acceptance fixture records editor, project setup, preview
and Environment doctor failures separately and continues the independent probes.
Any recorded failure still fails the job after host-key refusal checks and
cleanup. A later PASS marker never erases an earlier failure.

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
haco env ssh --key <projected-public-key-path> --port 22229 my-dev
haco env ssh-config my-dev
```

Save the generated text as a UTF-8 file on Windows. It contains a loopback host,
port, root user and `StrictHostKeyChecking yes`. It does not embed a private key
or edit the user's `.ssh/config`.

The `haco env ssh` JSON response includes `host_public_key`. Hacocoon reads only
the Environment's public Ed25519 host key through the trusted provider channel,
validates its SSH wire structure and drops its comment. Use that value for
pinning; ordinary users no longer need to invoke Incus as an administrator.
Malformed keys fail preparation and trigger managed key/proxy cleanup.

Put `[127.0.0.1]:22229 ssh-ed25519 <public-key-data>` into a dedicated Windows
`known_hosts` file. An unauthenticated `ssh-keyscan` result alone is not trusted
identity. With the generated configuration, run Windows native OpenSSH:

```powershell
& "$env:WINDIR\System32\OpenSSH\ssh.exe" `
  -F <generated-config> -i <windows-private-key> `
  -o UserKnownHostsFile=<dedicated-known-hosts> `
  haco-my-dev "pwd && test -d /workspace"
```

The path is Windows `127.0.0.1:<port>` to WSL Physical Host loopback, through an
Incus loopback proxy into Environment sshd. No raw Incus socket is projected to
the guest. Windows drives, `/init`, WSL sockets and Windows executable authority
remain exclusive to trusted `haco-host`. Host-key mismatch fails before remote
execution; after intentionally recreating an Environment, verify the new public
key through the trusted channel before updating the pin.

Use `haco env disconnect my-dev <connection-id>` to revoke this access. Normal
Environment deletion removes its listener and retains Workspace/OCI Store data.
Keep the client private key on Windows throughout. The maintained
[`tools/test_windows_environment_ssh.ps1`](../../tools/test_windows_environment_ssh.ps1)
uses an isolated Windows key directory, a dedicated pin, a mismatch rejection
probe, `/workspace` verification and cleanup; it never installs user SSH config.

After a fresh candidate ZIP passes the ordinary Windows installer gate, run
`python tools/windows-native-access-e2e.py --require-non-c` on a machine with
a writable additional drive. The maintained ConPTY driver keeps an ordinary
trusted Host shell open, checks native interop before and after the complete
SSH lifecycle, and leaves the user's SSH configuration untouched.
