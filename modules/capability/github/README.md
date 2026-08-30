# GitHub Git capability plugin

This package implements the optional GitHub-aware Git capability used by `haco plugin git fetch` and `haco plugin git push`.

It is an adapter/plugin, not a Core domain dependency. Core owns only generic capability contracts and policy decisions. This package owns Git/GitHub-specific behavior such as remote parsing, repository/branch authority checks, stale approval detection, and the final brokered `git fetch` / `git push`.

The sandbox does not receive GitHub credentials from this plugin. Host-side brokered Git keeps global/system Git configuration disabled and explicitly installs only `gh auth git-credential` as the HTTPS credential provider for `github.com`. Operators who normally use `gh auth login` / `gh auth setup-git` can therefore access private repositories without copying PATs or the complete host credential-helper configuration into the sandbox or repository.

`fetch` runs only after the capability service evaluates the GitHub repository, remote, and repository identity. Execution uses the validated GitHub URL plus a fixed refspec rather than repository-controlled `remote.<name>.fetch`, and updates only `refs/remotes/<remote>/*`. Tags and submodules are not fetched automatically.

`push` continues to run only after the capability service evaluates the exact GitHub repository, target ref, source commit, and force-push semantics. SSH remotes may still use the host user's default key or `SSH_AUTH_SOCK`.

Hacocoon currently uses ordinary Go package boundaries and static composition for plugins. This is intentionally not a dynamic shared-object/plugin loader. The CLI namespace makes that extension boundary explicit without moving security-sensitive authority out of the host-owned Capability implementation.
