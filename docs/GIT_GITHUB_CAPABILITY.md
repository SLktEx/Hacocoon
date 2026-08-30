# Git / GitHub capability

Hacocoon v0.5 keeps GitHub authority on the host side. An Environment can keep using ordinary Git for local operations, while privileged pushes cross the Hacocoon Policy/Capability boundary.

## Brokered push

The current narrow privileged entry point is:

```bash
haco git push <environment> --branch feature/x
haco git push <environment> --branch main --force
```

Optional selectors:

```text
--source <revision>   default: HEAD
--remote <remote>     default: origin
```

This is intentionally **not** a Hacocoon wrapper for every Git command. Commit, diff, status, fetch, worktree handling, and other ordinary Git UX remain Git's responsibility. A future transparent remote-helper/IPC path should only be added if it preserves the same security boundary without revealing credentials.

## Trusted repository identity

Before any brokered Git command, Hacocoon resolves an explicit repository boundary for the registered Workspace and pins every Git subprocess to that exact `--git-dir` and `--work-tree`. Git is therefore not allowed to discover a parent repository or follow a newly swapped `.git` pointer between validation and execution.

The accepted layouts are deliberately narrow:

- a normal Workspace-local `.git/` directory;
- a standard linked Git worktree whose `.git` file points to `<common-dir>/worktrees/<id>`, whose admin directory has a `gitdir` backlink to this exact Workspace `.git` file, and whose `commondir` resolves back to that common directory.

Arbitrary external `gitdir:` targets, symlink `.git` entries, mismatched worktree backlinks, non-standard linked-worktree admin layouts, and Git object alternates are rejected. Repository-controlled config includes, HTTP transport configuration, credential helpers, URL rewrites, hook/SSH/askpass commands, and `core.worktree` overrides are also rejected for the privileged broker path.

The canonical worktree/gitdir/commondir tuple is hashed into a non-secret `repository_identity` capability attribute. The provider recomputes that identity immediately before privileged execution and fails stale if it changed after policy evaluation. Raw host paths are not written into the capability audit for this purpose.

## Request normalization

Before policy evaluation Hacocoon:

1. resolves the Environment to its registered host Workspace;
2. validates and pins the trusted repository identity described above;
3. reads the configured remote URL from that pinned repository;
4. accepts only credential-free `github.com` HTTPS/SSH remotes;
5. normalizes organization, repository, target branch/ref, and operation;
6. resolves the requested source revision to an exact Git object SHA;
7. for a force push, resolves the expected target SHA only from the already-fetched local `refs/remotes/<remote>/<branch>` tracking ref;
8. records those non-secret, authority-sensitive values as auditable capability attributes.

No authenticated remote lookup is performed during this pre-policy normalization. A force push therefore requires a fetched local remote-tracking ref. If that local baseline is absent, Hacocoon fails closed and the operator must fetch/update the tracking ref before retrying.

Example policy:

```json
{
  "default": "deny",
  "rules": [
    {
      "capability": "github.git",
      "action": "push",
      "resource": "github://acme/demo/refs/heads/feature/x",
      "environment": "demo",
      "attributes": {
        "organization": "acme",
        "repository": "demo",
        "repository_identity": "*",
        "remote": "origin",
        "source_sha": "*",
        "target_ref": "refs/heads/feature/x"
      },
      "decision": "allow"
    },
    {
      "capability": "github.git",
      "action": "force-push",
      "resource": "github://acme/demo/refs/heads/main",
      "environment": "demo",
      "attributes": {
        "organization": "acme",
        "repository": "demo",
        "repository_identity": "*",
        "remote": "origin",
        "source_sha": "*",
        "target_ref": "refs/heads/main",
        "expected_remote_sha": "*"
      },
      "decision": "require-approval"
    }
  ]
}
```

Policy scope is exact by default. `resource` must always be present, and only the literal value `"*"` means any resource. Environment is also part of the policy scope. Every request attribute must be represented by the matching rule; use an explicit attribute value of `"*"` when the value may vary safely, such as the source commit SHA or hashed repository identity.

