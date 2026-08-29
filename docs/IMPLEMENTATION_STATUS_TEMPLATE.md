# Implementation Status

Status date: YYYY-MM-DD.

This template records **current repository reality**. Roadmap versions are design/implementation milestones, not compatibility guarantees.

| Area | Current repository reality | Roadmap stage | Validation / next action |
|---|---|---:|---|
| Secure Workspace Runtime | | v0.1 | |
| Workspace identity / leases | | v0.2 | |
| Client / interactive access | | v0.3 | |
| Policy / Capability / audit | | v0.4 | |
| Git / GitHub capability | | v0.5 | |
| Agent / orchestrator integration | | v0.6 | |
| Environment routing | | v0.7 | |
| Experimental EC2 runtime | | v0.7 | |
| AWS capability | | v0.7 | |
| EBS replacement / recovery | | v0.7 | |
| Historical storage code | | historical/provider detail | |
| CI / race / vet / docs | | cross-cutting | |

Notes:

- Distinguish implemented code from real-provider acceptance.
- Record fake/process-boundary tests separately from real Incus or real AWS acceptance.
- Experimental/default-off providers must remain clearly marked as such.
- Record destructive deletions, state-format changes, and compatibility decisions explicitly.
- Hacocoon is pre-1.0; do not turn implementation presence into an accidental stability promise.
- When a breaking change is deliberate, record operator impact and any supported migration/recovery path.
