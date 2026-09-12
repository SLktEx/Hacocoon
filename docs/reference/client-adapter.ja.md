# 再利用可能な Client Adapter Contract

Hacocoon は `github.com/SLktEx/Hacocoon/pkg/clientadapter` を通じて、client-neutral なアダプター API を公開します。

この契約は、VS Code固有の挙動へ依存せずにEnvironmentを作成/再利用し接続したいIDE、browser/code-server クライアント、CLI ツール、JetBrains アダプター、将来のクライアント向けです。

これは **クライアント連携境界** であり、新しいUIでもauthorization bypassでもありません。Policy/Capability 承認は信頼された権限パスに残り、対話 eventは `pkg/interaction` から読み取り専用に観測します。

## Public operation

| Operation | 役割 |
| --- | --- |
| `NewLocal` | local Hacocoon Hostへアダプターを開く |
| `Ensure` | Environment/Workspace/access-modeが完全一致すれば再利用し、それ以外は新規作成 |
| `Status` | client-safeなEnvironment 状態を取得 |
| `Connections` | Hacocoon/runtime 状態から現在のクライアント接続を照合・調整 |
| `PrepareSSH` | クライアントが渡す **公開 key** のみをinstallし、ループバック限定 SSHを作成 |
| `Forward` | ループバック限定 TCP 転送を作成 |
| `Revoke` | 管理対象の SSH/forward 接続を1つ撤回 |
| `Delete` | EnvironmentとHacocoon ライフサイクル状態を削除 |
| `InteractionBatch` | minimized/resumableな `pkg/interaction` eventを読む |

アダプターへ返すEnvironment内Workspace パスは常に次です。

```text
/workspace
```

Host側元データパスはlocal lifecycle/reuse判定用に `source_workspace` として別途返します。HacocoonがこのHost パスをremote サービスへ自動送信することはありません。

## Ownership model

### Hacocoonが持つもの

- Environment 識別 / ライフサイクル
- Workspace lease enforcement
- Incus/provider 接続 setup / 後始末
- ループバック限定 proxy enforcement
- Environmentへinstallする管理対象の SSH **public-key** 識別情報
- 再接続可能な接続メタデータ
- 信頼された Policy/Capability 承認 / execution
- 読み取り専用対話 event 元データ

### clientが持つもの

- SSH 非公開 key
- IDE/project 設定
- クライアントプロセス / 起動 behavior
- UI、通知、Browser Notification permission
- セッション間 dedupが必要な場合の対話 cursor/event ID persist

`pkg/clientadapter` は非公開 keyを受け取りません。`PrepareSSH` が受け取るのはpublic-key textだけです。対応する非公開 keyはクライアント自身がループバック接続先へ接続するときに直接使います。

## Fail-closed reuse

`Ensure` が既存Environmentを再利用できるのは、次の両方が完全一致する場合だけです。

1. 正規の Host Workspace パス
2. requested read-only/read-write access モード

異なるWorkspaceや異なる権限を持つEnvironmentを暗黙のに使い回しません。アダプターは `ErrAlreadyExists` を返します。

作成成功後の確認が失敗した場合は新規Environmentを後始末します。後始末できたか証明できない場合は `ErrRecoveryRequired` を返します。

## Connection security

underlying プロバイダーは管理対象の proxyをループバック限定に制限しています。`pkg/clientadapter` でもprojection時に再検証し、返却/reconcileされた接続のHostがループバックでなければrejectします。

SSHではさらに次を要求します。

- 接続 kindが `ssh`
- 対象ポートが `22`
- validなループバック Host / Host ポート

TCP 転送ではrequested 対象ポートとの一致を検証します。新規接続が契約違反ならアダプターが撤回し、撤回を証明できなければrecovery-requiredです。

これによりプロバイダー不一致でlocal-only 接続がLAN/WAN listenerへ広がってもクライアントアダプターが誤って受け入れません。

## Reconnect / process restart

クライアントプロセスはHacocoon 接続の正本ではありません。再起動後は次を呼べます。

1. `Status(environment)`
2. `Connections(environment)`
3. `InteractionBatch(lastOffset, ...)`

Incus-backed 接続照合・調整は管理対象の proxy メタデータを実行基盤から再構成するため、再接続するクライアントはメモリ内なVS Code セッションなしでも現在の接続先を発見できます。その接続を再利用するか、明示的に撤回できます。

## VS Codeを使わないgeneric proof

以下は Physical Host の旧 `hacoq` による外部パス用アダプターの例です。通常の管理 Workspace は[利用開始](../guides/getting-started.ja.md)の手順を使います。VS Code の拡張機能や通信方式には依存しません。

```sh
hacoq create --workspace "$PWD" demo
hacoq ssh demo --public-key "$HOME/.ssh/id_ed25519.pub" --host-port 2222
ssh -i "$HOME/.ssh/id_ed25519" -p 2222 root@127.0.0.1
```

クライアントシェルや別アダプタープロセスを再起動した後も確認できます。

```sh
hacoq status demo --json
hacoq connections demo --json
```

クライアント接続だけを撤回する場合:

```sh
hacoq unforward demo ssh-2222
```

Environment ライフサイクルを終える場合:

```sh
hacoq delete demo
```

非公開 keyを使うのは通常の `ssh` クライアントであり、Hacocoonではありません。

## code-server / 他IDE

code-server、JetBrains remote ツール、その他IDEはEnvironment内の通常software + client-owned launch/connection アダプターとして扱えます。Hacocoon Coreへ `code-server` / `jetbrains` / `vscode` conditionalを追加する必要はありません。

web 実行対象ではクライアントが実行対象ポートへのループバック転送を準備できます。ブラウザー exposure、authentication、URL handling、UIはクライアント責務です。

## Interaction event

`InteractionBatch` はclient-neutral 通知用の公開 `pkg/interaction` 契約を返します。eventを読むだけではcapabilityの承認・実行は起きません。

minimization、再開 cursor、Browser Notification mappingは [`INTERACTION_EVENTS.ja.md`](interaction-events.ja.md) を参照してください。

## Public compatibility boundary

`pkg/clientadapter` のexported signatureはpackage-owned DTOと公開 error sentinelだけを使い、`internal/core` typeを公開しません。provider/runtimeやIDE固有詳細はアダプター境界の内側に残します。

pre-1.0のためbreaking changeはまだあり得ますが、クライアント固有branchingはHacocoon Coreではなくクライアントアダプター側へ置きます。

## 公開ホスト鍵の固定

Status: IncusのSSH準備について実装済みです。`PrepareSSH` はプロバイダー経路で取得し
検証した公開server 識別を `host_public_key` に返します。commentや任意のguest出力は
含みません。不正な鍵は準備を失敗させ、管理対象の接続を撤回します。後始末に失敗した場合は
recovery-requiredです。非公開 Host keyは読みません。アダプターも公開前に再検証します。
他プロバイダーや接続一覧では省略される場合があり、クライアントは固定の作成・変更前に信頼できる
識別を取得する必要があります。既存known-host keyの無断置換を許可する機能ではありません。
製品の SSH 準備は自動化済みで、非公開 keyとローカル設定はクライアントが所有します。

## SSHの自動ポート選択

`PrepareSSH`の`HostPort: 0`は実行基盤権限側にループバックポート選択を任せます。
アダプターはクライアントのネットワーク名前空間でSSHポートを選びません。Incusはguestの鍵を
変更する前にproxyをbindし、bind失敗は操作失敗として返します。クライアントは応答の検証済み
ポートを使います。明示した非ゼロのポートも利用できます。この規則はSSHが対象で、
汎用転送は従来のクライアント側ポート選択のままです。
