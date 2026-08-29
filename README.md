# Hacocoon

**Pronounced: はこーん (ha-kōn)**

Hacocoon is an OSS **secure workspace runtime** for humans, developer tools, and coding agents.

Hacocoon does not own the IDE, Git workflow, worktree orchestration, or AI-agent scheduler. It accepts a workspace, places it in an isolated execution environment, runs commands there, and later adds narrowly-scoped capabilities and human approval at security boundaries.

```text
VS Code / Shell / Daintree / Rookery / other clients
                         |
                    Workspace
                         v
                  +-------------+
                  |  Hacocoon   |
                  | secure      |
                  | workspace   |
                  | runtime     |
                  +------+------+ 
                         |
                 EnvironmentProvider
                         |
                       Incus
```

## Current implementation target

The repository is being **rebaselined**. Historical code may contain functionality that belonged to the previous v0.1-v0.7 plan; its presence does not make that functionality part of the new v0.1 gate.

The authoritative release order is now:

1. **v0.1 Secure Workspace Runtime MVP**
2. **v0.2 Workspace Abstraction & Lease**
3. **v0.3 Client & Interactive Access**
4. **v0.4 Policy & Capability Foundation**
5. **v0.5 Git / GitHub Capability**
6. **v0.6 Agent & Orchestrator Integration**
7. **v0.7 Remote / Cloud Runtime & External Capabilities**

Read `docs/00_REBASELINE_AND_ROADMAP.md` and `CODEX_START_HERE.md` before extending the implementation.

## v0.1 definition of done

v0.1 is intentionally small:

```text
host workspace
    -> haco create
Incus system container with workspace mounted
    -> haco exec / haco shell
    -> haco delete
```

Target CLI:

```text
haco create --workspace <path> <environment>
haco exec <environment> -- <command...>
haco shell <environment>
haco delete <environment>
```

The first implementation uses an external path as the workspace and Incus system containers as the environment provider.

## Design rules

- Core owns only Hacocoon concepts: Workspace, Environment, Execution, CapabilityRequest, PolicyDecision, and ApprovalRequest.
- Core does not know whether a workspace is a Git repository, Git worktree, or ordinary directory.
- Incus, Git, GitHub, AWS, VS Code, Daintree, Rookery, storage backends, and cloud runtimes remain outside Core.
- Hacocoon does not choose Codex vs Claude, build an agent DAG, or optimize model budgets.
- Long-lived host credentials are not mounted into an execution environment for convenience.
- Human-in-the-loop inside Hacocoon is primarily a **security approval boundary**.
- Do not build a provider/plugin framework before a second implementation actually needs the seam.
- Implement one release gate at a time.

## Build and test

```bash
go test ./...
go build ./cmd/haco
```

Historical tests may need to be reclassified as the rebaseline is implemented. Do not delete useful code merely because it moved to a later release; isolate it behind the new boundaries or defer it without allowing it to expand v0.1.
