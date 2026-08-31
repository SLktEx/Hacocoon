# WSL nftables bridge compatibility PoC

This directory tests whether Hacocoon can keep Incus MAC/IP anti-spoofing enabled on stock Microsoft WSL kernels without replacing the whole WSL kernel.

## Why a normal missing module is not enough

Current Microsoft WSL 6.18 configs can have `CONFIG_NF_TABLES=y` and `CONFIG_NETFILTER_FAMILY_BRIDGE=y`, while `CONFIG_NF_TABLES_BRIDGE` is disabled. Upstream does **not** build `CONFIG_NF_TABLES_BRIDGE` as a standalone `nf_tables_bridge.ko`: the bridge default filter chain implementation lives in `net/netfilter/nft_chain_filter.c`, which is linked into `nf_tables.o`.

Because WSL builds `CONFIG_NF_TABLES=y`, that object is already built into the kernel with the bridge section omitted. There is no upstream `.ko` that can simply be copied into `/lib/modules` later.

The experimental `haco_nft_bridge` module therefore registers only the missing `NFPROTO_BRIDGE` default filter chain type through nftables' exported GPL APIs. It intentionally does not replace nftables, bridge networking, or Incus policy.

## Security boundary

This PoC must not be used to silently disable these Incus protections as a fallback:

- `security.mac_filtering=true`
- `security.ipv4_filtering=true`
- `security.ipv6_filtering=true`

If the compatibility module cannot be built and loaded for the **exact** running WSL kernel, Hacocoon should fail closed and use another supported design (for example a Hacocoon WSL kernel build or a network design that does not require nftables bridge-family hooks).

## Build

Install the Microsoft WSL kernel build dependencies first. The helper performs a full build of the exact Microsoft source tag so that `Module.symvers` matches `CONFIG_MODVERSIONS=y`, then builds the external module.

```bash
tools/wsl-nft-bridge/build.sh linux-msft-wsl-6.18.40.1
```

Inside a matching WSL kernel, the tag can normally be derived from `uname -r`:

```bash
tools/wsl-nft-bridge/build.sh
```

Output is written to `tools/wsl-nft-bridge/out/`.

## Runtime probe

First test the stock kernel without loading anything:

```bash
sudo tools/wsl-nft-bridge/probe.sh
```

A valid bridge-family test must create a **base filter chain with a hook**. Creating only `nft add table bridge ...` is not sufficient to prove `CONFIG_NF_TABLES_BRIDGE` functionality.

Then test the exact-matching module:

```bash
sudo tools/wsl-nft-bridge/probe.sh \
  --module tools/wsl-nft-bridge/out/haco_nft_bridge.ko
```

The probe refuses a module whose vermagic does not exactly match `uname -r`. It leaves a successful shim loaded so the next step can exercise the real Incus NIC anti-spoofing configuration.

## Status

Experimental. A compile-only GitHub Actions job validates that the compatibility module still builds against the pinned Microsoft WSL kernel release. Runtime success must still be confirmed inside real WSL before this can become an installer dependency.
