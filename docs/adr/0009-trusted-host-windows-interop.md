# ADR 0009: Limit Windows interoperability to the trusted Host

Status: accepted; implementation updated 2026-09-07  
Date: 2026-09-06

The WSL Physical Host remains the sole controller and Incus owner. The explicit
administrator operation `scripts/setup-wsl-host-interop.py` projects existing
DrvFs drive roots, read-only `/init` and the WSL interop socket directory into
the exactly marked trusted `haco-host`. It does not change shared profiles,
Environment devices, network guards, Incus authority or controller ownership.
Device collisions and unowned instances fail closed. Windows user ACLs still
govern file access. Windows programs have that user's Windows authority; these
devices must never be inherited by an Environment or embedded in a Base.

Use `/init /mnt/<letter>/path/to/tool.exe <arguments>` in the trusted shell.
Explicit interpreter invocation avoids changing kernel binfmt handlers or
requiring a privileged container. Setup resolves the WSL socket identity;
repeat setup after a WSL restart if it changes. Hotplug, reconnection and
universal application compatibility are deferred.

Rejected alternatives are shared profiles, drive mounts on Environments,
privileged containers and a second guest controller. None is needed here.

## Native WSL continuation (2026-09-07)

The original manual `/init` invocation above is historical acceptance, not the
current UX. A fresh main installation confirmed direct absolute `.exe` execution
works once the existing mounts are projected, using WSL's existing binfmt
registration. Normal install/setup now performs projection and passes only
WSL-converted Windows PATH entries selected against actual DrvFs mounts. The
stable `1_interop` socket path avoids recording a transient session PID.
No custom launcher, extra binfmt registration or Linux PATH copy is introduced.
See the current [trusted Host contract](../design/trusted-host.md#windows-interop).
The [WSL interop description](https://wsl.dev/technical-documentation/interop/)
explains the native interpreter/socket mechanism.
