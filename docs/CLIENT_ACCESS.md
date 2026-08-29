# Client access

Hacocoon keeps IDEs outside Core. A client receives stable Environment and connection information and uses standard protocols.

## Status

```bash
haco status demo
haco status demo --json
```

The JSON form is intended for scripts and client adapters.

## Active connections

Hacocoon derives active client connections from its `haco-*` Incus proxy devices, so a new CLI process can rediscover listeners created by an earlier process.

```bash
haco connections demo
haco connections demo --json
```

The Incus device state is the source of truth for transport reconciliation; Hacocoon does not maintain a second connection-state database.

## Local port forwarding

v0.3 only creates host-loopback listeners. It does not publish services on LAN/public interfaces.

```bash
haco forward demo --host-port 8080 --target-port 3000
haco unforward demo tcp-8080-3000
```

An omitted protocol is normalized to TCP before runtime dispatch. Broad exposure belongs to the v0.4 Policy/Capability boundary.

## SSH and VS Code Remote-SSH

Hacocoon accepts a **public** SSH key only. The private key stays with the client. Public keys are structurally validated as OpenSSH keys before the Environment is changed.

```bash
haco ssh demo --public-key ~/.ssh/id_ed25519.pub --host-port 2222
```

SSH setup first reserves the loopback proxy. Only after that succeeds does Hacocoon install a connection-scoped managed key marked with the connection ID. A failed proxy reservation therefore cannot leave a newly granted key behind. If later provisioning fails, the reserved proxy is removed as cleanup.

The command prints the equivalent SSH connection command. A matching OpenSSH client entry is:

```sshconfig
Host haco-demo
    HostName 127.0.0.1
    User root
    Port 2222
    IdentityFile ~/.ssh/id_ed25519
```

VS Code can then use its standard Remote-SSH workflow against `haco-demo`. Hacocoon does not require a VS Code extension for this path.

SSH credentials follow the Hacocoon connection lifecycle. Removing an SSH connection removes its managed key before removing the proxy:

```bash
haco unforward demo ssh-2222
```

Other keys already present in `authorized_keys` are preserved. Hacocoon does not stop `sshd` automatically because the service may also be used by non-Hacocoon configuration inside the Environment.

## code-server

Run code-server as an ordinary workload in the Environment and expose only its local port:

```bash
haco exec demo -- code-server --bind-addr 127.0.0.1:3000 /workspace
haco forward demo --host-port 3000 --target-port 3000
```

Authentication remains the responsibility of the service/client. Hacocoon v0.3 only supplies the local connection mechanism.
