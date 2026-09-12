# インストールとHostへの接続

[English](installation.md) | 日本語

インストーラーの選択と中断後の扱いを説明します。最初の開発まで進める場合は
[利用開始ガイド](getting-started.ja.md)を参照してください。
ローカルHostの前提はsystemdを使うUbuntu 26.04以降です。
Windowsでは専用のUbuntu 26.04 WSL 2を使います。
[実機検証の範囲](../status/acceptance-evidence.ja.md#installation)は実装とは区別します。

## 完全なパッケージを選ぶ

[Releases](https://github.com/SLktEx/Hacocoon/releases)からCPUに合う
`hacocoon-windows-amd64.zip` / `hacocoon-windows-arm64.zip`、
または`hacocoon-ubuntu-amd64.tar.gz` / `hacocoon-ubuntu-arm64.tar.gz`を取得し、
全体を展開します。`haco_linux_*.tar.gz`だけでは通常のHost導入になりません。

パッケージは対応CPU一種類のLinuxアーカイブ、スクリプト、チェックサム、VERSIONを含みます。
同梱アーカイブを使いますが、Ubuntu パッケージや設定した来歴検証には通信が必要です。
公開版はmainより古い場合があります。`haco version --json`で識別し、
mainの新機能が古い公開版にもあると判断しないでください。

## Windows

現在のWSLを導入したWindowsで、展開先のPowerShellから実行します。

```powershell
.\install-windows.bat
wsl -d Hacocoon
```

一度の呼出しで専用WSLディストリビューションだけを作成・再利用し、WSL 2とsystemdを設定します。
新規導入ではパスワードログインを無効にした非rootの`hacocoon`を作成します。
既存の非root既定ユーザーとパスワードは保持します。
WSL全体の既定変更、包括的sudo付与、他のWSLディストリビューションの登録解除は行いません。

Linux側の先行セットアップで`hacocoon`グループが作成済みなら、非rootのGIDを検証して管理ユーザーに再利用します。グループがなければ通常の専用グループを作ります。照会失敗、不正な記録、GID 0はアカウント作成前に拒否し、既存アカウントとパスワードは変更しません。

日本語Windowsへの新規導入では、ユーザー準備前に`ja_JP.UTF-8`を設定します。既存WSLの言語設定は保持し、Host接続時の案内はメッセージの言語設定に従います。日本語Windowsへの新規導入の実機確認は未完了です。[Host入口の言語](../design/trusted-host.ja.md#host-入口の言語)を参照してください。

Ubuntuの対話的なアカウント・初回設定を使う場合は、代わりに次を実行します。

```powershell
.\install-windows.bat -InteractiveUserSetup
```

初回設定を完了してシェルを終了すると、同じインストーラーが続行します。
既定の管理アカウント方式では、Ubuntuの既知のアカウント・metrics初回コマンドだけを解除し、
metrics収集への同意は設定しません。未知の設定は拒否します。
[ADR 0004](../adr/0004-wsl-installer-authority.md)に理由があります。

通常の`wsl -d Hacocoon`は信頼済み`haco-host`へ入ります。
明示的・非対話のWSLコマンドはPhysical Hostで動きます。
Windows実行ファイルとドライブは信頼済みHostだけに投影し、Envには渡しません。
rootでの復旧入口は明示的に残ります。

```powershell
wsl -d Hacocoon -u root
```

これは既存の管理権限で診断するための入口です。通常の開発作業や所有記録の手動削除には使いません。
管理境界は[信頼済みHost](../design/trusted-host.ja.md)を参照してください。

## ネイティブUbuntu

パッケージの展開先で実行します。

```bash
./install-ubuntu.sh
```

この入口はWSLを拒否し、Ubuntu・systemdを確認して必要時にsudoを使います。
利用者のログインシェルは変更しません。
信頼済みHostへは移行用の[Host接続コマンド](../reference/cli-migration.md#host-entry)を使います。
接続後の通常開発は製品CLIの`haco`で行います。

## 完了確認と診断

共通処理は依存パッケージ、Incus、バイナリ、Physical Hostのコントローラーを導入し、
`root:hacocoon`・モード 0660のソケットを検証して`haco setup`を呼びます。
実際の信頼済みHost経由でコントローラー、DNS、route、HTTPS、doctorを確認してから完了します。
ストレージからネットワーク準備を推測せず、未使用の既定Incus プールを初期化しません。
所有Btrfs・ネットワークの構成確認はアダプターが担当します。

通常のHost接続後に実行します。

```bash
haco doctor
haco doctor --json
```

doctorは修復や停止Hostの起動をせず状態を示します。
失敗・スキップ・利用不可は非ゼロで終了し、マウント方針適用待ちを準備完了とはしません。
検査を通すためにデータをリセットせず、表示された次の対応に従ってください。

## インストールが中断した場合

WSL一覧の取得失敗は状態不明であり、「存在しない」ではありません。
登録後も成功した二度目の一覧に対象が必要です。終了値3010は再起動要求です。
終了値0で登録がなければ未完了とし、それだけで再起動が原因とは判断しません。

登録失敗時は展開先へ新しい`hacocoon-installation-<id>.json`を保存し、
段階・オプション・再実行コマンドを記録します。このファイルを実行したり検証回避に使ったりしません。
WSLがWindows再起動を求めた場合は作業を保存して再起動し、同じ現行パッケージの場所から
表示されたコマンドを再実行します。それ以外は示された失敗を先に解決してください。
OS自動再起動や自動実行はありません。

アカウント・準備確認は読み取りだけを制限付きで再試行し、変更操作はこの経路で再試行しません。
照会失敗から既存アカウントの欠落を推測しません。
現行インストーラーの再実行は所有が一致したリソースを再確認し、データを保持します。
任意の古いpre-1.0インストーラーとの互換は保証しません。

## 検証用の任意のイメージキャッシュ

`-UseCachedWslImage`は繰り返し試験用に展開先の`ubuntu.wsl`を使います。
なければMicrosoftのUbuntu-26.04の該当CPU項目を解決し、SHA-256を確認後に確定します。
`-WebDownload`とは併用できず、キャッシュはリリースの配布物ではありません。
登録は起動したWindowsユーザーで行い、必要な昇格はWSLが担当します。
エラーはBATのコンソールに残します。

信頼済みmainのCIが検証したキャッシュを作り、PRのCIは読出しだけ行います。
キャッシュがなければ同じ検証付きダウンロードを使います。
[CIの信頼境界](../../.github/security/CI_TRUST_BOUNDARY.md)を参照してください。

## 容量回収ツールと内部構成

通常のWindows管理導入は、検証した`haco-wsl.exe`を永続配置し、
正確なWSL GUID、導入ID、Windows所有者、VHDX IDを登録します。
PATH追加や展開ZIPの保持は不要です。同じ登録は再利用し、不一致は保持して拒否します。
公開[容量回収](../design/storage-reclamation.ja.md)では開始・完了・失敗確認を区別します。

処理段階とパッケージ構成は[インストーラー設計](../design/installer.md)、
来歴検証は[リリースの安全性](../security/release-security.ja.md)が管理します。
パッケージ E2EはBAT・Ubuntuの入口スクリプトから利用準備を確認します。
ローカルで作成した候補は公開・署名済みリリースではありません。

## Windowsのインストール結果を読む

`install-windows.bat`をダブルクリックすると、完了・失敗・再起動待ちの結果を
表示した後、キー入力まで画面を残します。終了3010は再起動待ちです。
Windows再起動後、保存済みの継続手順に従ってください。

自動実行では呼び出し元に`HACO_INSTALL_NO_PAUSE=1`を設定します。
`CI`環境変数が定義された場合も待機しません。元の終了コードとインストールの
検証・PowerShell引数は保持し、最後の待機だけを省略します。

```powershell
$env:HACO_INSTALL_NO_PAUSE = '1'
cmd /c .\install-windows.bat
```
