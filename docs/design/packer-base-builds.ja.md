# PackerによるBase作成

日本語 | [English](packer-base-builds.md)

状態: 実装済み。従来の通常Env内Packerアダプターをnested Incus方式へ置き換えます。
専用WSL fixtureとfreshなUbuntu hosted CIで実Incus E2Eは成功しました。
TB規模の画像は未検証です。受入対象はamd64です。
実機結果は[受入記録](../status/acceptance-evidence.ja.md#nested-packer)で区別します。

## ツールを含むBaseを作る

通常の `haco setup` 後、trusted `haco-host` で実行します。

```bash
haco image build --name my-tools examples/packer
haco image inspect my-tools
haco open --new my-tools --name dev --client none
haco exec dev my-tool
```

ツールは `hello-from-packer` を返します。オプションはディレクトリの前に置きます。
[サンプル](../../examples/packer/base.pkr.hcl)は標準HCL2、実
[Incus plugin v1.0.5](https://github.com/bketelsen/packer-plugin-incus/tree/v1.0.5)、
外部 `setup.sh` を使います。shellの標準属性`remote_folder`は`/root`とし、
起動時の`/tmp` mount／初期化との競合を避けます。Packerは1.16.0、pluginは `= 1.0.5` に固定します。
`image`、`output_image`、`container_name`、`launch_config`、
`publish_properties` はこのupstream版の契約です。

Packer本体、plugin、`shell-local`、local post-processorは通常のPackerの意味で
trusted `haco-host` 上で動きます。HCLとpluginは信頼済みビルドコードです。
provisionerの対象は別のnested Incus instanceです。そこにも通常Envにも、
Physical HostのIncus socket、管理状態、再利用可能な認証情報は渡しません。
Physical HostはHCL評価やplugin実行を行いません。

## 依存ツールと入力ファイル

標準Host setupがSHA-256固定のamd64/arm64 Packerを導入します。
IncusはPhysical Hostと共通の署名検証付きLTS installerをHost内で実行します。
対象は7.0.x、最低7.0.1で、通常のpatch更新を追い、新しい系列から自動降格しません。
所有者不明・非互換のnested storageは削除せず拒否します。
Pythonやpackage管理、IncusはHost側の依存で、ビルド対象Baseの前提ではありません。

nested daemonはHost内だけで待ち受け、Host自身の `/var/lib/incus` を使います。
所有属性付きdir pool `haco-packer` とNAT bridge `haco-packer0` を再利用します。
既存の `security.nesting=true` を使い、privileged化やPhysical Hostマウントは追加しません。
新規Hostでは初回起動前に設定し、Incusのnesting用mountとAppArmor namespaceを初期化します。
起動後の設定変更だけではこの準備を完了できません。
Host setupはnestedネットワーク用の`nftables`も導入します。build専用profileは`security.idmap.size=65536`を設定し、非特権のnested instanceへ渡すUID/GID範囲を限定します。
既存archive importに合わせimage圧縮は `none` です。
buildごとにランダムID、所有属性、独立image/profileを持つ専用projectを作ります。

CLIは通常ファイル128個・合計512 KiBまでを複製します。直下の `.pkr.hcl` が必要です。
隠し項目を除外し、不正path、link、特殊ファイル、上限超過を拒否します。
スクリプトは信頼済みHostで動くため、認証情報をcontextへ入れないでください。
Linux／WSLから利用します。

templateには `HACO_PACKER_BUILD_ID` を渡します。出力imageの
`user.hacocoon.packer-build` 属性へこの値を書き込み、一致する非公開container imageを
1個だけ生成します。変数と `auto.pkrvars.hcl` は通常のPacker仕様です。
元imageはHCLで選びます。`--from` と `--builder` はJSON定義専用です。

画像サイズと操作全体の固定上限は既定ではありません。`--max-image-size 2TiB` で任意の上限を設けられ、`--max-image-size unlimited` は既定動作を明示します。`haco image import` も同じ指定に対応します。明示上限はexport、転送、controller検証に適用します。整数のファイルoffset表現とfilesystem／storageの容量制約は残ります。転送メモリは一定で、画像全体を読み込みません。import Envのroot disk quotaは実artifactの2倍（最低64 GiB）とし、Coreの有限quota表現を超える場合だけ既存のdisk unlimitedを使います。CPU・メモリ・PID予算は有限のままです。容量不足は既存の失敗・復旧処理へ戻します。TB規模の実画像の所要時間・ディスク使用量は未検証です。

## artifactと公開

`fmt -> init -> validate -> build` はHost内の時間制限付きsystemd service/cgroupで動きます。
project所有者と完全なimage fingerprintを確認し、native unified container imageをexportして、
サイズ・SHA-256・CPU architectureを記録します。終了コードだけで成功を判定せず、
instance不在を確認してから専用project内の正確なimage、project、一時contextを削除します。

CLIが既存の上限付き `base.import` streamでartifactを送ります。controllerは操作ID、
hash、size、ローカルCPUを確認し、全入力の受信、archive検証、所有管理、
resource limit、既存Base import／公開ライフサイクルを再利用します。
import用の通常一時EnvはPacker実行場所ではありません。

immutable revisionとaliasの契約を維持します。再buildしても既存Envは元revisionを保持し、
新規Envだけが新revisionを選びます。JSON定義は別経路のままです。
[Base import](base-images-and-custom-environments.md#import-a-container-image-archive)を参照してください。

## 失敗と復旧

`/var/lib/hacocoon-packer/builds/<build-id>/receipt.json` に、正確なnested project、
Base名、image fingerprint、artifact hash/size、段階、controller側の `build-<build-id>` を永続記録します。
公開するnative imageにもbuilder名を記録し、import完了後の応答喪失でも
`haco image list --all --json` の `build_environment` と照合できます。これは診断情報であり、
削除の所有権確認は既存のfingerprintとimmutable build-instance identityで行います。
`--json` は状態と保持identityを返します。source・子process出力・認証情報を
controllerのログには入れません。

HCL不正、init／plugin／provisioner失敗、中断、export失敗、nested cleanup失敗では
importを開始しません。変更結果が不明ならreceiptとresourceを復旧必要として保持します。
stream中断やimport応答不明でもartifactとcontroller identityを残し、
自動再試行、alias巻き戻し、名前推測による削除は行いません。

import確認後に正確な操作のartifactとreceiptを削除します。既存archive importと同じく、
完全に公開されたBaseは後続cleanup失敗でも保持し、成功ではなく復旧必要と報告します。
native公開結果が不明なら既存のimage/build証拠を維持します。
応答を失っただけではpointerが動かなかったとは証明できないため、再実行せず証拠を調べます。

Packer各段階とimage exportにwrapperの固定timeoutは設けません。
worker終了時はcgroup内の子processも停止し、CLI中断はその操作のserviceだけを停止要求します。
管理操作・通信停滞・cleanupの待ち時間は有限のままです。CLIの強制終了ではworkerが継続する場合があるため、
正確なreceiptとserviceを確認し、必要ならそのserviceを停止します。永続記録を保持し、推測cleanupはしません。[ADR 0114](../adr/0114-trusted-host-nested-packer.ja.md)がADR 0089を置き換えます。
