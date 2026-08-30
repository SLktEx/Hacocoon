# Git / GitHub Capability Plugin

Hacocoon keeps privileged GitHub authority on the trusted Host side. Ordinary local Git remains Git's responsibility; fetch/push that need Host credentials or external authority cross the Hacocoon Policy/Capability boundary through the plugin namespace.

## CLI

```bash
haco plugin git fetch <environment>
haco plugin git fetch <environment> --remote upstream

haco plugin git push <environment> --branch feature/x
haco plugin git push <environment> --branch main --force
```

Push may also select `--source <revision>` and `--remote <remote>`.

This is intentionally not a wrapper for every Git command. `status`, `diff`, `commit`, ordinary fetches performed with Environment-owned credentials, branch/worktree management and other local Git UX remain outside Hacocoon Core.

## Trusted repository identity

Before a brokered operation, Hacocoon resolves and pins the registered Workspace repository boundary. It accepts only deliberately narrow normal-repository / standard linked-worktree layouts and rejects unsafe `.git` indirection, symlink tricks, mismatched worktree backlinks, object alternates and transport/credential/command-sensitive repository configuration.

The canonical repository identity is hashed into a non-secret capability attribute. It is recomputed before privileged execution; a changed repository boundary makes the request stale.

## Fetch normalization and execution

For `haco plugin git fetch` Hacocoon:

1. resolves the Environment to its registered Host Workspace;
2. validates and pins repository identity;
3. resolves the selected remote;
4. accepts credential-free `github.com` HTTPS/SSH remotes;
5. normalizes organization, repository and remote name into capability-visible authority;
6. runs policy/approval before authenticated network access;
7. after authorization, executes fetch using the validated GitHub URL and a fixed refspec that updates only `refs/remotes/<remote>/*`.

The broker does **not** trust repository-controlled `remote.<name>.fetch` for the privileged fetch path. Tags and submodules are not implicitly fetched.

## Push normalization and execution

Push authorization includes GitHub organization/repository, target branch/ref, exact source SHA and repository identity. Approval is granted to an exact source commit rather than a moving branch name.

Force push uses `--force-with-lease`, never raw `--force`. The expected target SHA comes from the already-fetched local remote-tracking ref before policy evaluation. After authorization, Hacocoon verifies the real remote ref still matches that approved baseline before executing the force-with-lease push.

## Host credential provider

Brokered Git processes run with a freshly constructed environment rather than inheriting arbitrary caller state. Global/system Git configuration and ambient credential/askpass overrides are disabled.

For GitHub HTTPS, the plugin explicitly configures the Host-owned `gh auth git-credential` provider for the brokered operation. This lets normal `gh auth login` / `gh auth setup-git` Host configuration work for private repositories without exporting PATs or the Host credential configuration into a Sandbox.

For SSH remotes, the hardened broker may use Host default keys / `SSH_AUTH_SOCK` while disabling user SSH config rewrites that could redirect the validated GitHub host.

The following are never copied into the Environment, policy request or audit log:

- GitHub PAT/token plaintext;
- credential-helper response plaintext;
- SSH private keys;
- authorization headers.

## Process isolation

Brokered Git starts from a minimal explicit environment and pins `--git-dir` / `--work-tree`. Caller-controlled `GIT_*`, askpass, proxy and transport-command state must not silently cross the capability boundary.

Repository-local configuration is visible only where needed to identify the remote, and transport/credential/command-sensitive entries are rejected before authorization and again before execution.

## Audit

Audit records non-secret authority attributes such as organization, repository, hashed repository identity, remote, target ref, exact source SHA, and expected remote SHA for force-with-lease. Credential material is excluded.

## Product boundary

The Git/GitHub implementation is an adapter/plugin, not Core domain behavior. Core owns generic Policy/Capability/Audit contracts; GitHub remote parsing, repository/ref authority, Host credential use and brokered Git execution belong to the Git plugin.
