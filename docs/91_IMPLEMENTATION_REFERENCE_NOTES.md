# Implementation Reference Notes

These are implementation-time references, not Core dependencies. Re-check current upstream documentation when coding.

## Local storage resize

- Incus Btrfs loop pools support increasing the `size` setting but not shrinking the Incus-managed loop pool through normal pool resize.
- Btrfs can shrink a mounted filesystem and may relocate extents; `btrfs inspect-internal min-dev-size` can estimate the minimum device size.
- Btrfs balance can compact allocation but is IO intensive; use filters rather than an unconditional full balance.
- `qemu-img resize --shrink` explicitly warns that the inner filesystem/partition must be reduced first or data loss occurs.

Normative Hacocoon consequence: **shrink filesystem first, outer image second; refuse when preflight cannot prove safety.**

## EC2/EBS

EBS-specific shrink in v0.7 is modeled as a verified replacement workflow, not an in-place resize. AWS infrastructure identity is distinct from v0.4 Session AWS developer capability.
