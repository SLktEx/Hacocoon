# 製品CLI参照

[English](cli.md) | 日本語

`cmd/haco-product`から作る製品コマンド`haco`を説明します。
管理コマンドは信頼済み`haco-host`またはPhysical Hostで実行します。
入力ファイルのパスはクライアント側です。ただし`env create --workspace`のパスは
コントローラー側で解決するか、`managed:<id>`を使います。
通常はオプションを位置引数の前に指定します。`haco help`はコマンド群を表示し、
help・versionにコントローラーは不要です。

| 目的 | 構文・既定値 | 詳細 |
|---|---|---|
| Workspaceパス | `haco workspace prepare --path <dir> --repo <id[,id...]> [--name <name>] [--oci auto\|none\|oci:ID]`; `haco workspace fork --path <new-dir> [--name <name>] <source-dir>`; `haco open [--repo <ids>] [--client vscode\|ssh\|none] <dir>` | [所有者を固定した再開と独立データfork](../design/workspace-workflow.md) |
| TCP/UDP | `haco network tcp\|udp`, `host add\|remove`, `rule`, `list`, `revoke`; `haco env forward --protocol tcp\|udp --target-port <port> <env>` | [詳細オプション・ゲスト待受・管理権限](../design/network-connections.md) |
| ビルド情報 | `haco version [--json]`, `haco --version` | [ビルド情報](build-release-identity.ja.md) |
| Host・プロジェクト設定 | `haco setup [--script <path> \| --clear-script] [environment]` | [Host](../design/trusted-host.ja.md)・[プロジェクト](../design/project-setup.ja.md)。対象省略時は信頼済みHost |
| Host設定の再適用・結果 | `haco setup --reapply-script`、`haco setup --script-result` | [Host設定](../design/trusted-host.ja.md); Host専用 |
| 診断 | `haco doctor [--json] [environment]` | 既定はHost。失敗・スキップは非ゼロで終了 |
| ポリシー | `haco config`, `--edit` or `--file <json>` | [設定](configuration.ja.md) |
| 承認 | `haco approve [--json] [request-id]`; `haco approve --list` | [承認確認](../design/pending-approval-review.ja.md)。対話選択・範囲保存 |
| 元リポジトリ | `haco repo clone --branch <branch> <id> <URL>`; `list [--json]`; `delete [--yes] <id>` | [Git](../guides/git-workflow.ja.md)。既存branchが必要 |
| Workspace | `haco workspace create --repo <id[,id...]> <workspace>`; `list [--json]`; `delete [--yes] <id>` | Git・データの独立コピー |
| Env作成 | `haco env create --workspace <path-or-managed:id> [--base <base>] [--resource oci:<store> \| --no-oci] <name>` | 既定Base、任意に設定されたOCI初期化 |
| 状態の参照 | `haco env list [--json]`; `haco env status [--json] <name>` | 既定はテキスト |
| 開始・停止・削除 | `haco env start <name>`, `stop <name>`, `delete <name>` | [データの寿命](../guides/data-lifetime.ja.md) |
| デスクトップ接続 | `haco ssh setup [environment]`; `haco open [--client vscode\|ssh] [environment]` | 既定はVS Code。停止Envを再開し、複数候補は対話で選択 |
| 手動SSH | `haco env ssh --key <public-key-file> [--port <port>] <name>`; `ssh-config <name>`; `disconnect <name> <connection-id>` | ポート省略時は実行基盤が割当。[SSH詳細](windows-environment-ssh.md) |
| プレビュー | `haco open --port <port> [--close \| --no-browser] [environment]` | [HTTPプレビュー](../design/development-preview.ja.md)。Env内ループバックポート |
| 一時実行 | `haco run [-i \| -it] [--workspace <workspace>] [--base <base>] [--no-oci] [--read-only] [--json] -- <command...>` | [一時実行](../design/temporary-execution.ja.md)。`--rm`の既定はtrue。`--json`は出力収集時だけ |
| Base | `haco base list`; `list --all [--json]`; `inspect <base>`; `build <definition.json>`; `delete [--yes] <name-or-fingerprint>` | [Base](../design/base-images-and-custom-environments.md)。通常のlist/inspectはJSON |
| Git仲介 | `haco git connect <env>`; `status [--json] [--request <id>] <env>`; `reconcile [--json] [--request <id>] <env>`; `pending`; `approve [--save env\|all\|ask-env\|ask-all] <id>`; `deny [--save ...] <id>` | [Git承認](../guides/git-workflow.ja.md) |
| OCI Store | `haco plugin oci store create <id> [--from <id>]`; `inspect <id>`; `list [--json]`; `delete [--yes] <id>` | [Store](../design/persistent-oci-store.md)。`--from`は対象名の前にも指定可能 |
| OCIイメージ一覧 | `haco plugin oci image list [--unused] [--runtime nerdctl\|docker] [--json] [--host] [<env-or-store-id>]` | [イメージ参照](../design/oci-image-deletion.ja.md)。既定はnerdctl。`--host`時は対象引数なし |
| OCIイメージ削除 | `haco plugin oci image delete [--unused] [--runtime nerdctl\|docker] [--yes] [--host] [<env-or-store-id>] [<image-id-or-tag>]` | `--unused`時はイメージ引数なし。タグ付きでも未使用候補になる場合あり |
| スナップショット | `haco snapshot create [--json] <env>`; `list [--json] [env]`; `restore [--json] <id> [new-env]`; `delete <id>` | [スナップショット](../design/environment-snapshots.md) |
| コピー | `haco env copy [--json] <stopped-env> [new-env]` | 既定名は`<source>-copy`。[コピー](../design/environment-copy.md) |
| 移送 | `haco env export [--json] <stopped-env> [file.haco]`; `import [--json] <file.haco> [new-env]` | Linux。既定は`<env>.haco` / `<source>-imported`。[移送](../design/environment-transfer.ja.md) |
| ディスク割当回収 | `haco reclaim [--yes \| --status \| --review [--yes]]` | 管理Windows/WSLのみ。[容量回収](../design/storage-reclamation.ja.md) |
| AWS | `haco aws s3 ls [--env <name>] [--profile <name>] [--region <region>] s3://bucket/prefix`; `haco aws s3 cp [same options] s3://bucket/key <file>` | [AWS](../design/aws-operations.ja.md)。プロファイルの既定は`default`。ゲストでは`--env`禁止 |

