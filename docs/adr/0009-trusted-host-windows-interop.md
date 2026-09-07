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

Revised fresh packages exposed two native lifecycle details: `1_interop`
points to the absolute `/run/WSL/<pid>_interop`, while the guest recreates `/run`
as tmpfs during boot. A direct Incus mount under `/run` is then hidden. Project
`/run/WSL` read-only at `/var/lib/hacocoon-wsl`; standard systemd tmpfiles restores
`/run/WSL` as a symlink to that directory after each boot. This preserves native
absolute socket resolution without a socket relay, launcher or stored session
PID. Foreign path collisions fail closed. A tmpfiles regression and the real
Windows restart test cover this layout.

Further combined acceptance observed the WSL VM's native registration disappear
while `/init` plus the projected socket still worked. The trigger is not proven.
[Ubuntu's binfmt explanation](https://ubuntu.com/wsl/docs/stable/explanation/binfmt/)
and [WSL systemd integration](https://wsl.dev/technical-documentation/systemd/)
describe native registration lifecycle and WSL's generated protection unit.
Setup leaves a healthy native handler untouched. Only when it is absent, setup
uses the root-owned WSL-generated `systemd-binfmt.service` integration and checks
that the native `/init` handler returned. Explicit disable or incompatible
handlers fail closed. Hacocoon never writes its own registration to `register`,
never installs an executable launcher and never changes another distribution.
External replacement of VM-wide binfmt state during an already-open session is
not silently monitored; use normal `haco setup` to reconcile it.