## Summary

<!-- What changed and why? -->

## Checkpoint / release classification

<!-- Check exactly ONE classification. Development checkpoints, software releases/tags, and acceptance/support status are separate concepts. See docs/reference/build-release-identity.md. -->

- [ ] **New development checkpoint** — meaningful product / implementation / operator / observability / acceptance slice; advances `v0.N`
- [ ] **Existing checkpoint** — feature / hardening / acceptance work stays inside the current checkpoint
- [ ] **Release / packaging only** — changes artifact/release behavior without advancing the development checkpoint
- [ ] **Docs / test / refactor / maintenance only** — no checkpoint or release identity change

If **New development checkpoint** is selected:

- [ ] I used or followed `tools/bump-milestone v0.N "Gate Name"`.
- [ ] `docs/status/checkpoints.yaml` contains the new current checkpoint and Gate identity.
- [ ] `docs/status/versioning-and-release-status.md` and `.ja.md` mirror the YAML numbering/Gate and describe status.
- [ ] `docs/IMPLEMENTATION_STATUS.md` and `.ja.md` describe the new code reality / acceptance status.
- [ ] The owning design/reference documentation is updated.

If **Release / packaging only** is selected:

- [ ] The development checkpoint was not advanced only because a release/tag/package changed.
- [ ] Release provenance/security expectations remain valid or are updated explicitly.

## Validation

- [ ] `go test ./...`
- [ ] `go vet ./...`
- [ ] `python3 tools/check_docs.py`
- [ ] Relevant E2E / provider-backed acceptance was run when required

## Security / trust boundary

<!-- Describe security-boundary impact, or write "none". Do not claim real-host acceptance that was not run. -->
