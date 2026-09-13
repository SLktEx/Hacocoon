# ADR 0019: Keep saved Host customization separate from project setup

Status: accepted
Date: 2026-09-07

## Context

Roadmap C2 requires user-selected CLI tools, scripts and dotfiles in the trusted
logical Host, with saved steps that can be changed and replayed after recreation.
Project-controlled setup belongs to Environment execution and must not obtain Host
authority by being discovered automatically.

## Decision

Extend the existing controller-owned setup operation with an explicit script
update or clear request. The product client reads the selected regular UTF-8 file;
the controller never accepts a caller-selected Physical Host source or executable
path. There is no automatic discovery in a repository or Workspace.

After canonical Host preparation, persist the recipe atomically under the
controller's private configuration, then pass it through stdin to Bash in the
verified owned `haco-host`. The working directory is `/root`. Neither the script nor
its arbitrary output enters process arguments, environment variables, application
logs or Environment provisioning. The Incus adapter owns this target-specific
execution; Core gains no editor or shell customization policy.

Historically, plain `haco setup` replayed the saved snapshot. The incarnation rule
below supersedes that behavior. Replacing the recipe is explicit;
clearing it does not execute it. Failure retains the recipe and reports failure
without undoing arbitrary user changes. Recipes must tolerate replay. Setup remains
serialized and bounded even when the caller disconnects. Separate process locking
protects the stored recipe during execution. The Incus adapter uses a fixed transient
systemd unit: its independent runtime deadline and control-group termination bound
surviving descendants after controller loss, and its fixed name refuses overlap.
The script runs without a login shell or additional controller-routing environment.

Configuration files must be private, owned regular files with one link. Confined
filesystem access, no-follow opens, bounded reads, atomic replacement and directory
sync protect persistence. Unsupported platforms reject this store rather than
dropping its isolation requirements.

## Consequences and rejected alternatives

- A repository hook cannot silently become trusted Host code.
- Saved recipes survive logical Host recreation through controller setup; they are
  not retained solely on the disposable Host root filesystem.
- No second local composition path or caller-selected Incus target is introduced.
- This is not Environment project setup, a credential-copy mechanism, automatic
  rollback, or a generic background execution API.
- Low-level provider readiness operations do not replay user scripts. Controller
  shell entry now composes canonical setup as described below; its installed
  recreation acceptance remains distinct from component tests.

## Incarnation-bound application and private results

Issue #595 extends the original recipe implementation. Controller setup and Host
shell entry now share mandatory provisioning followed by automatic customization.
The private result record binds application to the verified provider incarnation
(`volatile.uuid`) and script digest. Durable running intent precedes execution;
only confirmed execution and unchanged identity produce success. A successful
incarnation skips automatic replay; failed or unknown completion requires explicit
`haco setup --reapply-script` or a new `--script` update. Recreation applies the
same saved snapshot after mandatory provisioning. Existing recipes without a
record get one initial application. Low-level provider ensure still owns no hook.

`--reapply-script` performs only user customization, while `--script-result`
retrieves the last private result without provisioning or execution. Neither is an
Environment option. The result holds separately bounded stdout/stderr and exit
status, never raw provider error strings. Raw output appears only in explicit
result inspection, never the application journal or stage diagnostics. A crash
can leave only running intent; output durability is promised on completed record
replacement, not across arbitrary process loss. Clearing disables future runs
without deleting the previous result or undoing effects.

Rejected alternatives: replay on every login/upgrade; treat a missing completion
as safe to retry; store completion only in disposable Host rootfs; log script
output; accept Physical Host script paths in controller requests. Windows files
are read by the Linux client using WSL path resolution and newline normalization.
The script retains existing trusted Host authority but gains no new Physical Host
authority. The incarnation is read before/after execution; out-of-band privileged
provider mutation is not an untrusted workload API.
