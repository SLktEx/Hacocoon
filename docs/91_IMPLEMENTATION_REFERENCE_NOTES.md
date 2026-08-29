# Implementation Reference Notes

Status: **non-normative** references; re-check upstream documentation at implementation time. Nothing in this file expands a release gate by itself.

## Incus local runtime

v0.1 uses Incus system containers as the first Environment implementation. Prefer a thin `incus` CLI adapter until a different API is clearly justified.

Authoritative upstream references:

- https://linuxcontainers.org/incus/docs/main/
- https://linuxcontainers.org/incus/docs/main/howto/instances_create/
- https://linuxcontainers.org/incus/docs/main/reference/devices_disk/

## Historical local-storage experiments

Earlier Hacocoon code explored Btrfs, loop-backed pools, raw images, QCOW2, growth, shrink, and compaction. **No current release gate commits Hacocoon to those backing formats or workflows.** Existing code is migration inventory only.

If a future Environment implementation creates a real storage requirement, write or update the relevant release specification/ADR first, then re-evaluate the upstream storage documentation at that time.

General Incus storage references:

- https://linuxcontainers.org/incus/docs/main/howto/storage_pools/
- https://linuxcontainers.org/incus/docs/main/reference/storage_btrfs/

## GitHub credential boundary

GitHub CLI accepts direct token inputs such as `GH_TOKEN`; exposing a broad parent token to an untrusted Environment defeats the capability boundary planned for v0.5.

References:

- https://cli.github.com/manual/gh_help_environment
- https://cli.github.com/manual/gh_auth_token

## AWS / EC2 / EBS

AWS integration is grouped with later remote/cloud work in v0.7 and builds on the generic capability foundation from v0.4.

Prefer provider-native short-lived credentials where possible.

Amazon EBS volumes can be increased but not decreased in place. If a future v0.7 design needs a smaller volume, it must use replacement plus verified migration/switchover rather than describing an in-place shrink.

References:

- https://docs.aws.amazon.com/sdkref/latest/guide/feature-container-credentials.html
- https://docs.aws.amazon.com/ebs/latest/userguide/ebs-modify-volume.html

## Orchestrator integration

Daintree and Rookery are reference examples only, not dependencies. Hacocoon should integrate through generic boundaries such as workspace paths, command execution, structured status/events, and—if justified—MCP rather than importing an orchestrator's internal task model.
