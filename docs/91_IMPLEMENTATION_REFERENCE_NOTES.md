# Implementation Reference Notes

Status: **non-normative** references; re-check upstream documentation when changing the relevant implementation. Nothing in this file expands product scope or creates a compatibility guarantee by itself.

## Incus local runtime

Incus system containers remain the default local Environment implementation. Prefer a thin `incus` process boundary unless a different API is clearly justified by testing, reliability, or a concrete provider need.

Authoritative upstream references:

- https://linuxcontainers.org/incus/docs/main/
- https://linuxcontainers.org/incus/docs/main/howto/instances_create/
- https://linuxcontainers.org/incus/docs/main/reference/devices_disk/

Real Incus acceptance is environment-dependent and must be reported separately from unit/process-boundary tests.

## Historical local-storage experiments

Earlier Hacocoon code explored Btrfs, loop-backed pools, raw images, QCOW2, growth, shrink, and compaction. **No current architecture contract commits Hacocoon to those backing formats or workflows.** Existing code is provider detail or migration inventory unless a current design document explicitly promotes it.

If an Environment implementation creates a real storage requirement, update the relevant specification/ADR first, then re-evaluate upstream storage behavior.

General Incus storage references:

- https://linuxcontainers.org/incus/docs/main/howto/storage_pools/
- https://linuxcontainers.org/incus/docs/main/reference/storage_btrfs/

## GitHub credential boundary

The v0.5 implementation uses a brokered host-side Git/GitHub capability path rather than treating broad parent credentials as ambient Environment state.

GitHub CLI can consume token inputs such as `GH_TOKEN`, but exporting a broad parent token into an untrusted Environment would defeat Hacocoon's capability boundary and must not be used as a shortcut.

References:

- https://cli.github.com/manual/gh_help_environment
- https://cli.github.com/manual/gh_auth_token

## AWS / EC2 / EBS

The v0.7 implementation adds provider-neutral Environment routing, an experimental/default-off EC2 provider, a narrow brokered AWS read capability, and EBS replacement/recovery mechanics.

Real AWS/EC2/SSM/EBS acceptance remains a separate environment-dependent gate. Fake AWS process tests prove local construction, sequencing, parsing, policy flow, and cleanup behavior; they do not prove IAM, networking, AMI, SSM, or real EBS behavior.

Prefer provider-native short-lived credentials where possible.

Amazon EBS volumes can be increased but not decreased in place. Shrink-like behavior therefore requires replacement plus verified migration/switchover rather than an in-place EBS shrink.

References:

- https://docs.aws.amazon.com/sdkref/latest/guide/feature-container-credentials.html
- https://docs.aws.amazon.com/ebs/latest/userguide/ebs-modify-volume.html

## Orchestrator integration

Daintree and Rookery are reference examples only, not dependencies. Hacocoon integrates through generic boundaries such as workspace paths, command execution, structured status/events, and—if justified—MCP rather than importing an orchestrator's internal task model.

Agent scheduling, model choice, task DAGs, retries, budgets, and development approval remain outside Hacocoon.

## Compatibility note

Hacocoon is pre-1.0. These references explain implementation choices; they do not freeze the current CLI, API, state format, provider interface, or configuration. Re-check upstream behavior when a breaking change or provider update changes the assumptions recorded here.
