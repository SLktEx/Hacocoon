# Hacocoon v0.1-v0.7 Master Index

This committed file is intentionally a lightweight index. Source documents under `docs/` are authoritative according to `docs/README.md`; do not edit a concatenated bundle as a source of truth.

Read in this order:

1. `README.md`
2. `docs/README.md`
3. `CODEX_START_HERE.md`
4. `docs/00_REBASELINE_AND_ROADMAP.md`
5. `docs/00C_TERMINOLOGY_AND_BOUNDARIES.md`
6. `docs/00B_SECURITY_ARCHITECTURE.md`
7. the current release spec (`docs/01_v0.1_SECURE_WORKSPACE_RUNTIME.md`)
8. `docs/IMPLEMENTATION_STATUS.md`
9. future release plans `docs/02_...` through `docs/07_...`
10. `docs/90_CODEX_IMPLEMENTATION_HANDOFF.md`
11. `docs/91_IMPLEMENTATION_REFERENCE_NOTES.md`

For an ad-hoc concatenated local bundle, run:

```bash
python tools/build_master.py
```

The generated bundle under `dist/` is convenience output only. If it disagrees with the source files, the source files win.
