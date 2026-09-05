# WSL installer authority and ordinary user identity

Status: accepted for the current installer implementation; real-host acceptance remains separate.

## Context

The first BAT previously returned success after creating WSL, before installing Incus or Hacocoon. PR #441 proposed continuing in one invocation, but temporarily granting the ordinary user `NOPASSWD: ALL` created a root escalation window on reruns (#452). Creating a user outside Ubuntu's OOBE also left its metrics-consent dialog pending at normal WSL entry.

The reset CLI already uses a Physical Host controller socket owned by `root:hacocoon`, mode `0660`. Its WSL login alias does not require a sudo login rule.

## Decision

- Reuse #441's one-invocation continuation and encoded script transport, including explicit handling of Windows PowerShell 5.1 native quoting.
- Complete Ubuntu's ordinary first-launch dialogs in the same BAT terminal. The user exits that initial Linux shell to continue installation. Do not replace distro OOBE, manufacture consent, or create an account outside that flow. A completed current installation keeps its existing non-root default user and skips this initial shell.
- Run the common Ubuntu installer through `wsl --user root --exec`, under the initiating Windows user's existing WSL authority. Windows UAC is scoped to distro creation.
- Pass the intended user name as `HACO_INSTALL_USER`; the common phase resolves and validates its actual UID/GID before privileged preparation. An explicit root UID/GID is rejected. A non-root installer may only select itself. Native ordinary/sudo invocation retains its own user identity.
- Delegate only the resolved workspace UID/GID to Incus subordinate-ID mapping. Add the same ordinary user to the controller access group. Incus admin access remains an explicit option.
- Do not create, replace, or remove sudo policy. Neither a bootstrap rule nor a login rule is needed. The candidate-replacement mechanism discussed in #452 is therefore unnecessary on the reset-CLI baseline; old installers are outside this change's compatibility scope.
- Preserve the distro on interruption or failure and fail before printing completion. Rerun the current BAT to continue. A normal restart must be tested before reinstall, so repair on rerun cannot hide failed startup.

## Rejected alternatives

Temporary broad sudo rules expose ambient root authority to existing processes. Replacing them with an operation list still creates a second policy surface for the installer. A fixed precreated user also does not complete Ubuntu OOBE by itself. Both are unnecessary when normal OS interaction and root-side preparation are available.

## Verification

Component tests exercise ordinary identity retention, invalid/root identity rejection, interrupted setup, current-installer rerun, and native argument transport. Windows E2E uses the packaged BAT, normal OS keystrokes, and ordinary `wsl -d Hacocoon` entry. Root assertions are read-only; they must not prepare accounts, sudoers, networks, or mounts. Trusted-host file preservation does not establish Environment/Workspace preservation. The reset product CLI's missing lifecycle commands remain a separate acceptance dependency.

See [Windows installation](../WINDOWS_WSL_BOOTSTRAP.md), [trusted Host](../design/trusted-host.md), and [implementation status](../IMPLEMENTATION_STATUS.md).
