# Hacocoon CLI reset and temporary `hacoq`

Hacocoon is rebuilding its product-facing CLI from the basic user workflow outward.

## Command names during the migration

- `haco` is the product-facing CLI. New user workflows are added here only when they are ready to be part of the supported product experience.
- `hacoq` is the previous `haco` implementation under a temporary migration-only name.

`hacoq` is **not** an advanced or expert CLI and is **not** a permanent public interface. No new product features should be added to it. It exists only so existing low-level functionality remains reachable while equivalent product workflows are rebuilt or intentionally removed.

The intended end state is to delete `hacoq` completely.

## Initial product CLI surface

The first reset slice intentionally exposes only standalone commands:

```text
haco --version
haco version
haco version --json
haco help
haco --help
```

These commands do not require Incus, the Hacocoon controller, `haco-host`, Hacocoon state directories, network access, or root privileges.

Unimplemented product commands fail clearly instead of falling back to the legacy runtime stack. The hidden `haco host ensure` / `haco host shell` subprocess bridge has been removed. The installer calls product `haco setup` through the existing controller. Legacy `hacoq host ensure` fails before composition; its local bootstrap pipeline has been removed.

## Controller-backed setup and diagnostics

`haco setup` prepares owned Host resources through the existing Physical Host controller. It accepts no source paths or repair/force options. Failed or interrupted attempts retain owned data, and an overlapping setup is rejected. See [setup contract](design/controller-client-transport.md#host-setup).

`haco doctor` and `haco doctor --json` now diagnose the same Physical Host through its controller from either execution domain. The command inspects runtime, configured storage, trusted-host/network ownership and trusted connectivity without repairs. It returns nonzero for failed, skipped, unavailable or malformed diagnostics. See [controller diagnostics](design/controller-client-transport.md#host-diagnostics) for the exact scope and limits.

## Managed repository development

Product `haco` now exposes controller-backed `repo clone`, `workspace create`,
`env create/list/status/ssh/disconnect/stop`, and `git connect/pending/approve/deny`.
These commands implement the initial independent-Workspace development journey;
they do not invoke `hacoq`. Follow the
[managed repository workflow](reference/managed-repository-workflow.md) for
ordinary SSH, Git fetch/pull, fixed-content push approval and retained shutdown.

## Packaged compatibility boundary

Release archives and installers temporarily contain both `haco` and `hacoq` inside the Linux/WSL runtime.

Fresh trusted-host setup receives only product `haco` and client-only `haco-host`. Legacy guest hacoq provisioning has been removed. Existing guest copies are not required by the product and are not used for setup.

There is no native Windows `haco` command. The Windows installer only provisions the dedicated Hacocoon WSL environment; product commands run inside that environment.

This reset does not preserve or clean up state from older installers. Pre-1.0 installer compatibility is intentionally out of scope; development and acceptance use the current installer contract only.

## Development rule

Do not implement new workflows by adding commands to `hacoq`, and do not make the new `haco` shell out to `hacoq` for ordinary product operations. Shared behavior should move behind reusable Go packages or controller APIs as each product command is rebuilt.

The WSL interactive-login alias is handled by the product `haco` binary itself. Interactive entry opens the trusted `haco-host` through the controller API directly; it does not invoke or depend on `hacoq`.
