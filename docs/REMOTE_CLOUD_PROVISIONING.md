# Remote / Cloud Provisioning Design Gate

Status: v0.7 implementation contract.

This document is the EC2/EBS provisioning design gate required by `07_v0.7_REMOTE_AND_CLOUD_RUNTIME.md`. It defines ownership, authority, workspace movement, cleanup, and recovery before a real AWS deployment is accepted.

## Boundary

The remote path preserves the same higher-level model as local Incus:

```text
Workspace -> Environment -> Execution
                 |
                 +-- runtime.incus
                 +-- runtime.ec2
```

The provider-neutral Environment router wraps provider identity inside the opaque runtime reference and dispatches lifecycle operations without adding cloud fields to Core domain values. Pre-v0.7 bare refs remain Incus-compatible. Core does not import AWS, EC2, S3, SSM, EBS, AMI, region, or device-specific packages and does not branch on `if ec2` or `if remote`.

`runtime.ec2` owns AWS-specific environment mechanics. `storage.ebs` owns EBS replacement mechanics. `aws.api` owns brokered AWS service authority. These are separate adapters and must not be collapsed into Core.

## Ownership table

| Resource / authority | Owner | Rule |
|---|---|---|
| Hacocoon state and policy | trusted host-side Hacocoon | never mounted into the Environment |
| Parent AWS credentials / AWS CLI credential chain | trusted host | never copied into EC2 workspace, Hacocoon state, capability Parameters, or audit |
| EC2 lifecycle | `runtime.ec2` on the trusted host | create, inspect, SSM execution, sync-back, terminate, cleanup |
| EC2 instance profile | EC2 Environment | narrowly scoped runtime identity; not a copy of host credentials |
| Workspace source directory | caller / WorkspaceProvider | Hacocoon consumes the selected Workspace; it does not take ownership of upstream Git/worktree lifecycle |
| Temporary S3 staging prefix | `runtime.ec2` | per Environment; deleted after successful termination/cleanup |
| Remote `/workspace` | EC2 Environment | materialized from the selected Workspace; RO/RW follows WorkspaceLease access mode |
| EBS replacement transaction | `storage.ebs` | adapter-owned journal and state machine; source volume retained |
| AWS external-service operation | `aws.api` capability provider | Policy/Approval/Audit precede host-side execution |

## Experimental EC2 opt-in

EC2 is disabled by default. Selecting it requires **both** `HACO_RUNTIME_PROVIDER=runtime.ec2` and the explicit v0.7 host/operator opt-in `HACO_EXPERIMENTAL_EC2=1`. Without the opt-in Hacocoon registers only a fail-closed placeholder for `runtime.ec2`; the real EC2 adapter is not constructed and no AWS CLI/network call is made. Merely having AWS credentials, AWS environment variables, or the AWS CLI installed never selects EC2.

The gate controls EC2 lifecycle operations, including access to pre-existing remote Environment state. Disabling it never destroys that state. Re-enable it explicitly before remote inspect/exec/shell/delete/recovery. The gate is independent of AWS credential presence, so an upgrade or preconfigured AWS shell cannot silently activate the experimental backend.

## EC2 creation flow

The initial implementation uses host-side AWS CLI operations and SSM rather than inbound SSH:

```text
host Workspace
  -> create tar archive
  -> upload input.tgz to per-Environment S3 prefix
  -> EC2 RunInstances
       - selected AMI / instance type / subnet / security groups
       - selected instance profile
       - IMDSv2 required
  -> wait for EC2 health
  -> wait for SSM managed state
  -> SSM bootstrap
       - download input.tgz using instance authority
       - extract under /opt/hacocoon/workspace
       - bind mount at /workspace
       - remount read-only when lease is RO
  -> persist one opaque routed runtime reference
```

A partial create is not considered successful merely because an instance ID exists. If staging, health, SSM readiness, bootstrap, or runtime-reference construction fails, Hacocoon attempts to terminate the partial instance and remove the staging prefix with a cleanup context independent of caller cancellation. Cleanup failures are returned, not hidden.

## Remote image contract

The configured AMI is an adapter contract, not a Core concept. The v0.7 EC2 path expects the image to provide:

- a working SSM Agent;
- AWS CLI usable by the instance role for the staging operation;
- `tar`;
- a Linux userspace suitable for `/workspace` bind mounts;
- `systemd` for the intended development environment.

The image must not contain copied Hacocoon host credentials. Image versioning/publishing may evolve independently behind `runtime.ec2`.

## IAM and credential scope

There are two different authority domains.

### Trusted host authority

The host-side AWS CLI performs control-plane operations such as EC2 lifecycle and brokered AWS capabilities. Its parent credential chain remains outside the untrusted Environment.

Hacocoon must not place these values into command arguments, workspace files, Environment variables, state JSON, or audit records:

- `AWS_ACCESS_KEY_ID`;
- `AWS_SECRET_ACCESS_KEY`;
- `AWS_SESSION_TOKEN`;
- copied `~/.aws` content;
- a broad parent role credential.

Where deployment permits, the host should itself use a provider-native short-lived role/session rather than a long-lived static key.

### Instance authority

