# v0.14 — Git Fetch Plugin

Status: **implemented on `main`.**

v0.14 adds brokered GitHub fetch under the plugin namespace while keeping reusable host credentials outside coding Environments.

## CLI

```text
haco plugin git fetch <environment> [--remote <remote>]
```

## Authority boundary

- Fetch is a Git/GitHub capability plugin, not a Core Environment lifecycle command.
- HTTPS authentication uses the host-owned `gh auth git-credential` helper.
- Credentials remain on the trusted Host and are never copied into the Environment.
- Global/system Git configuration is disabled for the privileged broker path.
- Repository-controlled credential helpers, URL rewrites, transport hooks, and unsafe HTTP configuration are rejected.
- The broker uses a validated GitHub remote and a fixed branch refspec rather than trusting repository-controlled `remote.<name>.fetch`.
- Tags and submodules are not implicitly fetched by the brokered operation.

## Relationship to v0.5

v0.5 introduced the Git/GitHub capability boundary and brokered push. v0.14 is a separate independently useful feature that extends that plugin with credential-safe fetch; it does not move Git into Hacocoon Core.

## Acceptance

CLI parsing, trusted credential-helper injection, and hostile Git configuration are covered by repository tests. Real private-repository/provider combinations remain environment-dependent acceptance.
