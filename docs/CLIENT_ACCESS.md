# Client access

Hacocoon keeps IDEs outside Core. A client receives stable Environment and connection information and uses standard protocols.

## Status

```bash
haco status demo
haco status demo --json
```

The JSON form is intended for scripts and client adapters.

## Local port forwarding

v0.3 only creates host-loopback listeners. It does not publish services on LAN/public interfaces.

```bash
haco forward demo --host-port 8080 --target-port 3000
haco unforward demo tcp-8080-3000
```

Broad exposure belongs to the v0.4 Policy/Capability boundary.

## SSH and VS Code Remote-SSH

Hacocoon accepts a **public** SSH key only. The private key stays with the client.

```bash
haco ssh demo --public-key ~/.ssh/id_ed25519.pub --host-port 2222
```

The command prints the equivalent SSH connection command. A matching OpenSSH client entry is:

```sshconfig
Host haco-demo
    HostName 127.0.0.1
    User root
    Port 2222
    IdentityFile ~/.ssh/id_ed25519
```

VS Code can then use its standard Remote-SSH workflow against `haco-demo`. Hacocoon does not require a VS Code extension for this path.

## code-server

Run code-server as an ordinary workload in the Environment and expose only its local port:

```bash
haco exec demo -- code-server --bind-addr 127.0.0.1:3000 /workspace
haco forward demo --host-port 3000 --target-port 3000
```

Authentication remains the responsibility of the service/client. Hacocoon v0.3 only supplies the local connection mechanism.
