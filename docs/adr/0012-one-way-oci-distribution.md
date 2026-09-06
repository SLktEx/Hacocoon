# ADR 0012: Distribute OCI image archives into independent guest runtimes

Status: accepted  
Date: 2026-09-06

The OCI plugin owns one-way image save/load. The Incus adapter runs fixed
commands in the exactly owned trusted Host and managed Environment, using
separate instance-local Docker/containerd sockets. Core acquires no OCI
dependency. The controller endpoint remains available only to trusted clients;
it is never projected into an Environment. Complete source export precedes
guest import; the only returned result is transfer metadata, not guest output.

A bounded private temporary archive avoids loading a source export which
already failed. Raw archive extraction on the Physical Host, shared writable
content stores, management sockets, credentials, bidirectional synchronization
and Seed construction are rejected. Each side independently runs/stops its
container. Explicit operator nesting may support this optional workload but
must preserve unprivileged instances and the existing external network guard.

This PoC does not migrate a running container, external volumes or guest
changes back to the Host. Crash leftovers and broader runtime acceptance are
follow-up work.
