# Hacocoon Master Index

> Historical filename note: this file is still named `Hacocoon_v0.1-v0.7_MASTER.md`, but the authoritative roadmap has progressed through **v0.8**.

This committed file is intentionally a lightweight index. Source documents under `docs/` are authoritative according to `docs/README.md`; do not edit a concatenated bundle as a source of truth.

The implementation progression currently reaches the v0.8 roadmap, while Hacocoon remains **pre-1.0** and may introduce breaking changes.

Read in this order:

1. `README.md`
2. `docs/README.md`
3. `CODEX_START_HERE.md`
4. `docs/00_REBASELINE_AND_ROADMAP.md`
5. `docs/00C_TERMINOLOGY_AND_BOUNDARIES.md`
6. `docs/00B_SECURITY_ARCHITECTURE.md`
7. `docs/IMPLEMENTATION_STATUS.md`
8. the relevant versioned release specs `docs/01_...` through `docs/08_...`
9. specialized design documents for the subsystem being changed
10. `docs/90_CODEX_IMPLEMENTATION_HANDOFF.md`
11. `docs/91_IMPLEMENTATION_REFERENCE_NOTES.md`

The v0.1-v0.8 specifications are versioned design contracts. They are not a declaration that the corresponding CLI/API/state/adapter surface is permanently frozen, and they do not replace `docs/IMPLEMENTATION_STATUS.md` for current repository reality.

For v0.8 client work, read `docs/08_v0.8_CLIENT_ADAPTERS_AND_VSCODE_INTEGRATION.md` together with `docs/CLIENT_ACCESS.md`. VS Code and other clients remain outside Core.

For an ad-hoc concatenated local bundle, run:

```bash
python tools/build_master.py
```

The generated bundle under `dist/` is convenience output only. If it disagrees with the source files, the source files win.
