# Implementation Reference Notes

Status: non-normative implementation references; re-check at implementation time.

## Local Incus/Btrfs

- Incus-managed loop-backed storage pools can be grown by increasing the pool `size`, but Incus documents that managed loop pools can only grow, not shrink.
- Incus Btrfs pools can also use an existing block device/loop file as `source`; Hacocoon's managed local-image path must keep outer-image lifecycle below the Storage boundary.
- Loop-backed storage is slower than a dedicated block device and usually does not physically shrink just because files/instances are deleted.

Authoritative references:
- https://linuxcontainers.org/incus/docs/main/howto/storage_pools/
- https://linuxcontainers.org/incus/docs/main/reference/storage_btrfs/
- https://linuxcontainers.org/incus/docs/main/explanation/storage/

## QCOW2 shrink

QEMU requires the filesystem/partitioning inside an image to be reduced before `qemu-img resize --shrink`; shrinking the outer image first can cause data loss. `qemu-img check` supports QCOW2 consistency checking. qemu-nbd attachment requires host support/privilege and may require the kernel NBD module, so Hacocoon must probe rather than assume it.

Authoritative references:
- https://www.qemu.org/docs/master/tools/qemu-img.html
- https://www.qemu.org/docs/master/tools/qemu-nbd.html

## AWS access vs EC2 runtime

AWS developer capability in v0.4 and Hacocoon deployment/runtime on EC2 in v0.7 are separate concerns. v0.4 should continue to use provider-native short-lived delegated credentials; v0.7 must not expose Manager/host credentials through IMDS or filesystem mounts.

AWS documents the container credential provider variables used by the v0.4 design, including `AWS_CONTAINER_CREDENTIALS_FULL_URI` and `AWS_CONTAINER_AUTHORIZATION_TOKEN_FILE`, with support in AWS CLI v2 and major SDKs.

Authoritative reference:
- https://docs.aws.amazon.com/sdkref/latest/guide/feature-container-credentials.html

## GitHub CLI token boundary

GitHub CLI documents that `GH_TOKEN`/`GITHUB_TOKEN` are direct authentication token inputs and that `gh auth token` outputs a stored token. This is why the default Hacocoon integration does not inject a broad Manager token into the Session and treats token-revealing/raw escape paths as denied/unsupported.

Authoritative references:
- https://cli.github.com/manual/gh_help_environment
- https://cli.github.com/manual/gh_auth_token

## EC2/EBS

Amazon EBS Elastic Volumes supports increasing volume size but not decreasing it. A smaller EBS target therefore requires a new smaller volume and verified data migration/switch-over. Treat this as replacement, not in-place resize.

Authoritative reference:
- https://docs.aws.amazon.com/ebs/latest/userguide/ebs-modify-volume.html
