# クライアント中立 Interaction Event

Hacocoon は `github.com/SLktEx/Hacocoon/pkg/interaction` を通じて、クライアントアダプタ向けの小さな read-only Interaction Event 契約を提供します。

これは **表示・再接続のための境界**であり、認可境界ではありません。イベントを読むだけで capability の承認、実行、retry、状態変更が発生することはありません。承認と実行は既存の Policy/Capability 経路に残ります。

## Event kind

| Kind | 意味 | 要対応 |
| --- | --- | --- |
| `approval-required` | 実行前に operator approval が必要 | yes |
| `approval-approved` | trusted approval provider が承認済み | no |
| `approval-denied` | 承認が拒否された、または取得できなかった | yes |
| `policy-denied` | policy が拒否した、または policy evaluation が失敗 | yes |
| `operation-completed` | provider 実行が成功 | no |
| `operation-failed` | recovery-required と証明されていない実行失敗 | yes |
| `recovery-required` | incompatible state または manual recovery が必要 | yes |

各イベントには stable な `event_id`、capability の `request_id`、UTC 時刻、kind、必要最小限の Environment/capability/action、attention/recovery flag、`next_offset` が含まれます。

`event_id` は 1 capability lifecycle 内で `<request_id>:<kind>` として決定的に生成されるため、複数クライアントが独立に同一イベントを dedup できます。

## セキュリティ上の最小化

クライアント向け schema は、raw capability resource、request attributes、opaque parameters、provider output、approval token、credential、free-form audit reason を**出しません**。

failure の `code` は closed allowlist からのみ出力されます。未知または古い free-form reason は `operation-failed` や `policy-error` のような一般化された code に落とし、元文字列を client payload にコピーしません。

そのため UI は Git credential、command stderr、署名付き URL、ローカルの機微な path、表示用途ではない policy detail を誤って通知へ露出させずに済みます。

## Resume / dedup

最後に commit した byte cursor から `Batch` を読みます。

```go
reader, err := interaction.NewDefaultReader()
if err != nil {
    return err
}

batch, err := reader.Batch(ctx, cursor, interaction.DefaultBatchSize)
for _, event := range batch.Events {
    present(event)
}
cursor = batch.NextOffset
if err != nil {
    return err
}
```

`next_offset` は client event を生成しない内部 audit record も跨いで進むため、再接続時に内部 lifecycle record を毎回読み直す必要はありません。

batch size は `MaxBatchSize` で制限され、基礎となる audit reader も 1 record 単位でメモリ上限があります。audit が破損している場合、`Batch` は破損直前までの trustworthy prefix と public な `*interaction.CorruptionError` を返し、最初の破損位置以降を公開しません。

## 複数クライアント

複数 client が同じ Interaction stream を読んでも構いません。観測は side-effect free なので、2 client が同じ `approval-required` を見ただけで operation が二重実行されることはありません。

将来 browser / VS Code / code-server / JetBrains 等の client が approval を送信する場合も、trusted Hacocoon approval/action boundary を通す必要があります。Interaction Event 自体は approval token ではありません。

## Browser Notification への対応

browser client は最小化済み field のみを使って通知へ対応できます。

- `approval-required` -> `Hacocoon approval required`
- `recovery-required` -> `Hacocoon needs recovery`
- `operation-failed` -> `Hacocoon operation failed`
- `operation-completed` -> 必要なら低優先度の完了通知

body は `capability`、`action`、`environment` のみから構成し、隠された raw audit field を復元しないでください。

Browser Notification permission、service worker、UI presentation は client 側の責務で、Hacocoon Core には入りません。

## Reference notification adapter

`haco-notify` は表示専用helperです。approvalを送信したりcapabilityを実行したりしません。

### Browser

trusted Host側でloopback限定clientを起動します。

```bash
haco-notify web --listen 127.0.0.1:18081
```

その後 `http://127.0.0.1:18081/` を開いてBrowser Notification permissionを許可します。pageは `pkg/interaction` を読むsame-origin/read-onlyの `/api/v1/events` をpollし、commit済みcursorと最近のstable event IDをbrowser local storageへ保存し、service workerから通知を表示します。HTTP bridgeはnon-loopback listenを拒否し、CORSも有効化しません。

### Native OS notification

```bash
haco-notify native
```

WSLでは最初のmaintained pathとして `powershell.exe` 経由でWindows toastを表示します。Linux desktopでは利用可能なら `notify-send` を使います。adapterはcursorと最近のstable event IDをmode 0600のstate fileへ保存します。`operation-completed` は `--include-completed` を指定した場合だけ通知します。

native通知文は最小化済みpublic interaction fieldだけから生成します。Windows向け文字列はPowerShell scriptへ直接interpolateせずencoded dataとして渡します。

### VS Code

optionalなVS Code presentation clientは [`../clients/vscode-notify/README.md`](../clients/vscode-notify/README.md) にあります。同じloopback `/api/v1/events` bridgeを読み、cursor/dedup stateはVS Code `globalState`へ保存し、通常のVS Code notification UIへ表示します。

このextensionは `haco-vscode` の必須要件ではなく、標準Remote-SSHを置き換えません。UI側のobserverにすぎず、通知を表示・clickすること自体はapprovalではありません。

## Root

`interaction.NewDefaultReader()` は local Hacocoon と同じ root 規則を使います。`HACO_ROOT` があればそれを、なければ `/var/lib/hacocoon` を使います。明示的な adapter/test では `NewReader(root)` を利用できます。
