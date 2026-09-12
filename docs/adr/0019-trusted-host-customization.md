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

Plain `haco setup` replays the saved snapshot. Replacing the recipe is explicit;
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
- Low-level provider readiness operations do not replay user scripts. Acceptance
  of implicit recreation outside controller setup remains separate from the
  implemented explicit setup/reconstruction path.
