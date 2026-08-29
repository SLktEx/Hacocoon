# Codex Handoff - Historical Hacocoon v0.1 to Current v0.1-v0.7

Status: Implementation instructions  
Audience: Codex or another coding agent entering a repository with historical v0.1 context

## 1. Critical context

Historical documents are input, not current release scope. The authoritative order is:

```text
0.1 Local Foundation
0.2 Developer Workspace
0.3 Security Framework + Git
0.4 External Capabilities
0.5 Local GUI + IDE
0.6 Local Web + Interaction
0.7 Remote + EC2
```

Do not infer current scope from old version numbers.

## 2. First actions before editing

1. Read `00_REBASELINE_AND_ROADMAP.md`.
2. Read `00C_TERMINOLOGY_AND_BOUNDARIES.md`.
3. Read `00A_PLUGIN_ARCHITECTURE.md`.
4. Read `00B_SECURITY_ARCHITECTURE.md`.
5. Read `01_v0.1_LOCAL_FOUNDATION.md` completely.
6. Run current tests and record pre-existing failures.
7. Inventory historical features with `IMPLEMENTATION_STATUS_TEMPLATE.md`.
8. Keep current project version at v0.1 until the new v0.1 gate passes.

## 3. Inventory mapping

```text
Area                              New target
Incus lifecycle                   0.1
Btrfs/local image storage         0.1
storage grow/shrink/compact       0.1
systemd + containerd/nerdctl      0.1
repo cache/workspace              0.2
VS Code/SSH/local ports           0.2
Agent lifecycle                   0.2
push approval/security            0.3
safe Git Provider                 0.3
GitHub/gh                         0.4
AWS delegated access              0.4
Packer/Terraform AWS auth         0.4
Registry/network profiles         0.4
WSLg/IntelliJ                     0.5
Web UI/Browser Notification       0.6
code-server local web             0.6
remote Linux/Gateway              0.7
EC2 runtime/AMI                   0.7
EC2/EBS lifecycle                 0.7
```

## 4. Do not perform a sweeping rewrite

- preserve working behavior behind better boundaries;
- stop expanding deferred features while fixing the current gate;
- refactor only dependencies that keep the current version from being small/testable;
- add regression tests before replacing security/storage-sensitive logic;
- prefer incremental commits that compile/test.

## 5. v0.1 issue sequence

### 0.1-01 Baseline
Inventory commands/packages/tests and record current failures.

### 0.1-02 Runtime boundary
Find every direct Incus call used by Session lifecycle; put it under `runtime.incus`; add fake Runtime; remove concrete imports from Core tests.

### 0.1-03 Storage boundary
Find Btrfs/loop/image/mount code; put it under Storage/Block backend boundaries; add fake Storage; keep concrete fields out of generic Session state.

### 0.1-04 Local storage lifecycle
Implement probe/ensure/inspect/grow and non-destructive shrink planning before destructive shrink. Add exclusive operation/recovery state.

### 0.1-05 Host init/doctor
Idempotent local composition and precise diagnostics.

### 0.1-06 Base Session
Ubuntu 26.04, systemd, containerd, nerdctl, nested smoke.

### 0.1-07 Lifecycle/exec
`new/list/status/start/stop/rm/exec/shell`, correct exit semantics.

### 0.1-08 Safe storage shrink/compact
Implement inner-to-outer shrink, targeted Btrfs balance only as needed, backend verification, crash recovery. Never outer-truncate first.

### 0.1-09 Reconciliation
Stale instance/storage attach, partial create/delete/resize, concurrent operation exclusion.

### 0.1-10 Acceptance
One automated gate proving v0.1 without repo/GitHub/AWS/IDE/Web/EC2.

## 6. Later issue tree

### v0.2
repo identity/cache; Session-isolated workspace/Git metadata; private ingress using Manager auth; User Base; `.hacocoon/setup.sh`; SSH; VS Code; localhost preview; snapshot/clone/restore; AgentProfile/Codex.

### v0.3
Session-bound IPC; CapabilityRequest; policy matcher; ALLOW/ASK/DENY; exact approval/grant; audit; Git Provider; exact source SHA/ref/force/delete rules; agent waiting-for-approval.

### v0.4
Provider strategies; delegated credentials; AWS standard credential endpoint; AWS CLI/SDK; Packer/Terraform validation; GitHub safe operations/`gh` shim; Registry; network profiles. **Do not postpone this AWS access work to v0.7.**

### v0.5
GUI probe; GUIProvider; GUILease; WSLg; IntelliJ profile; `haco open`; optional GPU/audio.

### v0.6
Interaction API; local Web UI; Browser Notification; approval UI; code-server provider; local web routes; optional IDE notification extension.

### v0.7
remote-linux HostAdapter; remote Gateway/access integration; runtime.ec2; AMI; v0.7 EC2/EBS provisioning design gate; EBS grow and replacement-based shrink; orphan/recovery hardening.

## 7. Rules Codex must not violate

- no host HOME convenience mount;
- no parent credential injection;
- no per-tool Manager wrapper zoo;
- no UI/plugin self-authorization;
- no concrete provider imports spread through Core;
- no automatic public Session port exposure;
- no EC2/EBS dependencies in v0.1-v0.6 local acceptance;
- no EBS in-place shrink fiction;
- no outer image truncate before filesystem shrink;
- no full Btrfs balance by default when targeted relocation is sufficient;
- no destructive retry without observed-state reconciliation.

## 8. Definition of done for each issue

As applicable include:
- happy-path test;
- negative/security test;
- cleanup/retry/restart test;
- state migration test;
- no-secret logging check;
- `haco doctor` diagnostics for new prerequisites;
- documentation update;
- prior-version regression suite.

## 9. Stop condition

When uncertain, choose the option that preserves, in order:

1. Manager/Session trust boundary;
2. no parent credential leakage;
3. data integrity;
4. Session-local development freedom;
5. standard external protocol/interface;
6. tiny vendor-neutral Core;
7. deletion/replacement boundary;
8. operational simplicity.
