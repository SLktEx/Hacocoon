# ADR 0009: Limit Windows interoperability to the trusted Host

Status: accepted  
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
