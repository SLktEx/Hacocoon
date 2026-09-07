# Development preview

[日本語](development-preview.ja.md) | English

Status: **partial roadmap C5**. The product CLI and loopback connection path are
implemented. Installed Windows acceptance for this slice is pending.

## Ordinary use

Start your application inside the Environment, then run:

```bash
haco open --port 3000 dev
haco open --port 3000 --close dev
```

The first command resumes the Environment, reuses an existing connection for
that target port or assigns a free Physical Host loopback port, prints its HTTP
URL and launches the desktop browser. With exactly one Environment, its name
can be omitted. Use `--no-browser` to print the URL for scripts.

The second command closes matching TCP connections without starting the
Environment. It does not stop the development server. Environment deletion
also removes the provider connections. Existing `haco open dev` still opens
VS Code.

## Boundaries

This uses the existing client connection provider contract, not a public
listener or an egress Policy exception. The Incus proxy listens only on
127.0.0.1 and connects to the selected Environment's loopback port. Host port
selection is a probe; successful provider binding remains required, and a
concurrent bind fails instead of redirecting the request.

Only validated loopback connection results become a browser URL. Provider
URLs, arbitrary hosts, paths and shell fragments are never launched. The
Environment cannot choose desktop commands. Explicit HTTP preview does not
grant outbound DNS or network access and does not expose the application to
the LAN. Access is available to other local processes.

## Acceptance

Focused tests cover reuse, close without resume, and invalid endpoint refusal.
The Windows installer fixture starts a Python HTTP server through ordinary
project setup, reads the Workspace marker from Windows, reuses the same URL,
and checks connection refusal after close. This fixture has not run for this
change yet. Actual browser launch and rendering remain unverified; HTTP
acceptance alone must not be reported as browser acceptance.

See [client adapters](client-adapters-and-vscode-integration.md).
