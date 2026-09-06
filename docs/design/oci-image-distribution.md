# One-way OCI image distribution

Status: implemented; real-host acceptance is tracked in
[implementation status](../IMPLEMENTATION_STATUS.md).

The optional OCI plugin provides:

```bash
haco plugin oci distribute --runtime docker --image example:dev my-dev
haco plugin oci distribute --runtime nerdctl --image example:dev my-dev
```

Run this from trusted `haco-host` or the Physical Host. Selecting the runtime
explicitly opts into this operation; neither runtime is required for Core or
ordinary Git development. No Seed is constructed. Install the chosen runtime
independently in trusted `haco-host` and the Environment first.

The controller resolves the Environment from authoritative state. Its Incus
adapter invokes the trusted Host's local `save`, stages a complete image archive
in a private controller temporary file (maximum 256 MiB), then streams `load`
to the Environment's local runtime. The archive is never extracted on the
Physical Host. Failure to save prevents load; failure to load is reported.
The result reports archive SHA-256 and byte count. Temporary files are removed
after success/failure; a process crash may leave a private temporary file.

Docker uses `/run/docker.sock`; nerdctl uses `/run/containerd/containerd.sock`
and namespace `default`, each **inside its own instance**. Save/load run with
a cleared environment and fixed socket arguments. Runtime configuration,
registry login credentials, control sockets, writable stores and container
execution state are not copied or mounted across the boundary. Guest output
is discarded, and no guest command is executed on the Host. Only image contents
chosen by the operator are distributed; container volumes are not included.

For nested OCI execution on this Incus PoC, the administrator on the Physical
Host may explicitly enable nesting on the two owned instances:

```bash
incus config set haco-host --project hacocoon security.nesting=true
incus config set haco-my-dev --project hacocoon security.nesting=true
```

Keep the unprivileged instance, existing devices, network guard and source
anti-spoofing rules. Do not enable privileged mode, expose Host management
sockets or disable AppArmor to make a runtime work. The manual setting is
needed again after Base replacement. See the
[Incus nested Docker instructions](https://linuxcontainers.org/incus/docs/main/faq/).

After distribution, use ordinary SSH and `docker run` or `nerdctl run` in the
Environment. For an offline smoke test, use `--network none`. Changes, stop,
and deletion of the guest container affect only that guest's runtime. Image
updates require another explicit distribution. Live-state migration, volumes,
automatic updates, interrupted-transfer recovery and broad image/runtime
compatibility are deferred. See [ADR 0012](../adr/0012-one-way-oci-distribution.md).
