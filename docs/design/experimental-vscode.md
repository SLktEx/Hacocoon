# Experimental VS Code integration

[日本語](experimental-vscode.ja.md) | English

VS Code settings and extension installation belong to the optional client adapter.
Core, providers and Environment lifecycle APIs gain no editor-specific authority.
The YAML subtree `experimental.vscode` is the canonical input; its compatibility
is not guaranteed during experimentation. [The reference](../reference/experimental-vscode.md)
owns the schema, defaults, commands, prerequisites and current limitations.

Trusted Host user YAML is edited through one strict parser, schema validator and
revision-bound, locked, atomic writer. Reads do not create files. Only the subtree
is replaced; other YAML values survive. Direct file editing remains supported
with writer coordination. No controller-side shadow DB or guest-controlled
configuration writer is introduced.

`haco open` and the standalone VS Code adapter share application code. They reuse
the existing pinned SSH alias and its lifecycle/Workspace binding. Marketplace
metadata is fetched from a fixed HTTPS origin on the Host, bounded and checked
for exact extension identity. Versions are compared semantically with a single
UTC cutoff. Explicit pins bypass age/pre-release filtering. Dependencies and
packs are resolved under the same policy before editor launch, with a bounded
graph. Platform artifact selection precedes age filtering so an old universal
artifact cannot authorize a newly published platform build.

Remote-SSH owns server bootstrap. The adapter waits for the exact stable desktop
build, uses its Node runtime and invokes its extension CLI inside the Env. There
is no extra guest Python package or backend-specific preparation step. Settings
travel as JSON on SSH stdin, never shell source or argv. A guest advisory lock
serializes Hacocoon applications. Directory descriptors, no-follow opens,
single-link regular-file checks and atomic replacement keep Remote settings
separate from repository paths. An embedded ownership comment tracks only the
derived managed setting keys; it is not an alternative configuration source.

Guest results remain untrusted; a successful application receipt grants no Host
authority and does not prove the editor UI is connected. Captured subprocess
output is bounded and omitted from diagnostics. Partial installation failures
retain the Env, settings and already installed extensions. Subsequent opens
reconcile selected versions; compatibility failures never choose an unreviewed
latest release. Existing UI changes/auto-updates are outside continuous enforcement.

All configurations are common Remote settings for the Env/Workspace. No repository
Folder Settings, `.vscode/settings.json` or `.code-workspace` file is generated.
Credentials remain with SSH Agent/Credential Store and existing trusted credential
owners. Future UI may expose state and references, not credential material.
See [ADR 0078](../adr/0078-experimental-vscode-ownership.md) for rejected alternatives.
