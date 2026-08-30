# Build / checkpoint / release / support identity

[English](build-release-identity.md) | **日本語**

Hacocoonでは、developmentの進行、配布binaryのsoftware identity、Host/backendがsupportされる根拠を意図的に分離します。

## Development checkpoint

**Development checkpoint** は [`../status/versioning-and-release-status.ja.md`](../status/versioning-and-release-status.ja.md) で管理する、pre-1.0の高速な `v0.N` sequenceです。

これは **`main` にどのproduct / implementation / operator / observability / acceptance sliceまでlandしたか** を表します。

checkpointは次のものではありません。

- 公開済みGit tag
- GitHub Release
- compatibility guarantee
- 過去のすべてのhost-dependent acceptanceが完了した証明

そのためcheckpoint番号はsoftware releaseより速く進んで構いません。

## Software version / release tag

**Software version** はbuild済み・配布済みartifactのidentityです。

- 通常のlocal `go build` はrelease metadataを注入しない限り `version: dev` を返します。
- GoReleaserはlinker flagsでsoftware version、commit SHA、build dateを `haco` に注入します。
- 公式GitHub Releaseのauthorization / publicationは [`../RELEASE_SECURITY.ja.md`](../RELEASE_SECURITY.ja.md) に従います。

たとえばrelease tag `v0.8.0` がdevelopment checkpoint `v0.8` を意味するわけではなく、development checkpoint `v0.26` が存在しても `v0.26.0` Releaseが存在するとは限りません。

## Acceptance / support status

**Acceptance/support status** はHost baseline、Incus behavior、storage path、WSL flow、client environmentなど、具体的な実行境界に対する検証結果です。

現在のrepository realityとacceptance gapは [`../IMPLEMENTATION_STATUS.ja.md`](../IMPLEMENTATION_STATUS.ja.md) を正本として扱います。checkpointがimplementedでも、一部real-host acceptanceをhost-dependentとして明示的に残せます。

## Runtime identity

`haco version` はそれぞれを別フィールドで表示します。

```text
Hacocoon
  checkpoint: <development checkpoint>
  version: <software/release version or dev>
  commit: <source commit or unknown>
  built: <release build timestamp or unknown>
```

tooling向けには:

```bash
haco version --json
```

IncusやHost stateを初期化せずcompactに確認する場合:

```bash
haco --version
```

`haco` にcompileされるcheckpointはrelease SemVerの定数ではありません。`tools/bump-milestone` がcheckpoint正本と同期するgenerated build inputを更新します。

## Checkpointを進める

```bash
tools/bump-milestone v0.N "Gate Name"
```

helperは必ず次の `v0.N` だけを受け付け、authority mismatchを拒否し、英日current-checkpoint宣言・version table・generated build inputを同時更新した後にdocumentation consistency checkを実行します。

機械的なbump後は、`IMPLEMENTATION_STATUS` とowner design/reference docの内容を実際のcode realityに合わせて仕上げます。

## Pull request classification

maintained PRは必ず次のどれか1つに分類します。

- 新しいdevelopment checkpoint
- existing checkpoint内のfeature / hardening / acceptance
- release / packaging only
- docs / test / refactor / maintenance only

new-checkpoint PRではcheckpoint authorityを同じ変更内で更新します。release-only変更だけを理由にdevelopment checkpointを暗黙に進めません。
