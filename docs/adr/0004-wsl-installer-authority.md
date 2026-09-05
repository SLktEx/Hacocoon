# WSL installer authority and ordinary user identity

Status: accepted for the current installer implementation; real-host acceptance remains separate.

## Context

The first BAT previously returned success after creating WSL, before installing Incus or Hacocoon. PR #441 proposed continuing in one invocation, but temporarily granting the ordinary user `NOPASSWD: ALL` created a root escalation window on reruns (#452). Creating a user outside Ubuntu's OOBE also left its metrics-consent dialog pending at normal WSL entry.

The reset CLI already uses a Physical Host controller socket owned by `root:hacocoon`, mode `0660`. Its WSL login alias does not require a sudo login rule.

## Decision

- Reuse #441's one-invocation continuation and encoded script transport, including explicit handling of Windows PowerShell 5.1 native quoting.
- Keep the default managed `hacocoon` account with password login locked at creation. WSL entry needs no password. Never reset an existing account's password on retry. `-InteractiveUserSetup` is the opt-in path for user-selected Ubuntu credentials and the normal OS dialogs.
- On the managed-account path, configure the documented WSL `oobe.command` and `oobe.defaultUid` keys after validating the ordinary account. Clear only Ubuntu's known account/metrics setup command, preserving unrelated distribution metadata. Do not opt the user into metrics collection. Unknown or ambiguous OOBE configuration fails before replacing the active file. Stage and rename the validated configuration atomically. Interactive setup keeps the upstream OOBE unchanged.
- Run the common Ubuntu installer through `wsl --user root --exec`, under the initiating Windows user's existing WSL authority. Windows UAC is scoped to distro creation.
- Pass the intended user name as `HACO_INSTALL_USER`; the common phase resolves and validates its actual UID/GID before privileged preparation. An explicit root UID/GID is rejected. A non-root installer may only select itself. Native ordinary/sudo invocation retains its own user identity.
- Delegate only the resolved workspace UID/GID to Incus subordinate-ID mapping. Add the same ordinary user to the controller access group. Incus admin access remains an explicit option.
- Do not create, replace, or remove sudo policy. Neither a bootstrap rule nor a login rule is needed. The candidate-replacement mechanism discussed in #452 is therefore unnecessary on the reset-CLI baseline; old installers are outside this change's compatibility scope.
- Preserve the distro on interruption or failure and fail before printing completion. Rerun the current BAT to continue. A normal restart must be tested before reinstall, so repair on rerun cannot hide failed startup.

## Rejected alternatives

Temporary broad sudo rules expose ambient root authority to existing processes. Replacing them with an operation list still creates a second policy surface for the installer. A fixed precreated user without updating WSL's first-launch contract leaves Ubuntu's account/metrics dialog pending and prevents immediate ordinary entry. Do not repair that gap by injecting consent or account fixtures in CI.

## Verification

Component tests exercise ordinary identity retention, invalid/root identity rejection, default/opt-in setup, current-installer rerun, known/unknown OOBE transformation, and native argument transport. Windows E2E uses the packaged BAT with the shipped `-UseCachedWslImage` option and ordinary `wsl -d Hacocoon` entry. Root assertions are read-only; they must not prepare accounts, sudoers, networks, or mounts. Trusted-host file preservation does not establish Environment/Workspace preservation. The reset product CLI's missing lifecycle commands remain a separate acceptance dependency.

WSL's first-launch keys are described in [Microsoft's distribution configuration documentation](https://learn.microsoft.com/en-us/windows/wsl/build-custom-distro); the [WSL init implementation](https://github.com/microsoft/WSL/blob/master/src/linux/init/init.cpp) treats an empty OOBE command as success and applies the configured default UID.

See [Windows installation](../WINDOWS_WSL_BOOTSTRAP.md), [trusted Host](../design/trusted-host.md), and [implementation status](../IMPLEMENTATION_STATUS.md).
