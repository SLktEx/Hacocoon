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

## Request normalization

Before policy evaluation Hacocoon:

1. resolves the Environment to its registered host Workspace;
2. reads the configured remote URL from the host Workspace;
3. accepts only credential-free `github.com` HTTPS/SSH remotes;
4. normalizes organization, repository, target branch/ref, and operation;
5. resolves the requested source revision to an exact Git object SHA;
6. records those non-secret values as auditable capability attributes.

Example policy:

```json
{
  "default": "deny",
  "rules": [
    {
      "capability": "github.git",
      "action": "push",
      "attributes": {
        "organization": "acme",
        "repository": "demo",
        "target_ref": "refs/heads/feature/x"
      },
      "decision": "allow"
    },
    {
      "capability": "github.git",
      "action": "force-push",
      "attributes": {
        "organization": "acme",
        "repository": "demo",
        "target_ref": "refs/heads/main"
      },
      "decision": "require-approval"
    }
  ]
}
```

Policy attributes are exact-match conditions. A rule may omit attributes it does not constrain.

## Exact source enforcement

Approval is not granted to a moving branch name. Hacocoon resolves the source to a SHA before policy/approval and the provider pushes that exact SHA:

```text
<approved SHA>:<approved refs/heads/...>
```

The provider re-reads and re-normalizes the GitHub remote immediately before the push. If the remote/repository/target no longer matches the approved request, the capability is stale and the push is refused.

Force pushes use `--force-with-lease`, not raw `--force`. The remote ref SHA observed before approval becomes an approval attribute and is checked again before execution. A changed remote ref invalidates the request.

## Credentials

No `GH_TOKEN`, GitHub PAT, SSH private key, credential-helper plaintext, or authorization header is copied into the Environment, Hacocoon state, policy request, or audit log.

The broker invokes host-side Git after authorization. Authentication therefore comes from the host's configured Git authentication path (for example a host credential helper or a future GitHub App-backed host adapter) and remains outside the untrusted Environment.

GitHub App token minting can be added behind this same provider boundary when deployment conditions make it useful; it is not required to weaken the boundary by injecting tokens into the Environment.

## Audit

Audit events include non-secret attributes such as:

- organization;
- repository;
- target ref;
- exact source SHA;
- remote name;
- expected remote SHA for force-with-lease.

Request `Parameters` remain excluded from audit by design.
