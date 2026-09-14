# Experimental VS Code設定

[English](experimental-vscode.md) | 日本語

状態: **リポジトリ内の実装済み。実際のVS Code・MarketplaceとWindows/WSLでの受け入れ確認は未実施**。
Experimental期間中は設定構造の互換性を強く保証しません。

`haco open`と同じ、信頼されたLinux/WSL Hostのユーザーで実行します。

```bash
haco experimental edit vscode
haco experimental edit vscode --file vscode.yaml
haco experimental edit vscode --json
haco experimental edit vscode --json \
  | jq '.settings["editor.formatOnSave"] = true' \
  | haco experimental edit vscode --json -
```

フラグなしでは`experimental.vscode`だけを`EDITOR`、`VISUAL`、`vi`の優先順で
開きます。`--file`はYAMLのサブツリーを読み込んで置換します。`--json`は変更せず
JSONを出力し、`--json -`は標準入力のJSONで置換します。`--file`と`--json`は
併用できません。保存の診断はstderrに出し、JSON適用時のstdoutは空です。
CLI専用のDBは持ちません。

正本は`$XDG_CONFIG_HOME/hacocoon/config.yaml`です。XDG_CONFIG_HOMEが未設定なら
`~/.config/hacocoon/config.yaml`を使います。信頼されたHostユーザーの設定であり、
`haco config`の承認Policyとは別です。ファイル全体の例は次のとおりです。

```yaml
experimental:
  vscode:
    settings:
      editor.formatOnSave: true
      files.trimTrailingWhitespace: true
    extensions:
      minReleaseAge: 30d
      preRelease: deny
      install:
        - id: ms-python.python
        - id: golang.go
        - id: rust-lang.rust-analyzer
          preRelease: allow
        - id: some.extension
          version: 1.2.3
```

`some.extension`は説明用です。実在するExtensionと版に置き換えてください。
エディタと`--file`に渡す内容は`vscode`以下の`settings`・`extensions`であり、
`experimental`や`vscode`のラッパーは含めません。

| フィールド | 動作 |
|---|---|
| `settings` | VS Codeの設定名とJSON互換の値。Env/Workspace全体に適用する共通Remote settings |
| `extensions.minReleaseAge` | 既定は`30d`。整数日数（`d` = 24時間）、または`12h`・`90m`等の非負の期間。`0d`なら公開直後も候補 |
| `extensions.preRelease` | 既定は`deny`。`allow`では安定版とpre-release版の両方を候補にする |
| `extensions.install[].id` | 必須の`publisher.name`。大文字小文字だけが違う重複も拒否 |
| `extensions.install[].preRelease` | Extension単位で`allow`・`deny`を上書き |
| `extensions.install[].version` | 完全なセマンティックバージョン。日数とpre-releaseの制約を適用せず、platformと導入の検証は維持 |

自動選択ではEnvのLinux x64/arm64向けに、日数とpre-release条件を満たす最も高い
セマンティックバージョンを選びます。経過時間はMarketplaceの版ごとの`lastUpdated`
から計算し、成果物が更新された場合も新しい公開として保守的に扱います。
platform専用成果物をuniversal版より優先します。30日ちょうどの版は候補に入ります。
`force`、バージョン範囲、最新版へのフォールバック、Extension単位の日数上書きは
ありません。不明なフィールド、YAMLの重複キー、複数文書、不正入力は保存前に
拒否します。settingsにはJSON互換の値を使い、文字列以外のキーや非有限数は使えません。
YAML 1.1で別の型になる値を文字列として扱う場合は引用符で囲んでください。

保存時は他のサブツリーと値を保持しますが、YAMLを書き直すためコメントや元の書式は
保持しません。CLI同士の書き込みは直列化し、古い読取結果からの更新は拒否します。
失敗した編集ファイルは保持して場所を表示します。`yq`等による直接編集も可能です。
直接編集を同時実行する場合は隣接する`.config.lock`を`flock`で共有するか、CLIの
書き込みがない間に編集してください。保存結果が不明なエラーでは正本を確認します。

## Environmentへの反映

編集後に`haco open <env>`または通常の`haco-vscode open <workspace>`を実行します。
サブツリーがなければ従来どおり開きます。SSH専用・`--no-launch`では反映しません。
管理対象settingsを削除する場合は、サブツリーを`{}`にして再度開きます。

安定版のデスクトップVS Codeを起動し、同じビルドのRemote-SSH serverの配置を最大2分
待ちます。設定の反映先はEnvユーザーの`~/.vscode-server/data/Machine/settings.json`
です。repo内に`.vscode`やworkspaceファイルを生成しません。無関係な既存JSON設定を
保持し、所有情報のコメントに管理するキーを記録します。YAMLから消したキーは次回の
反映で削除します。既存のJSONCコメント・末尾カンマは現段階では上書きせず拒否するため、
そのRemote settingsを先に厳密なJSONへ変換してください。既存のrepo設定を含め、
VS Code本来の適用範囲と優先順位は有効です。

HostがMarketplaceのメタデータを取得し、選んだ完全指定の版をEnvのVS Code Server CLIで
導入します。依存ExtensionとExtension Packにも共通の選択条件を適用し、明示したinstall
項目があれば依存先にもその指定を使います。server側の暗黙の依存解決は無効にします。
Hostのcredentialや管理ソケットは渡しません。Envからのダウンロードには既存の外向き
通信許可が必要で、この機能からネットワークPolicyを追加しません。VS Codeがengineの
互換性を検証します。候補なし、非互換、導入失敗、導入版の不一致はエラーとし、
自動的に別の版へフォールバックしません。

Envごとの適用を直列化します。失敗時は設定や先に導入したExtensionが残る場合があり、
Env削除やExtensionの巻き戻しはしません。install項目の削除はアンインストールを
意味しません。版を再確認するのはHacocoonから開いた時です。VS Code UIでの変更・
自動更新・信頼しないゲストプロセスに対する継続的なセキュリティ強制機構ではありません。

Envのrootfsを削除すると反映済み設定とExtensionも消えます。HostのYAMLは残り、
Envを再作成してから再適用できます。credentialや秘密鍵を設定へコピーしません。
repo別Folder Settings、Env・repo・Git・SSH Agent・remote・Approvalの状態を表示する
VS Code UI、Agent Host adapterへの接続は将来の対象です。初版ではserverの独自配置先と
Insidersは非対応です。

[所有と失敗時の動作](../design/experimental-vscode.ja.md)も参照してください。
