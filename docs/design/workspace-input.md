# Import a local Git working directory

[日本語](workspace-input.ja.md) | English

Status: implemented candidate for the Linux/WSL client. Native acceptance is
separate from repository implementation.

## Use

Register the intended upstream using `haco repo clone`. Its current Host-owned
routing supplies Git connectivity; the input cannot choose a Host path or import
credentials. Then, from the client that can read the source directory:

```bash
mkdir task
haco workspace import --repo sample --name task --path ./task ../linked-worktree
haco open ./task
```

The destination directory holds the existing Workspace reference. The source
checkout/linked worktree is copied into a new independent managed volume and is
left in place. It is not mounted into the Env. Stop editors/builds/Git writers
while capturing. Source changes are not synchronized after import. The new Env
uses the normal Base and OCI selection (`--base`, `--oci`, default auto); local
input does not import an OCI Store. Later stopped forks preserve retained work.

Working files (including untracked/ignored files), the selected HEAD/index and
independent copies of repository objects/refs are preserved. Host Git config,
credentials, hooks, reflogs and other worktree administration are excluded. A new
config points only to `haco://<registered-name>`. Normal read/approved push rules
still apply, including main confirmation. Choosing a registered source explicitly
chooses the intended upstream; it does not assert that local data came from it.

## Boundaries and failures

The client reads source files without executing source Git/config/hooks. It
verifies linked-worktree directory/backlink relationships, refuses symlinked Git
metadata and never follows alternate object paths. It sends a provider-neutral
tree as bytes through the same upload framing/cancellation/checksum receipt used
by Environment transfer. The management endpoint alone accepts the operation;
clients cannot submit a controller source path or native configuration.

Standard uses current registered routing and the existing import creation,
ownership/publication and cleanup transition. Incus privately converts and
validates the complete tree before native import and supplies new ownership.
UID/GID become Env root; ordinary permission bits and confined relative symlinks
are retained. No source ownership, xattrs, device files or special privileges are
replayed. A failure/unknown result retains its reference and exact owned recovery
records; reopening does not blindly repeat an import or overwrite an existing work.

Initial support is SHA-1, ordinary and linked Git working directories, packed
refs and split indexes. Sparse/partial clones, alternate object stores, nested
repositories/submodules and special files are refused. Input is bounded to 64 GiB
and one million entries; provider staging can impose a smaller bound. Directory
and file changes during reads are checked, but this is not an atomic live-filesystem
snapshot. Large-repository speed/storage measurements remain deferred. See
[the ownership decision](../adr/0098-independent-worktree-input.md).
