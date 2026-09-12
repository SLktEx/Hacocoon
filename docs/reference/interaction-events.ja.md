# クライアント中立 Interaction Event

## 通常の trusted Host での通知購読

実装済み: 通常の setup は同じリリースの `haco-notify` を `haco-host` に配布します。
Host 内で `haco-notify native` を実行できます。任意のブラウザー表示は
`haco-notify web` です。既存の `HACO_CLIENT_MODE=controller` により管理ソケットを
自動選択するため、接続先や監査パスの引数は不要です。接続失敗時にローカル監査へ
フォールバックしません。ファイル・オフライン用途の明示的な `NewReader(root)` は残ります。

信頼されたクライアントは既存の非公開 `events.stream` を読み、通知・HTTP 応答の前に
既存の公開スキーマへ絞ります。監査ファイルや新しい管理口を Environment に
マウントしません。購読側のエラーと件数制限でも再開位置を保持します。
コントローラーの失敗は一般化した公開エラーと受信済みの部分結果を返します。
型付きの詳細な破損診断は、直接ファイルを読む経路に限ります。

Windows インストールは検証済みのディストリビューション名を Windows PATH と
併せて記録し、setup が所有権を確認した Host に渡します。名前の衝突は拒否します。
旧インストールの更新は Windows インストーラーを再実行し、コントローラーの setup 前に
名前を記録してください。


信頼された Host の haco approve で現在の承認待ちを確認できます。これは非公開な管理経路であり、VS Code はローカル確認を開けます。任意の Windows native アダプターは同じローカル CLI を開きます。[承認待ちの契約](../design/pending-approval-review.ja.md)を参照してください。

## 承認要求の照合

信頼された承認画面と Git の承認待ち情報には、通知イベント・監査・実行結果と
同じコントローラー発行の `request_id` を渡します。この照合部分は実装済みですが、
VS Code からの確認はローカル CLI を使い、任意の Windows native アダプターは同じローカル CLI を開きます。ID 自体は権限を与えません。Git の決定には
既存の信頼された管理接続先と proposal ID を使います。読み取り専用のイベント
bridge に操作接続先や機密の詳細情報を追加しません。

Hacocoon は `github.com/SLktEx/Hacocoon/pkg/interaction` を通じて、クライアントアダプタ向けの小さな読み取り専用 Interaction Event 契約を提供します。

これは **表示・再接続のための境界**であり、認可境界ではありません。イベントを読むだけで capability の承認、実行、再試行、状態変更が発生することはありません。承認と実行は既存の Policy/Capability 経路に残ります。

## Event kind

| Kind | 意味 | 要対応 |
| --- | --- | --- |
| `approval-required` | 実行前に運用者承認が必要 | yes |
| `approval-approved` | 信頼された承認プロバイダーが承認済み | no |
| `approval-denied` | 承認が拒否された、または取得できなかった | yes |
| `policy-denied` | 方針が拒否した、または方針 evaluation が失敗 | yes |
| `operation-completed` | プロバイダー実行が成功 | no |
| `operation-failed` | recovery-required と証明されていない実行失敗 | yes |
| `recovery-required` | incompatible 状態または manual 復旧が必要 | yes |

各イベントには stable な `event_id`、capability の `request_id`、UTC 時刻、kind、必要最小限の Environment/capability/action、attention/recovery フラグ、`next_offset` が含まれます。

`event_id` は 1 capability ライフサイクル内で `<request_id>:<kind>` として決定的に生成されるため、複数クライアントが独立に同一イベントを dedup できます。

## セキュリティ上の最小化

クライアント向け schema は、生の capability resource、要求 attributes、内部を解釈しないパラメーター、プロバイダー出力、承認 token、認証情報、自由形式監査理由を**出しません**。

失敗の `code` は closed allowlist からのみ出力されます。未知または古い自由形式理由は `operation-failed` や `policy-error` のような一般化された code に落とし、元文字列をクライアント payload にコピーしません。

そのため UI は Git 認証情報、コマンド stderr、署名付き URL、ローカルの機微なパス、表示用途ではない方針詳細を誤って通知へ露出させずに済みます。

## Resume / dedup

最後に commit したバイト cursor から `Batch` を読みます。

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

`next_offset` はクライアント event を生成しない内部監査 record も跨いで進むため、再接続時に内部ライフサイクル record を毎回読み直す必要はありません。

batch size は `MaxBatchSize` で制限され、基礎となる監査 reader も 1 record 単位でメモリ上限があります。監査が破損している場合、`Batch` は破損直前までの trustworthy prefix と公開な `*interaction.CorruptionError` を返し、最初の破損位置以降を公開しません。

## 複数クライアント

複数クライアントが同じ Interaction ストリームを読んでも構いません。観測は副作用がないため、2 クライアントが同じ `approval-required` を見ただけで operation が二重実行されることはありません。

将来ブラウザー / VS Code / code-server / JetBrains 等のクライアントが承認を送信する場合も、信頼された Hacocoon approval/action 境界を通す必要があります。Interaction Event 自体は承認 token ではありません。

## Browser Notification への対応

ブラウザークライアントは最小化済み項目のみを使って通知へ対応できます。

