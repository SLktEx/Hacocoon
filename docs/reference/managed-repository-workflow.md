# Managed repository workflow

For work-oriented preparation, path-based reopen, independent data forks and
Base-independent recreation, see [open and branch a Workspace](../design/workspace-workflow.md).
It composes the repository/collection operations below without moving a local
source folder or sharing a Git index.

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
        "target_ref": "refs/heads/my-branch", "old_oid": "*", "new_oid": "*", "operation_id": "*",
        "update_kind": "fast-forward"
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

With exactly one prepared SSH connection, generate OpenSSH configuration from
the trusted terminal instead of transcribing its JSON fields:

```bash
haco env ssh-config sample-dev > sample.ssh
# On the SSH client, after pinning the host key as described above:
ssh -F sample.ssh -i /path/to/client-private-key haco-sample-dev
```

Transfer only this credential-free configuration to the SSH client if needed.
It enforces strict host-key checking. Supply a client-local known-hosts file
with `-o UserKnownHostsFile=/path/to/known_hosts` if using a dedicated pin file.
The loopback must address the controller's Physical Host; another WSL
distribution may have a different loopback namespace.

`haco env status sample-dev` displays the target Environment, state, Workspace
and Base as labeled text. Use `haco env status --json sample-dev` for scripts.

In the SSH session:

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

## Installed saved-approval acceptance

The locally packaged and normally installed `eb16300b6700` build passed Windows
native SSH and ordinary guest Git push through the Host broker on 2026-09-08.
The only upstream was `https://github.com/SLktEx/Hacocoon-test.git`, branch
`codex/stage-b-b-first-20260906`. `haco git approve --save ask-env <id>` returned
`saved_choice: ask-environment`, successful execution and complete audit; the
protected Policy file contained the matching creation identity and fixed ref.
Remote verification observed `3ca59c3a0b56f2c05287c289f17ae9f0ce41b416`.

After removing the temporary administrator push rule, the saved ask rule alone
prompted for the next commit, `26a7b664214242d663520d10f1f4a111a1dccaa8`.
Ordinary `haco git deny <id>` made push exit 1 as expected; the remote stayed at
`3ca59c3a0b56f2c05287c289f17ae9f0ce41b416`. This is successful denial acceptance,
not a successful second push. Other saved choices have repository integration
coverage, not this installed GitHub acceptance.

The test Environment was canonically stopped/deleted. Its Workspace and
repository `git-save-eb16300` retain the unpushed second commit. Test-only
Policy rules, saved decision and SSH entry/pin were removed; the original eight
rules and default deny were preserved. Initial SSH setup failed without package
egress permission; it passed after temporary, creation-bound Ubuntu archive
permissions were added. No guest received reusable GitHub credentials.

## Select a Base

Use `haco base list` and `haco env create --base haco/ubuntu-26.04
--workspace managed:both both-dev` (on one command line). `switch-base` is
currently disabled; Stage D or later will reconsider whether it is needed.
Ordinary Environment delete/create can reuse retained resources with another
Base. See the [Base contract](../design/base-images-and-custom-environments.md)
and [Persistent OCI Store](../design/persistent-oci-store.md).

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

For manual Windows native OpenSSH, including client-owned keys and trusted
host-key pinning, use the [Windows SSH procedure](windows-environment-ssh.md).

To return to the same work, use `haco env start sample-dev`, then reconnect SSH.
The existing root filesystem, Workspace and optional Store remain attached.
Repeated start is safe. A recovery-required ownership or network error must be
resolved before reconnecting; start never disables isolation to proceed.

## Find the development target

`haco env list` shows Environment names, their Workspace and Base. Use
`haco open sample-dev` to connect and `haco env status sample-dev` to inspect its
current runtime state. Scripts that consume the registered list use
`haco env list --json`.

## Remember a reviewed operation

Review `haco git pending`: current old/new commits and summary are separate from
`saved_scope`, which fixes future repository, remote, branch and operation.

```bash
haco git approve --save env <id>
haco git approve --save all <id>
haco git deny --save env <id>
haco git approve --save ask-env <id>
```

`env` applies to this creation only; `all` explicitly includes other/future
Environments. `deny --save all` saves global denial. `ask-env` / `ask-all` save
require-approval while approve/deny answers the current request separately.
Omitting --save changes no Policy.

Saved allow never overrides administrator deny/require-approval. If remembering
ordinary approvals is intended, use default require-approval for unspecified
requests instead of a mandatory ask rule on that scope; preserve required
restrictions. The example above intentionally always asks.

The same private administrator-owned policy.json stores generated saved_decisions.
Edit/remove the matching entry there to revoke it. Policy is read on each request
and rechecked before execution; invalid files fail closed. A saved_choice receipt
requires durable storage and audit. Later execution/transport failure may leave
Policy saved; inspect Policy and remote before retrying.

Existing hand-written push rules need update_kind=fast-forward, as shown above.
Rules without that field fail closed. The saved branch scope never grants force,
deletion or branch creation. See [ADR 0026](../adr/0026-reusable-git-approval-scope.md).

## Remove retained Workspace data

```bash
haco workspace list
haco env delete sample-dev
haco workspace delete sample-work
```

The final command displays the exact managed identity, members and references,
then asks for confirmation. It deletes every file and Git record in that Workspace,
including uncommitted, untracked and unpushed work. Source repositories, saved
snapshots and OCI Stores remain. Use `--yes` only to explicitly confirm the same
operation in automation. `list --json` provides machine-readable metadata.

A stopped Environment still holds its Workspace lease and blocks deletion.
Native Incus snapshots, backups or a snapshot schedule on any member also block
deletion before the registry changes state. Export or explicitly remove those
native saved objects and disable their schedule through Incus before retrying.
Independent Hacocoon snapshots remain preserved. A
failed deletion keeps its owned record visible as `deleting`; retry the same
command. Incomplete creation requires inspection and is refused. This does not
reclaim Windows VHDX allocation; capacity reclamation is separate roadmap work.

## Inspect and remove a retained source repository

```bash
haco repo list
haco repo list --json
haco repo delete sample
```

Deletion previews the source identity, owner, upstream, branch and Workspace users,
then asks for confirmation. `--yes` is available for deliberate automation. It
removes only the selected local Host source checkout. The remote GitHub repository,
Host credentials, OCI Stores and independent saved snapshots remain.

A referencing Workspace prevents source deletion, even when its Env is stopped or
absent: the Git broker still uses this source. Keep the source while that Workspace
is needed. Do not delete valuable Workspaces merely to clear a source dependency.
Native child snapshots/backups or uncertain ownership also block removal. Inspect
those retained objects; retries use the exact current owner and never guess paths.
