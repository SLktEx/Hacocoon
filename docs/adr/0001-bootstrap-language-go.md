# ADR 0001: Use Go for the current bootstrap implementation

Status: provisional  
Date: 2026-08-29

## Decision

Implement the current Hacocoon bootstrap primarily in Go.

This is not a permanent product-level promise that every future component must be Go. Architecture boundaries are stronger than the language choice: Workspace, Environment, Execution, Capability, Policy, and Client concerns must remain separable from concrete tools and providers.

Small shell scripts are acceptable at OS/tool integration edges when they are simpler than equivalent Go, but they must not become a second Core or bypass Hacocoon's security boundary.

## Implementation style

- Prefer a small functional/declarative core with imperative adapters at the edges.
- Prefer explicit data transformations and reconciliation over large conditional workflows.
- Keep one responsibility per package/function where practical.
- Prefer maps/strategies/interfaces where they represent a real replacement boundary; do not invent abstraction only to remove an `if`.
- Do not formalize provider/plugin interfaces merely because the roadmap names a future implementation; add a seam when testing or a second implementation makes it useful.
- Keep external commands, OS operations, Incus, and provider APIs behind narrow Ports & Adapters boundaries.
- Preserve UNIX-style composability: standard tools/protocols remain usable and Hacocoon should not wrap every developer command.
