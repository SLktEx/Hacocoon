# Documentation Refactor Notes

Date: 2026-08-29

This revision is a documentation/architecture refactor. It intentionally adds no new release feature.

## Contradictions removed

1. **Storage naming drift**: unified the local implementation on `storage.btrfs`; removed simultaneous `storage.local-btrfs` / `storage.btrfs` usage.
2. **Premature EBS abstraction**: removed EBS from the v0.1 private BlockStore seam. The exact EBS package/contract is now a v0.7 design gate because EC2 placement/attachment ordering is materially different from local Incus storage creation.
3. **Legacy `Hacocoon IAM` terminology**: replaced with `Security Framework`, `Capability Profile/Policy`, and `Capability Policy evaluator`. `AWS IAM` remains provider terminology.
4. **Capability Broker authority ambiguity**: clarified that Broker means routing/materialization, while Security Framework remains authorization authority.
5. **Access Layer ambiguity**: remote Gateway consumes verified identity context; Hacocoon does not become the SSO/TLS/WAF/IdP layer.
6. **Storage shrink duplication/wording**: removed duplicate prohibition and retained strict inner-to-outer shrink ordering.
7. **Generated master drift**: the MASTER file is now regenerated from canonical source documents by a build script rather than hand-maintained.

## Explicitly preserved decisions

- AWS developer access stays v0.4.
- EC2/remote deployment stays v0.7.
- Local GUI/IntelliJ is v0.5.
- Local Web UI, Browser Notification, approval interaction and code-server are v0.6.
- Core stays vendor-neutral; Incus/Btrfs/QCOW2/AWS/GitHub/IDE details remain outside Core.
- Access Layer remains deployment-owned.
