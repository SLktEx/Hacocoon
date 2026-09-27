# trusted HostのPackerとnested Incus

状態: 採用。[ADR 0089](0089-guest-packer-provisioning.ja.md)を置き換えます。
[English](0114-trusted-host-nested-packer.md)

Packer・HCL・pluginは信頼済みビルドコードとして `haco-host` 上で実行します。
実Incus pluginがHost内nested daemonの新規所有projectにinstanceとimageを作ります。
既存Hostのnesting設定と署名検証付きIncus LTS installerを再利用し、
Physical Hostのsocket、storage、管理状態は公開しません。

native container archiveだけを既存Base importへ渡します。
Physical Hostはbytes検証と公開を担当し、HCL解釈、Packer実行、別Baseカタログを持ちません。
JSON buildは通常Env内の構築を維持し、immutable revisionと公開契約を保ちます。

次の失敗し得る処理より先にproject・image・artifact・import identityを永続receiptへ記録します。
plugin cleanupは別途確認し、nested resourceを片付けてからuploadします。
結果不明は自動再試行や推測削除をせず保持します。commit応答喪失はrollbackの証明ではありません。
公開済みの完全なrevisionは後続cleanup失敗でも保持します。

Physical HostでのHCL実行、通常Envへの管理権限、過去の同一Env null／SSH方式、
公開・カタログの重複、mock実行要求だけによる成功判定を採用しません。
実plugin取得とbuild、native export/import、新規Envでのツール実行をCIで必須にします。
[契約](../design/packer-base-builds.ja.md)を参照してください。

管理者の選択により、build/importのサイズと操作全体の時間には既定の固定上限を設けません。任意の画像上限、一定メモリのstream、キャンセル、有限のCPU・メモリ・PID予算、archive検証は維持します。filesystemの無限容量やTB規模の受入を意味しません。CLI強制終了では正確なworker serviceが継続し得るため、receiptを保持してそのserviceを確認・停止し、resourceを推測削除しません。
