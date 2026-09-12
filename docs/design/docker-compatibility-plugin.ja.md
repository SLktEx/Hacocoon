# Docker Compatibility Plugin

> 旧CLIの任意連携です。以下のコマンドはPhysical Hostの移行用 `hacoq` で使います。現行の通常操作は[CLI参照](../reference/cli.ja.md)と[移行情報](../reference/cli-migration.md)を参照してください。

[English](docker-compatibility-plugin.md) | **日本語**

Status: **マイルストーン順より先にリポジトリ実装完了。実機検証は環境依存で別途。**

v0.18はDocker互換を任意 OCI プラグイン機能として扱います。Hacocoon CoreはEnvironmentとExecutionを扱い、Docker Engine / containerd / nerdctlを必須にしません。

## Maintained OCI profile

project-maintained OCI プラグインプロファイルでは `containerd + nerdctl` を使えます。これはCore invariantではありません。

Docker互換は追加機能です。

```text
maintained profile -> nerdctl -> containerd
compatibility      -> genuine Docker CLI -> optional/on-demand dockerd -> existing containerd where supported
```

## Commands

`HACO_PLUGIN_OCI=docker` を選んだ場合だけDocker ライフサイクルコマンドを公開します。

```text
hacoq plugin oci docker status <environment> [--json]
hacoq plugin oci docker prepare <environment> [--json]
```

`status` は観測だけを行い、`dockerd` を起動しません。

`prepare` は意図的に狭く、繰り返しても同じ結果になるです。

1. 信頼された Hacocoon 状態から管理対象の Environmentを解決する
2. genuine `docker` CLI、`dockerd`、`containerd`、systemd、`docker` groupを確認する
3. 導入済み `hacocoon-docker.socket` / `.service` がプラグインに固定されたunitと完全一致することを確認する
4. vendor Docker daemon/socketが既に稼働中なら勝手に停止せず安全側で拒否する
5. 停止中なvendor Docker autostartだけ無効化する
6. `hacocoon-docker.socket` だけをenable/startする
7. 再検査して期待したsocket-activated 状態でなければ安全側で拒否する

`prepare` はパッケージ install、イメージ pull、Host ソケットマウント、既存guest Docker daemonの暗黙の停止を行いません。必要なDocker compatibility プロファイルと固定済み unitはBase/Seed側で提供します。

`hacocoon-docker.service` が停止中でも正常です。Environment 内クライアントが `/run/docker.sock` を開いた時だけEngineがon-demand起動する想定です。

## Plugin boundary

- Docker/nerdctl固有処理は `modules/plugin/oci` / `hacoq plugin oci` に置く
- `HACO_PLUGIN_OCI=nerdctl|docker` で明示明示的な有効化。未設定ならOCI プラグインなし
- dockerdをalways-on要件にしない
- EngineはEnvironment 内かつソケット activationで必要時起動する
- Host Docker/containerd/Incus/Hacocoon control ソケットをマウントしない
- Docker APIのTCP listenerを既定で公開しない
- complete byte-level deduplicationは保証しない

## Repository gate

plugin-owned systemd packaging、lifecycle/status サービス、CLI 連携、unit 不一致の安全側で拒否する検証、unit テストまでリポジトリ gateとして実装済みです。実機検証は別で、実際のBaseが必要binary/unitを持ち、対象Incus/systemd環境でソケット activationが動作することの確認は環境依存です。


## Non-goals

- containerd / nerdctlをCore requirementにすること
- Docker Engineをmandatoryにすること
- dockerdを常駐させること
- Host Docker/containerd ソケットを公開すること
- `hacoq plugin oci docker prepare` からDocker パッケージをinstall/updateすること
- Docker固有概念をCoreへ持ち込むこと
