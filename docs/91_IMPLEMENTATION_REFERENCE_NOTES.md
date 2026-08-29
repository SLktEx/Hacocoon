# Implementation Reference Notes

Status: non-normative references; re-check upstream documentation at implementation time.

## Incus local runtime

v0.1 uses Incus system containers as the first Environment implementation. Prefer a thin `incus` CLI adapter until a different API is clearly justified.

Authoritative references:

- https://linuxcontainers.org/incus/docs/main/
- https://linuxcontainers.org/incus/docs/main/howto/instances_create/
- https://linuxcontainers.org/incus/docs/main/reference/devices_disk/

## Storage references

Historical Hacocoon code explored Btrfs, loop-backed pools, raw images, QCOW2, growth, shrink, and compaction. These are no longer v0.1 acceptance requirements.

If later retained behind a storage/environment adapter:

- Incus-managed loop storage pools can grow but their managed loop backing is not an in-place shrink primitive.
- Any filesystem/image shrink flow must reduce inner filesystem/partition structures before truncating an outer image.
- `qemu-img resize --shrink` must not be used before the contained data structures are safely reduced.

References:

- https://linuxcontainers.org/incus/docs/main/howto/storage_pools/
- https://linuxcontainers.org/incus/docs/main/reference/storage_btrfs/
- https://www.qemu.org/docs/master/tools/qemu-img.html

## GitHub credential boundary

GitHub CLI accepts direct token inputs such as `GH_TOKEN`; exposing a broad parent token to an untrusted Environment defeats the capability boundary planned for v0.5.

References:

- https://cli.github.com/manual/gh_help_environment
- https://cli.github.com/manual/gh_auth_token

## AWS / EC2 / EBS

AWS integration is now grouped with the later remote/cloud work in v0.7, built on the generic capability foundation from v0.4.

Prefer provider-native short-lived credentials where possible.

Amazon EBS volumes can be increased but not decreased in place. A smaller target requires a replacement volume and verified migration/switchover.

References:

- https://docs.aws.amazon.com/sdkref/latest/guide/feature-container-credentials.html
- https://docs.aws.amazon.com/ebs/latest/userguide/ebs-modify-volume.html

## Orchestrator integration

Daintree/Rookery are reference examples only. Hacocoon should integrate through generic boundaries such as workspace paths, command execution, structured status/events, and possibly MCP rather than importing their internal task models.