- `approval-required` -> `Hacocoon approval required`
- `recovery-required` -> `Hacocoon needs recovery`
- `operation-failed` -> `Hacocoon operation failed`
- `operation-completed` -> 必要なら低優先度の完了通知

body は `capability`、`action`、`environment` のみから構成し、隠された生の監査項目を復元しないでください。

Browser Notification permission、サービス worker、UI presentation はクライアント側の責務で、Hacocoon Core には入りません。

## Reference notification adapter

`haco-notify` は表示専用helperです。承認を送信したりcapabilityを実行したりしません。

### Browser

信頼された Host側でループバック限定クライアントを起動します。

```bash
haco-notify web --listen 127.0.0.1:18081
```

その後 `http://127.0.0.1:18081/` を開いてBrowser Notification permissionを許可します。pageは `pkg/interaction` を読むsame-origin/read-onlyの `/api/v1/events` をpollし、commit済みcursorと最近のstable event IDをブラウザー local ストレージへ保存し、サービス workerから通知を表示します。HTTP bridgeはnon-loopback listenを拒否し、CORSも有効化しません。

### Native OS notification

```bash
haco-notify native
```

WSLでは最初のmaintained パスとして `powershell.exe` 経由でWindows toastを表示します。Linux デスクトップでは利用可能なら `notify-send` を使います。アダプターはcursorと最近のstable event IDをモード 0600の状態ファイルへ保存します。`operation-completed` は `--include-completed` を指定した場合だけ通知します。

native通知文は最小化済み公開対話項目だけから生成します。Windows向け文字列はPowerShell スクリプトへ直接interpolateせずencoded データとして渡します。

### VS Code

任意なVS Code presentation クライアントは [`../clients/vscode-notify/README.md`](../../clients/vscode-notify/README.md) にあります。同じループバック `/api/v1/events` bridgeを読み、cursor/dedup 状態はVS Code `globalState`へ保存し、通常のVS Code 通知 UIへ表示します。

このextensionは `haco-vscode` の必須要件ではなく、標準Remote-SSHを置き換えません。表示は読み取り専用で、Review は別のローカル CLI を開きます。通知の表示・クリック自体は承認ではありません。

## Root

`interaction.NewDefaultReader()` は local Hacocoon と同じ root 規則を使います。`HACO_ROOT` があればそれを、なければ `/var/lib/hacocoon` を使います。明示的な adapter/test では `NewReader(root)` を利用できます。

任意のデスクトップ VS Code Review は回答せずローカルの信頼された CLI を開きます。任意の Windows native アダプターは同じローカル CLI を開きます。 [Contract](../design/pending-approval-review.ja.md).

### Native 通知の状態ファイル

Linux/WSL クライアントは終了まで状態ファイルのプロセスロックを保持します。同じ状態を使う
二つ目のクライアントは、イベント読み取り・通知前に停止します。新しい必須オプションは不要です。
別 inode のロックを同時取得させないため、終了後もロックファイルは残します。

状態は現在の利用者が所有する非公開の通常ファイルで、ハードリンク数を 1 に限定します。
親は同じ利用者が所有し、group/other が書き込めないディレクトリである必要があります。
クライアントはディレクトリを実行中固定し、リンク・特殊ファイルを拒否します。読み取りは
128 KiB に制限し、排他的なランダム一時ファイル、ファイル同期、不可分な rename、
ディレクトリ同期で保存します。旧来の固定 `.tmp` パスには書き込みません。
未対応プラットフォームは所有権やロックの保証を省略せず拒否します。
これはバックグラウンド動作の準備であり、Windows の自動起動は下記の通り実装済みで、インストール済み受入は別途確認します。

通知の表示と再開位置の保存は別の操作です。その間で終了すると、再起動後に表示が重複する場合がありますが、capability の承認や再実行にはなりません。

## Windows の自動通知

Windows インストールはデスクトッププロトコル登録後に、信頼された Host の所有する
`hacocoon-notify.service` を有効化します。Host とともに起動し、既存のコントローラー
接続から読み、非公開の再開位置を使います。日常の追加コマンドは不要です。
`-SkipDesktopReview` は登録をスキップし、所有するサービスも停止・無効化します。
所有しない unit やリンクは置き換えず拒否します。Linux デスクトップの自動起動は
この Windows 統合の対象外です。

初回の自動起動は過去の通知を再表示せず、現在の末尾から始めます。保存済みの再開位置は
リセットせず、待機中の要求は `haco approve` で確認できます。任意の helper フラグ
`--from-now` がこの初回方針を実装します。通知失敗には systemd の回数制限付き再起動を
使い、notifier 自身は native interop の修復やコントローラーの起動を行いません。
起動失敗はインストーラーが報告し、後の失敗はサービスの journal に残ります。

バイナリ更新は非公開の一時ディレクトリでダイジェストと root 所有の実行権限を検証し、
インストール名を不可分なに置換します。動作中の旧実行ファイルは切り詰めません。
後片付けは所有する正確な一時ファイル・ディレクトリだけを削除し、失敗を
recovery-required として返します。

通常の `haco setup` はバイナリの公開後に有効化済みの通知サービスを更新します。無効化したサービスを有効に戻さず、Windows のデスクトップ登録前にサービスを作成しません。
