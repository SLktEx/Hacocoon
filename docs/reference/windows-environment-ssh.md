# Windows OpenSSH access to an Environment

Status: implemented manual configuration; real Windows acceptance belongs in
[implementation status](../IMPLEMENTATION_STATUS.md). Automatic SSH config
installation, VS Code integration and final SSH UX are deferred to Stage D+.

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

Pin the server's public host key using a trusted channel. For this local manual
PoC, the Physical Host administrator can read it directly:

```powershell
wsl -d Hacocoon -u root --exec incus exec haco-my-dev --project hacocoon -- cat /etc/ssh/ssh_host_ed25519_key.pub
```

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
