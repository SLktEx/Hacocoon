# Packer Base builds

[日本語](packer-base-builds.ja.md) | English

Status: partial on the development candidate. Actual Packer execution is wired
through ordinary disposable Environments; installed acceptance remains separate.
The existing [Base lifecycle](base-images-and-custom-environments.md) owns image
publication, revisions, retention and reviewed deletion.

## Build a reusable tool

Run from the Linux/WSL client with the example directory available locally:

```bash
haco base build --name my-tools --from haco/ubuntu-26.04 examples/packer
haco base inspect my-tools
haco env create --base my-tools --workspace managed:my-project dev
haco ssh setup dev
ssh haco-dev my-tool
```

The [example](../../examples/packer/base.pkr.hcl) uses real HCL2 and
`provisioner "shell" { script = "setup.sh" }`. Keep the shell script as a separate
file. No Hacocoon JSON definition is required. `--from` may be omitted for the
normal default Base. Options precede the directory. The final tool prints
`hello-from-packer`. Existing Environments retain their original Base revision
when another build moves `my-tools` to a new image.

Packer 1.16.0 runs `fmt`, `init`, `validate`, then `build` inside the builder.
Formatting changes only the staged copy. HCL evaluation, variables, plugins,
external scripts, `shell-local` and post-processors all run with guest authority.
Use ordinary `variables.auto.pkrvars.hcl` files for variable values. The example obtains
the guest-local SSH port/key through variable defaults: Packer's
[env function](https://developer.hashicorp.com/packer/docs/templates/hcl_templates/functions/contextual/env)
is valid there, not directly in a source block.

The built-in [null builder](https://developer.hashicorp.com/packer/docs/builders/null)
provisions the existing builder over loopback SSH. Its lack of an artifact is
expected: Hacocoon subsequently stops that same owned Env and publishes its
rootfs through the canonical Incus Base lifecycle. Running another builder or
post-processor does not authorize Host resources or change what is published.

Standalone `packer build` remains a guest-local Packer operation. To use this
example independently, supply your own guest-local SSH port and private-key path
as `haco_packer_port`/`haco_packer_key` variables inside the Env. It provisions
that guest and does not register a Hacocoon Base. The managed publication entry
is `haco base build`; no Host-side standalone Packer launcher is provided.

## Dependencies and source files

Packer is optional for ordinary Base selection and creation. A Packer build
prepares Python 3, CA certificates and OpenSSH in its disposable Ubuntu builder
when Python/OpenSSH tools are missing, using ordinary `apt-get` operations.
Custom Bases can provide these tools themselves; otherwise a Base without apt
fails the dependency stage. Nothing is installed on the Host by this plugin.

The guest downloads the official Linux amd64/arm64 Packer archive from
`releases.hashicorp.com` and verifies a pinned SHA-256 before writing the fixed
executable. Archive and executable sizes are bounded; archive paths are never
extracted. Package/Packer/plugin downloads keep the normal proxy and approval
path. A denied download fails; the build does not inject an allow rule or bypass
the network guard. A new build downloads into a fresh guest-local directory;
cross-build package/Packer caches are not implemented by this slice.

The client copies at most 128 regular files, totaling 512 KiB. Paths use ASCII
letters, digits, underscores, dots and hyphens, with at most eight components
and 240 characters; each component starts with a letter, digit or underscore.
At least one `.pkr.hcl` file must be at the directory root. Dot-prefixed entries
(including `.git` and `.env`) are skipped. Other invalid names, symlinks,
hardlinks, special files, collisions and oversized contexts are refused.
Directory/file identities are pinned while reading; no Host directory is mounted
in the builder. This is a script/configuration context, not a large-repository
upload mechanism. Keep credentials out of ordinary source files too: selected
files are deliberately supplied to the untrusted builder.

Linux and WSL read the context; native non-Linux context collection is unsupported.
Windows users invoke the Linux command through their normal WSL/Host entry.
Projected Windows-file and installed Windows-to-WSL acceptance must be recorded
separately from Linux component tests.

## Results and failure

`--json` returns Base identity/revision, state and any retained builder name.
Dependency, preparation, formatting, initialization, validation and provisioning
failures report a stage. `--output` additionally reveals the failed stage's
private stdout/stderr, bounded to 16 KiB each with truncation flags in JSON.
Human output is quoted to prevent guest terminal controls from posing as prompts.
Output may include script contents or secrets written by the script; it never
enters the controller's error/log chain. Successful stage output is not retained.

Cancellation and pre-publication failure use exact-owner temporary-Env cleanup.
Cleanup ambiguity retains ownership and reports recovery-required. Publication
uncertainty retains the builder and native image evidence for review. A published
Base survives a later builder-cleanup failure. Retry starts a fresh build; it is
not automatic replay. The ephemeral SSH keys, Packer files and build context live
under `/run/hacocoon/packer` and are removed by the existing instance cleanup
before publication. This is not a sanitizer for arbitrary secrets placed
elsewhere by a user script.

The legacy `haco base build base.json` form remains a migration path with bounded
`name`, optional `from`, and `run`; do not convert HCL to that format. The private
control request permits exactly one provisioning engine. Both paths reuse the
same lifecycle service, leases and native catalog. See
[ADR 0075](../adr/0075-guest-packer-provisioning.md).