## Exact source enforcement

Approval is not granted to a moving branch name. Hacocoon resolves the source to a SHA before policy/approval and the provider pushes that exact SHA:

```text
<approved SHA>:<approved refs/heads/...>
```

The provider revalidates the repository identity and re-reads/re-normalizes the GitHub remote immediately before the push. If the repository boundary, remote, repository, or target no longer matches the approved request, the capability is stale and the push is refused.

Force pushes use `--force-with-lease`, not raw `--force`. Before policy/approval, the expected remote SHA comes from the local remote-tracking ref only. After policy/approval succeeds, the provider performs the authenticated `ls-remote`, requires the real remote ref to equal the approved local baseline, and only then executes the force-with-lease push. A stale local tracking ref or a remote change invalidates the request instead of causing pre-policy network access.

## Parameters and credentials

The broker currently carries the normalized remote URL as compatibility metadata, but the provider does not use that opaque value to select authority or execute the push. The provider explicitly declares that key as non-authority; execution re-resolves the remote from the Workspace. Future Git/GitHub inputs that can affect authority must be added to policy-visible attributes instead of hidden in parameters.

No `GH_TOKEN`, GitHub PAT, SSH private key, credential-helper plaintext, or authorization header is copied into the Environment, Hacocoon state, policy request, or audit log.

### Host Git process isolation

Every Git command used by the broker/provider runs with a freshly constructed environment rather than inheriting the Hacocoon process environment. This is part of the capability trust boundary, not an optional hardening step.

The brokered process starts through `/usr/bin/env -i` and `/usr/bin/git`. It sets only:

```text
PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
GIT_CONFIG_NOSYSTEM=1
GIT_CONFIG_GLOBAL=/dev/null
GIT_TERMINAL_PROMPT=0
GIT_ASKPASS=/usr/bin/false
SSH_ASKPASS=/usr/bin/false
GIT_SSH_COMMAND=/usr/bin/ssh -F /dev/null -o BatchMode=yes -o ClearAllForwardings=yes -o PermitLocalCommand=no
```

and selectively carries `HOME`, `SSH_AUTH_SOCK`, `LANG`, and `LC_ALL` when present.

All ambient `GIT_*` variables other than the values explicitly created above are discarded. In particular, caller-controlled `GIT_CONFIG_COUNT`, `GIT_CONFIG_KEY_*`, `GIT_CONFIG_VALUE_*`, `GIT_CONFIG_GLOBAL`, `GIT_CONFIG_SYSTEM`, `GIT_DIR`, `GIT_WORK_TREE`, `GIT_SSH`, `GIT_SSH_COMMAND`, and `GIT_ASKPASS` cannot flow into brokered Git. Ambient `SSH_ASKPASS`, proxy variables, and other unrelated process state are also not inherited.

Global and system Git configuration are deliberately disabled. Repository-local configuration remains visible because the Workspace remote must be resolved, but transport/credential/command-sensitive local entries are rejected before authorization and again before execution.

For SSH remotes, Hacocoon disables user SSH configuration with `-F /dev/null`. The host user's default SSH keys/known-hosts and `SSH_AUTH_SOCK` may still provide authentication, but `~/.ssh/config` cannot introduce `HostName`, `ProxyCommand`, or similar transport rewrites for `github.com`.

For HTTPS remotes, ambient global credential helpers are intentionally not trusted because global Git configuration is outside the brokered capability boundary. A future HTTPS credential provider must be explicit host-owned provider state, not inherited Git configuration or Environment-supplied credentials.

GitHub App token minting can be added behind this same provider boundary when deployment conditions make it useful; it is not required to weaken the boundary by injecting tokens into the Environment.

## Audit

Audit events include a request ID plus non-secret attributes such as:

- organization;
- repository;
- hashed repository identity;
- target ref;
- exact source SHA;
- remote name;
- expected remote SHA for force-with-lease.

Opaque provider-declared non-authority `Parameters` remain excluded from audit by design.
