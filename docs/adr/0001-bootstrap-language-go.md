# ADR 0001: Use Go for the current bootstrap implementation

Status: provisional  
Date: 2026-08-29

## Decision

Implement the current Hacocoon bootstrap in Go.

This is not a permanent product-level promise that every future component must be Go. The architectural boundaries are intentionally stronger than the language choice: Core contracts, Runtime/Storage seams, Security and Feature Plugin boundaries must remain replaceable.

## Implementation style

- Prefer a small functional/declarative core with imperative adapters at the edges.
- Prefer explicit data transformations and reconciliation over large conditional workflows.
- Keep one responsibility per package/function where practical.
- Prefer maps/strategies/interfaces where they represent a real replacement boundary; do not invent abstraction only to remove an `if`.
- Keep external commands and provider APIs behind Ports & Adapters boundaries.
- Preserve UNIX-style composability: standard tools/protocols remain usable and Hacocoon should not wrap every developer command.