The EC2 instance profile is separate and narrower. It should grant only what the remote Environment requires, principally:

- SSM managed-instance operation;
- read/write access to that Environment's temporary Workspace staging prefix when RW sync-back is enabled;
- no Hacocoon control-plane authority unless explicitly added by another capability design.

An instance role is not an authorization shortcut around Hacocoon Policy/Capability. External privileged operations that Hacocoon brokers remain host-side capability requests.

## Network model

The default EC2 path requires no inbound developer port and no public SSH key. Control and interactive access use SSM.

The deployment network must provide outbound/reachable paths needed for SSM and the selected S3 staging bucket, through NAT or provider endpoints as appropriate. AWS control-plane access remains on the trusted host.

Instances are launched with IMDSv2 token usage required. Security groups and subnet are explicit deployment configuration. Hacocoon does not silently open a broad ingress rule.

Port exposure beyond the local v0.3 loopback model requires a separately reviewed remote network/capability design; `runtime.ec2` does not pretend Incus local proxy forwarding works remotely.

## Execution and shell

Non-interactive command execution uses SSM `AWS-RunShellScript`. Hacocoon reconstructs the exact argv as a shell-quoted command under `/workspace`, waits for the command invocation, then reports the remote `ResponseCode`, stdout, and stderr as the provider-neutral `ExecutionResult`.

Interactive shell uses SSM Session Manager. No SSH private key is copied into the instance for this path.

## Workspace synchronization and deletion

Read-only and read/write Environments intentionally differ.

### Read-only

The input archive is materialized read-only. Delete may terminate the instance without a sync-back.

### Read/write

Delete is ordered to protect Workspace changes:

```text
remote /workspace
  -> tar output
  -> instance uploads output.tgz to S3
  -> host downloads output.tgz
  -> host restores through a sibling temporary directory and rename/swap
  -> only after successful restore: terminate instance
  -> wait terminated
  -> remove staging prefix
```

If the instance is not running when an RW Workspace needs synchronization, Hacocoon fails with `recovery-required` instead of terminating it and losing uncollected data.

If sync-back or host restore fails, termination is not attempted. A retry after an already-terminated instance skips sync-back and performs only remaining staging cleanup.

## EBS replacement contract

EBS is not modeled as a generic Core shrink operation. EBS does not shrink in place. A smaller target is an explicit adapter-owned replacement transaction:

```text
planning
  -> target-created
  -> target-attached
  -> migrated
  -> verified
  -> source-detached
  -> target-promoted
  -> activated
  -> complete
```

The operation journal records its phase and target identity. Before the source is detached, failures may clean up the replacement target. After source detach, ambiguous or failed transitions are marked `recovery-required`; Hacocoon does not attempt a destructive automatic rollback whose safety cannot be proven.

The source EBS volume is **never automatically deleted** by the replacement transaction. A later explicit retention/deletion policy may decide what to do with the preserved source after independent verification.

The migration implementation must provide `Preflight`, `Migrate`, `Verify`, and `Activate` operations appropriate to the filesystem/application. EBS volume mechanics and filesystem mechanics remain separate concerns.

## AWS capability boundary

The first v0.7 AWS capability is intentionally narrow and brokered. `aws.api` supports host-side read operations for:

- caller identity;
- one EC2 instance;
- one EBS volume.

Before execution, every authority-sensitive input is encoded in the existing policy-visible `Action` and exact `aws://...` `Resource`. For example, caller identity is `sts.get-caller-identity` on `aws://ap-northeast-1/identity`, while an instance read is `ec2.describe-instance` on `aws://ap-northeast-1/ec2/instance/i-...`. The provider re-parses and revalidates that scope before invoking the AWS CLI; no opaque Parameters or hidden Attributes select AWS authority.

This capability does **not** vend an AWS token into the Environment. Additional mutating AWS operations must be added as explicit capability actions with policy-visible authority; they must not be smuggled through opaque Parameters.

## Recovery rules

The remote implementation follows these fail-closed rules:

- incomplete EC2 creation -> attempt instance + staging cleanup and surface every failure;
- RW sync-back failure -> keep the EC2 instance instead of terminating it;
- unknown/non-running RW state requiring collection -> `recovery-required`;
- EBS failure before source detach -> target cleanup is allowed;
- EBS failure after source detach -> persist `recovery-required`, retain both volumes, require reconciliation;
- capability policy/approval/audit failure -> no unreviewed AWS authority is executed, using the v0.4 capability semantics.

## Test gate

The repository must keep distinct claims:

1. unit tests for routing, EC2 sequencing, quoting, sync-back, EBS phases, and capability scope;
2. process-boundary integration using executable `aws` shims through the real `os/exec` runner;
3. actual `haco` binary E2E against the same process boundary for remote lifecycle and brokered AWS capability;
4. **real AWS acceptance**, requiring an AWS account/VPC/AMI/S3/SSM/EBS setup, remains a separate environment-dependent gate.

A fake AWS process test proves Hacocoon command construction, policy flow, lifecycle ordering, parsing, and cleanup logic. It does not prove IAM policies, VPC endpoints, AMI behavior, AWS service availability, or real EBS/filesystem migration.