`haco env switch-base`は明示的に無効です。製品`haco`にはroot直下の
`create/exec/shell/events/connections/forward`、`plugin git`、`plugin oci seed/docker`、
`env create`・`run`のCPU・memory・PID・root容量フラグはありません。
残る旧機能は[移行情報](cli-migration.md)へ分離しています。

通常の失敗は非ゼロ、構文誤りは多くの場合2です。一時実行は後始末確認後にゲストの終了値を返し、
クライアントキャンセル時は130、後始末不明は失敗です。`reclaim`の開始受付は完了ではありません。
JSONは明示したコマンドだけに使え、全体共通の`--json`はありません。

## 階層別のヘルプ

開発候補ではEnv・repo・Workspace・Git・Base・snapshot・OCI・network・AWS・SSH・openを
共通の一覧／個別ヘルプで案内します。`haco env --help`は1コマンドずつ用途を表示し、
`haco env create --help`は対象だけの構文と例を表示します。明示した`--help`/`-h`は
controllerやIncusを必要とせずstdoutへ表示して終了0、不正引数はstderrと非0です。
説明は60桁を基準にインデントを揃えて折り返します。長いコマンド引数の構文と
コピー可能な実行例の改行改善、補助haco-hostヘルプは残件です。
日英の対応範囲は[表示言語](cli-language.ja.md)を参照してください。
