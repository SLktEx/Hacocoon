# Hacocoon v0.1-v0.7 Master Bundle

This committed file is intentionally a lightweight index after the 2026-08-29 architecture rebaseline. The source documents under `docs/` are authoritative; do not edit a concatenated bundle as a source of truth.

Read in this order:

1. `README.md`
2. `CODEX_START_HERE.md`
3. `docs/00_REBASELINE_AND_ROADMAP.md`
4. `docs/00C_TERMINOLOGY_AND_BOUNDARIES.md`
5. `docs/00B_SECURITY_ARCHITECTURE.md`
6. `docs/01_v0.1_SECURE_WORKSPACE_RUNTIME.md` through `docs/07_v0.7_REMOTE_AND_CLOUD_RUNTIME.md`
7. `docs/90_CODEX_IMPLEMENTATION_HANDOFF.md`
8. `docs/IMPLEMENTATION_STATUS.md`

For an ad-hoc concatenated local bundle, run:

```bash
python tools/build_master.py
```

The generated bundle is written under `dist/` by default and is convenience output only.
