# Hacocoon

**Pronounced: ha-kōn**

⚠️ **Experimental:** Hacocoon is under active development and may introduce breaking changes without notice.

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
                 Environment boundary
                         |
                   Incus adapter
```

## Current implementation target

The repository is being **rebaselined**. Historical code may contain functionality that belonged to the previous plan; its presence does not make that functionality part of the new v0.1 gate.

The authoritative release order is:

1. **v0.1 Secure Workspace Runtime MVP**
2. **v0.2 Workspace Abstraction & Lease**
3. **v0.3 Client & Interactive Access**
4. **v0.4 Policy & Capability Foundation**
5. **v0.5 Git / GitHub Capability**
6. **v0.6 Agent & Orchestrator Integration**
7. **v0.7 Remote / Cloud Runtime & External Capabilities**

Read `docs/README.md` for documentation precedence, then `docs/00_REBASELINE_AND_ROADMAP.md` and `CODEX_START_HERE.md` before extending the implementation.

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

The first implementation uses a direct external path as the workspace and a concrete Incus adapter. v0.1 does not require a generalized provider/plugin framework.

## Design rules

- Core owns only stable Hacocoon concepts needed by the current design; later concepts are introduced in their release, not pre-built in v0.1.
- Core does not know whether a workspace is a Git repository, Git worktree, or ordinary directory.
- Incus, Git, GitHub, AWS, VS Code, Daintree, Rookery, storage backends, and cloud runtimes remain outside Core.
- Hacocoon does not choose Codex vs Claude, build an agent DAG, or optimize model budgets.
- Long-lived host credentials are not mounted into an execution environment for convenience.
- Human-in-the-loop inside Hacocoon is primarily a **security approval boundary**.
- Do not create a provider/plugin interface before testing or a second implementation makes the seam useful.
- Implement one release gate at a time.

## Build and test

```bash
go test ./...
go build ./cmd/haco
python tools/check_docs.py
```

`docs/IMPLEMENTATION_STATUS.md` records the current code reality. Historical tests and code may need to be reclassified during the rebaseline; preserve useful behavior without allowing old implementation surface to expand v0.1.
