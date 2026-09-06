# Managed repository workflow

Status: **implemented** for the WSL PoC. Real-host results are recorded separately
in [implementation status](../IMPLEMENTATION_STATUS.md). This guide covers one
existing GitHub branch, the default Base and one Environment. It uses the
[Git authority contract](../design/git-and-github-capability.md) and
[ADR 0008](../adr/0008-managed-repository-workspaces.md).

## Prepare the trusted Host

Install a branch-built Windows ZIP using its BAT as described in
[Windows/WSL bootstrap](../WINDOWS_WSL_BOOTSTRAP.md), then enter it normally with
`wsl -d Hacocoon`. Interactive entry opens trusted `haco-host`. Run `haco doctor`
and `haco version --json`. Product commands in this Host and on the WSL Physical
Host address the same controller. The Host has no raw Incus socket or daemon.

Install Git and GitHub CLI inside trusted `haco-host` and authenticate there:

```bash
apt-get update
apt-get install -y git gh
gh auth login --hostname github.com --git-protocol https
haco repo clone --branch my-branch sample https://github.com/OWNER/REPO.git
haco workspace create --repo sample sample-work
haco env create --workspace managed:sample-work sample-dev
haco git connect sample-dev
haco env list
```

The branch must already exist upstream. Each ID is a lowercase name with digits
and hyphens. The source repository is under `/var/lib/hacocoon-repos/sample`
inside trusted `haco-host`. Workspace creation uses an Incus-owned Btrfs custom
volume copy, including an independent `.git`, rewrites its upstream to
`haco://sample`, and detaches it from the trusted Host before making it available
to an Environment. Failed creation keeps an ownership/recovery record and does
not remove data. Diagnose it before choosing a new ID or retrying; automated
recovery is deferred.

## Multiple repositories in one Workspace

Register each upstream separately, then create an immutable collection:

```bash
haco repo clone --branch first-branch first https://github.com/OWNER/REPO.git
haco repo clone --branch second-branch second https://github.com/OWNER/REPO.git
haco workspace create --repo first,second both
haco env create --workspace managed:both both-dev
haco git connect both-dev
```

Inside the Environment, work in `/workspace/first` and `/workspace/second`.
Each has its own Incus Btrfs copy and `.git`. Ordinary fetch, pull, commit and
approved push apply independently to its registered remote and branch. Add
Policy rules for both repository IDs. The one canonical Workspace lease owns
the complete collection; members cannot be leased separately. Files outside
the repository mounts belong to the disposable Environment root filesystem.
Changing collection membership and partial-creation recovery are deferred.
See [ADR 0010](../adr/0010-multiple-repositories-per-workspace.md).

## Configure narrow Policy

Policy is an ordinary administrator-owned file on the WSL Physical Host,
`/var/lib/hacocoon/policy.json`. Preserve existing rules. Add rules for the
registered Environment/repository, for example:

```json
{
  "default": "deny",
  "rules": [
    {
      "capability": "git.repository", "action": "fetch",
      "environment": "sample-dev", "resource": "https://github.com/OWNER/REPO.git",
      "attributes": {
        "repository": "sample", "remote": "https://github.com/OWNER/REPO.git",
        "target_ref": "refs/heads/my-branch", "old_oid": "*", "new_oid": "*", "operation_id": "*"
      },
      "decision": "allow", "reason": "Read the registered development branch"
    },
    {
      "capability": "git.repository", "action": "push",
      "environment": "sample-dev", "resource": "https://github.com/OWNER/REPO.git",
      "attributes": {
        "repository": "sample", "remote": "https://github.com/OWNER/REPO.git",
        "target_ref": "refs/heads/my-branch", "old_oid": "*", "new_oid": "*", "operation_id": "*"
      },
      "decision": "require-approval", "reason": "Review the fixed push proposal"
    }
  ]
}
```

Environment package downloads need separate, exact `network.egress` allows for
the configured Ubuntu archive hosts. Follow [egress authorization](../EGRESS_AUTHORIZATION.md);
do not disable the Environment network guard. Git upstream traffic runs inside
the trusted Host and does not require exporting credentials or an authenticated
proxy into the Environment.

## Develop over standard SSH

Generate a client-owned key with `ssh-keygen`. Put only its public key in trusted
`haco-host`, then run:

```bash
haco env ssh --key /root/client.pub --port 2222 sample-dev
```

Use the returned user and port with an ordinary SSH client on Windows
or the WSL Physical Host. Check the server fingerprint through the trusted
provider before accepting it on first connection (for example, the administrator
can read `/etc/ssh/ssh_host_ed25519_key.pub` using `incus exec` on that exact
Environment and compare `ssh-keygen -lf` output). The loopback address refers to that Physical Host,
not to the loopback of `haco-host`. Keep the private key on the SSH client.
Record the returned connection ID for disconnect. In the SSH session:

```bash
cd /workspace
export http_proxy=http://169.254.254.1:18080
export https_proxy=$http_proxy
apt-get update
apt-get install -y git
git config user.name 'Your Name'
git config user.email 'your-address@example.com'
git fetch origin
git pull --ff-only
# Edit files, then run the repository's build/test commands.
git add <files>
git commit -m 'Describe the change'
git push
```

The current Standard proxy uses the credential-free URL shown above. The
existing Incus exec environment receives it automatically; an SSH login needs
these ordinary shell exports. They do not grant network authority: the proxy
still requires the matching Environment Policy. Automatic SSH shell setup is
deferred.

While push waits, use a second trusted Host terminal to run `haco git pending`.
Review the repository, remote, ref, old/new OIDs and summary. Run
`haco git approve <id>` or `haco git deny <id>`. Each decision applies only to
that pending request. A denial must leave the remote unchanged; a subsequent
ordinary `git push` creates a new proposal. Verify the resulting upstream OID
using authenticated Git or GitHub from the trusted side.

## Select and switch Base

```bash
haco base list
haco env create --base haco/ubuntu-26.04 --workspace managed:both both-dev
# Work and commit in either repository; pushing first is not required.
haco env switch-base --base haco/ubuntu-24.04 both-dev
haco git connect both-dev
haco env ssh --key /root/client.pub --port 2222 both-dev
```

Switching keeps the managed Workspace volumes and all their Git state. It
replaces the Environment root filesystem, so install development packages
again if the new Base does not supply them. Files outside repository mounts
are discarded. SSH gets a new host key: verify and update the client's pinned
key through the trusted provider. If recreation fails, the error identifies
the retained Workspace and gives the next command. Interruption recovery is
deferred; inspect current state before retrying.

## Finish and retain work

Exit SSH, then use the trusted terminal:

```bash
haco env disconnect sample-dev <connection-id>
haco env stop sample-dev
haco env status sample-dev
```

Stop is graceful and keeps the Environment metadata, Workspace volume and lease.
Uncommitted, untracked and unpushed data remain in that volume. It is not a delete
or garbage-collection command. Inspect remote state after an ambiguous Git
failure before retrying. Large transfers, multiple refs, generalized
recovery and automatic SSH configuration are deferred.
