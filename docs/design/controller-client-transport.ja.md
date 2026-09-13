# Controller client transport

承認待ちは、この管理ソケットの approval.pending／approval.decide だけで確認・回答できます。実行や監査が失敗しても実際の capability 処理記録を返し、生のプロバイダー出力は除外します。読み取り専用の通知 bridge と guest Git ソケットには登録しません。[承認待ちの契約](pending-approval-review.ja.md)を参照してください。

日本語 | [**English**](controller-client-transport.md)

Status: **部分実装**。Local Unix domain プロトコル、Physical Host コントローラー、trusted-host 接続先投影、クライアント専用 `haco-host`、typed Environment API、対話ストリームは実装済み。製品の操作は[CLI参照](../reference/cli.ja.md)に集約します。ライフサイクル、スナップショット、転送、一時実行は実装済みです。PTY 制御、汎用ポート転送 CLI、遠隔通信は未実装です。

## 概要

現行product クライアントはBase一覧・確認と通常のEnvironment作成・削除を提供する。
`switch-base`は現在無効で、再導入時期は未定。SSH設定は既存のループバック接続情報から生成する。
任意の`plugin.oci.store`は信頼されたコントローラーで永続OCIデータを管理し、Environmentの
Git専用接続先には登録しない。Environment作成はWorkspaceと追加永続資源の利用権を
同じtransactionで予約する。[Persistent OCI Store](persistent-oci-store.md)を参照。

製品 `haco` は[管理repo利用手順](../guides/git-workflow.md)で既存コントローラーを呼ぶ。typed管理APIに `repository.clone`、`workspace.copy`、`environment.stop`、`git.connect/pending/decide` を追加した。これらは信頼された管理接続先に限り、EnvironmentのGit専用ソケットには公開しない。受入は[実装status](../IMPLEMENTATION_STATUS.ja.md)、残る旧コマンドは[CLI移行](../reference/cli-migration.md)を参照。

WSLは有効なコントローラーサービスがソケットをbindする前にlogin シェルを開くことがある。login aliasは読み取り専用pingで最大2分待ち、通信未準備だけを再試行する。プロトコル・operationの拒否は再試行せず、クライアントが第二のコントローラーを起動したりサービス状態を変更したりしない。この起動待ち期限は対話セッションの寿命を制限しない。

対話セッションはremote シェル終了後にlocal stdinが閉じられるまで待ってはいけない。Incus アダプターは子プロセスへ専用OS stdin pipeを渡してclosureを所有し、コントローラーはプロセス終了結果の記録後にクライアント接続を閉じる。出力をdrainし、実際のexit statusを保持する。Windows受入で、以前のソケット reader直接指定では `exit` 後も終了待ちする不具合が見つかった。構成要素テストはクライアント入力を開いたまま正常・非zero終了を確認する。WSL login aliasも実際のterminal fdを要求し、`/dev/null` のようなcharacter デバイスからtrusted-host シェルを開始しない。

Hacocoon Clientは生の Incus 権限を直接受け取らず、信頼された Physical Host コントローラーにEnvironment / Host-authority operationを要求します。

Local パスは次です。

```text
client (`haco`, `haco-host`, future adapters)
  |
  | Hacocoon Unix-domain endpoint
  v
Physical Host haco-controller
  |
  | provider/backend boundary
  v
Incus or another Environment backend
```

Trusted `haco-host`内ではクライアント接続先を次のように投影します。

```text
trusted haco-host
  |
  | /var/lib/hacocoon-control.sock
  | Incus proxy device: haco-control
  v
Physical Host /run/hacocoon/control.sock
```

Local IPC hopを1つ増やすことは意図的です。Policy、Approval、authoritative 状態、logging、プロバイダー権限はコントローラー側に集約します。

## Trust boundary

`haco-host`は信頼されたですが、生の Incus daemon ソケット、`/var/lib/incus`、Physical HostのHacocoon 状態ディレクトリは渡しません。

通常のEnvironmentにはHacocoon control 接続先自体を渡しません。

```text
ordinary Environment       X---- haco-control deviceなし
trusted haco-host          -----> Hacocoon controller UDS
Physical Host controller   -----> Incus authority
```

専用Incus proxy `haco-control`は正確なtrusted-host 所有権識別情報を確認した後だけ照合・調整します。既存デバイスやクライアント接続先設定が想定外なら暗黙の overwriteせず安全側で拒否します。

## Physical Host endpoint

Controllerの既定local 接続先は次です。

```text
/run/hacocoon/control.sock
```

Supported WSL 初期設定では`haco-controller`をPhysical Hostのsystemd サービスとして常駐させます。信頼された Hostの配備前にcontrol ソケットが `root:hacocoon`、モード `0660` であることを検証します。このlocal groupへの所属はコントローラー権限を与えます。以下のtrusted-host側投影ソケットは `root:root`、モード `0600` のままです。

localhost TCP listenerは不要です。将来remote 通信が本当に必要になった場合だけ、同じクライアント境界の別実装として追加します。

Development/testでは`HACO_CONTROL_SOCKET`でlocal パスを上書きできます。ただしroot 権限でtrusted-hostを照合・調整する経路では、inherited environmentによって任意のHost ソケットへredirectされないよう固定のPhysical Host 接続先を使います。

既存パスが安全な古いソケットだと証明できない場合は安全側で拒否します。

## Trusted `haco-host` endpoint

Trusted instanceには`haco-control`という1本だけのIncus `proxy` デバイスを付与します。

```text
type=proxy
bind=instance
listen=unix:/var/lib/hacocoon-control.sock
connect=unix:/run/hacocoon/control.sock
mode=0600
uid=0
gid=0
```

さらにinstance 設定として次を設定します。

```text
environment.HACO_CONTROL_SOCKET=/var/lib/hacocoon-control.sock
environment.HACO_CLIENT_MODE=controller
```

Instance側ソケットを`/run`配下に置かないのは意図的です。Guest systemdはboot時に実行基盤 tmpfsをマウントするため、guest boot orderingから独立して存在させたいproxy listenerはstableな`/var/lib` パスに置きます。

`haco setup`は所有権識別情報を検証し、接続先 shapeを完全一致で照合・調整し、必要ならinstanceを起動し、`/usr/local/bin/haco-host`と同じreleaseのgeneral `/usr/local/bin/haco`の両方を配備します。各クライアントバイナリはSHA-256で検証し、Physical Host側元データはinvoking effective UID所有の通常の executableかつgroup/other writableでないことを要求します。Install後は`0755 root:root`へ収束させます。

`HACO_CLIENT_MODE=controller`はauthorization 認証情報ではなく、意図的なsafety / execution-context 識別情報です。移行用 `hacoq` はこの識別情報でguest-local 状態の構築を防ぐ。reset後の製品 `haco` はそのlocal 構成経路を持たない。Authorizationと方針は引き続きコントローラー側が権限です。

Supported WSL 初期設定はその後、実際の信頼された instance内で`haco-host doctor`を実行します。Physical Host コントローラーへのround tripが成功しない場合、通常の利用者の自動 login シェルを変更する前に初期設定を失敗させます。

## Host setup

Status: **実装済み**。commitを固定したパッケージ / 実際の Incus受入は[実装status](../IMPLEMENTATION_STATUS.ja.md)に記録する。

`haco setup` は両クライアントの実行場所から既存Physical Host コントローラーの `system.setup` を呼ぶ。所有Host・ストレージ・ネットワークと必要な2本のクライアントバイナリを準備する。通常の準備要求は資源パスを受け取らず、companion パスは稼働コントローラー executableの隣から解決する。両元データをプロバイダー変更前に検証する。旧CLI・guest コントローラー・callerが指定するroot コマンドは使わない。

同時setupは1件に限定する。server上限は15分、CLIは16分。クライアントのキャンセルは接続を閉じるが、コントローラーの期限付き操作が続いている場合がある。その間の別要求はbusyとなる。明示的な再実行は所有resourceと検証済みクライアントを再利用し、失敗時もデータを保持する。失敗は再形式や削除の許可ではない。setupはresourceの準備を報告し、インストーラーがコントローラー round tripと疎通を別途検証してから完了する。読み取り検査には `haco doctor` を使う。

setupの失敗logはコントローラーが所有し、プロバイダー生出力を含めず、選んだerrorと次の操作を返す。クライアントはその失敗を表示し、transport/protocol失敗はクライアント側でlogにする。[ADR 0006](../adr/0006-controller-owned-host-setup.md)を参照。


明示的な保存手順の指定は[Host のカスタマイズ](trusted-host.ja.md#保存したカスタマイズ手順)に従います。呼び出し側が選ぶ管理コマンドや資源ルートは受け取りません。

## Host診断

Status: **実装済み**。このコマンドのpackaged受入は実装statusで別途追跡する。配布コントローラーバイナリには製品クライアントと同じversion・commit・ビルド日時を埋め込む。Windows gateは両方の実行場所でビルド識別子全体を照合し、開発用の既定値や古いコントローラーをpackaged受入の成功としない。

`haco doctor` と `haco doctor --json` は、Physical Hostと信頼された `haco-host` 内で同じ `system.doctor` コントローラー methodを使う。help/versionは引き続き単独で動作する。応答はコントローラーのビルド・プロトコルと、順序を固定した6項目を返す。

| Check | 確認する内容 |
|---|---|
| 実行基盤 | Incus APIの利用可否と信頼されたな管理アクセス |
| ストレージ | 設定対象Btrfs プールと設定上のマウント方針 |
| storage_mount | backing 識別とlive Btrfs 方針の読み取り検査。設定一致・live不一致は未完了 |
| trusted_host | 所有Hostの稼働、明示root/NIC、プロファイル継承なし、限定コントローラー接続先とクライアントモード |
| trusted_network | 所有bridgeのDNS・DHCP・NAT・経路選択・firewall設定 |
| trusted_connectivity | 検証済み信頼された HostからのIPv4 DNS、既定 route、固定公開対象github.comへのHTTPS |

検査はコントローラーのプロバイダーアダプターが実行する。クライアントは `hacoq` / Incusを起動せず、guest-local 状態を作らない。RPCはパス・コマンド・通信先・修復オプションを受け取らない。Host作成・起動、ストレージ初期化、NIC/firewall調整、サービス状態変更は行わない。Hostが停止していればfailedとなり、host/networkの所有権・設定が不一致なら疎通検査をskipする。

結果は `ok`・`failed`・`skipped`・`pending`（検証済みlive ストレージ方針不一致だけ）。全項目成功だけが終了0で、failed/skipped/pendingがあればreportを出して終了1、不正な使い方は終了2。transport/protocol失敗は終了1で、成功を示すJSON reportを出さない。項目欠落・重複・不明値・不正応答を拒否する。要約は成功した検査条件と失敗を区別する。failed/skipped/pendingには短い `action` を付け、textでは `Next:` として示す。成功項目には修復を勧めない。両項目は表示可能なASCII 256 バイトまでとし、backend/guestの生出力・errorをreportへコピーしない。固定検査終了値でDNS・既定 route欠落・HTTPS失敗を区別し、時間切れや未知の終了値から失敗段階を推測しない。失敗は共有loggerでstderrへ記録し、stdoutはtext/JSON結果に使う。

cold WSLでは、enabled コントローラーのソケットよりCLIが先に動くことがある。最初に読み取り専用pingで最大2分待ち、通信 unavailableだけを再試行する。その後の診断は一度だけ行う。protocol/operation拒否やfailed checkは再試行せず、サービスの起動・resource修復も行わない。

IncusのRunningはguest DNS/DHCPの準備完了より先になることがある。外部疎通検査前に、既存DNS サービスの稼働中と既定 IPv4 routeの出現を最大5秒待つ。localな前提を観測するだけでサービスを起動せず、DNS/HTTPSを再試行しない。待機に失敗した場合はDNS lookup障害とせず、ネットワーク起動準備が未完了と示す。外部検査は一度だけ行う。

inventory 検査は各5秒、疎通（起動待ちと検査）は10秒、server operationは40秒、CLI全体は165秒を上限とする。割込み・キャンセルでクライアント接続を閉じる。自動修復や権限を上げる代替経路はしない。固定対象への外部GETにHost 認証情報やcaller入力を渡さない。guest 検査は継承環境変数を消去し、curlの利用者設定を無効にする。対話シェルや `.curlrc` のcredential/proxy オプションは取り込まない。

成功reportはその時点の基盤検査である。設定/liveの検査は [マウント診断契約](btrfs-storage-layout.ja.md#読み取り専用のmount診断) に従うが、実圧縮率やCOW効率の証明ではない。trusted-host疎通はEnvironmentのproxy-only egress、SSH、Workspace保持、将来のfirewall再読込・起動順変更の受入ではない。保持している `haco-host doctor` は引き続きpingだけの移行用診断である。

## Protocol boundary

各接続の先頭にはversionedかつsize-boundedなJSON envelopeを置きます。Requestはmethodと、成功後にbidirectional ストリームへ遷移するかを指定します。

Protocol mismatchは明示的なerrorとし、direct Incus accessへ代替経路しません。Controllerはaccepted 接続数も上限付きのにします。

現在のtyped Environment APIは次を含みます。

- 作成
- list
- status
- 上限付きの exec
- interactive シェルストリーム
- 削除
- コントローラー ping / doctor 診断

Client-only `haco-host` と移行用に残る `hacoq env ...` はdirect Incus 権限を持たず、このAPIを利用する。これらの保持は、reset後の製品 `haco` での提供を意味しない。

## General `haco` client namespace

製品 `haco` はWSL Physical Hostと信頼された `haco-host` 内で共通の利用者入口となる。help/versionは単独で動作し、setup・診断・repo/Workspace/Environment管理・Git承認・WSL login aliasはコントローラーを直接呼ぶ。`hacoq` へ処理を委譲せず、未提供の `haco host ensure`・`haco host shell` も明示的に失敗する。

追加のEnvironment コマンドは一時的な `hacoq` に残り、製品の最初の開発経路はtyped コントローラー APIを使う。guest-local 構成やIncus 権限は持たない。インストーラーは `haco setup` から既存コントローラーへ初期設定を依頼する。旧CLIの初期設定 orchestrationとguestへのhacoq配備は撤去した。

## `haco-host` transition surface

Package済みクライアント専用バイナリは現在次を提供します。

```text
haco-host env list
haco-host env create --workspace <path> <environment>
haco-host env status <environment>
haco-host env exec <environment> -- <command...>
haco-host env shell <environment>
haco-host env delete <environment>
haco-host doctor
```

`haco-host env ...`は移行中のsurfaceとして有用ですが、通常のEnvironment ライフサイクルはgeneral `haco` UXへ移します。Long-termの`haco-host` コマンドは信頼されたツール、認証情報 broker、OCI/runtime administration、Windows/WSL 連携など、信頼された logical Host自体がexecution domainであるoperationへ寄せます。

`env create --workspace`はコントローラー側のWorkspace 元データ契約を使う。外部パスも維持し、`managed:<id>` は `WorkspaceProvider` 経由で登録済みの独立ボリュームへ解決する。登録上流 repoは信頼された logical Hostに置き、メタデータとプロバイダー所有権はコントローラーが保持する。

## Streaming

Stream handshakeでは可能な検証を成功 acknowledgementより前に行い、その後同じUnix-domain 通信上でbidirectional バイト列を流します。

対話Env shellは既存のraw streamとhalf-closeを維持します。別methodの`run.process`は
stdin／stdout／stderrと最終結果をframeに分け、共通run lifecycleを使います。
消費分だけ入力を許す上限と、入力停止／EOF確認によって早期終了時のreset競合を防ぎます。
結果frameと管理session完了の両方の成功が必要です。
[ADR 0069](../adr/0069-bounded-process-streams.md)を参照してください。今後の応用には次があります。

- Environment TCP 転送
- その他の上限付きの controller-mediated ストリーム

`Session`を新しい公開 domain conceptにはしません。StreamはExecutionまたはクライアント接続の実装詳細です。

### 対話端末の画面サイズ

Host-shell要求は任意の`display_language`を受け付け、Host準備前に空・`en`・`ja`だけに
限定します。clientで判定した値をadapterがそのHost sessionの`HACO_UI_LANGUAGE`へ渡します。
任意の環境変数やOS localeは転送せず、通常Envのshell要求に言語フィールドはありません。
[ADR 0065](../adr/0065-host-presentation-language.md)を参照してください。

状態: **implemented。インストール済み Incus/Windows/WSL での受入は pending**。

Host と Environment の shell client は、開始時の端末の列数・行数を request で渡す。
有効な非ゼロの画面サイズがある session は handshake で `terminal_resize` を通知する。
以後の変更は既存のランダムな session identity と、サイズを制限した
`_control.session.resize` RPC で送る。制御データはプロセスの stdin に混ぜない。
列数・行数はともに 1–10000 とし、連続更新は最新サイズに集約する。
終了済み・不明な session への制御は拒否し、管理 endpoint や Incus 権限を追加公開しない。

共通 terminal bridge は Linux/WSL の `SIGWINCH` を監視する。他の OS の native client は
console size を定期取得し、変更時だけ送信する。Linux Incus adapter は初期サイズを設定した
専用の raw PTY を `incus exec` に与え、その PTY の更新で Incus 本来の resize 転送を使う。
Incus の設定・project 選択・guest PTY 実装を維持する。Ctrl-C/Ctrl-D は入力バイトとして
転送する。対話 client の切断時は対応する local Incus process を終了させ、通常終了時は
最終出力と終了コードを受け取ってから接続を閉じる。

capability を返さない旧 peer は既存 stream 動作を維持する。非 TTY 入力はサイズを渡さず
既存の pipe 経路を使う。controller service の環境から端末サイズを推測しない。

component test は両 shell service 経路、サイズ検証・更新集約、入力バイト保持、実 PTY の
初期サイズ・変更 signal、長い入力の readline 編集、終了・切断・呼出元端末の復元を検証する。
installed acceptance では通常の WSL login と各 Host/Environment shell 入口を使い、
window resize と全画面 TUI を追加確認する。

## Performance

BaselineはUnix domain ソケット上の通常のGo buffered 転送です。Local callでもコントローラー hopを残し、権限の一元化を優先します。

巨大検証用構成をcommitせず測れる明示的な有効化 100 GiB-class benchmarkがあります。FD passing、`splice(2)`、buffer poolingなどはprofilingで価値が確認できた場合だけ追加します。

## 現在のacceptance

以下はリポジトリ内のテストと維持する実際の Incus gateの検証契約を示す。setup/client-only gateは `b71f88e` で成功した。後続の製品変更はそれぞれの受入を必要とする。

- TCPなしのlocal UDS request/response
- 上限付きの envelope / 接続同時実行
- 明示的なプロトコル error / キャンセル
- half-close behavior
- コントローラー経由のtyped Environment ライフサイクル call
- interactive シェル streaming
- 信頼された `haco-host` 所有権照合・調整
- 正確な `haco-control` proxy 照合・調整とmismatch 拒否
- `haco-host`とgeneral `haco` バイナリ配備のダイジェスト / 再実行時の一貫性検証
- 明示的な controller-client モードと想定外モード不一致の拒否
- 実信頼された instanceの`haco-host doctor`からPhysical Host コントローラーへのround trip
- stopped/restarted 信頼された Hostでのコントローラー再疎通
- production 配備済み`haco-host env`からcreate/list/status/exec/deleteをPhysical Host コントローラー経由で実行できること
- 新規 setupでguestに旧`hacoq`を配備しないこと
- 保持した旧alias・Base 経路選択・local 構成拒否の構成要素検証
- クライアント専用 companionでguest コマンドのexit status/stdout/stderrが保持されること
- 生の Incus control ソケット非露出
- 通常Environmentに信頼されたコントローラー接続先とclient-mode 識別情報が存在しないこと

今後のfollow-up:

- 残る`haco` コマンドをclassifyし、適切なものをコントローラークライアント interfaceへ移行
- replacementが確立したcompatibility aliasをremoveまたは明示deprecate
- 信頼された Host-local ツールをlong-termの`haco-host` 名前空間へ移行
- stdout/stderr/exit メタデータを持つstreamed Execution framing
- generic Environment 転送
- 実需が出た場合のみremote 通信
- profilingで必要性が示された場合のみFD passing / zero-copy

## 一時実行のキャンセル

状態: **通信と製品の一時実行 CLI は実装済み**。
`run.execute` はストリーム handshake の後、サイズ制限付きの JSON 結果を1つ返します。
入力 frame は受け付けません。クライアント接続の切断や想定外の入力で execution を中断します。
正規の run 後始末は独立した期限を使い、呼出元は切断を削除成功と扱ってはいけません。
結果の書込み期限は30秒です。通常のライフサイクル RPC の意味は変えません。
pre-1.0 の旧 call 形式は置き換え、結果が不明な実行を自動で再試行しません。
詳細は [ADR 0018](../adr/0018-ephemeral-run-cancellation.md) を参照してください。

## 日常の Environment 確認

状態: **CLI の範囲は実装済み**。`haco env list` は登録済み Environment の名前、Workspace、Base を表で表示します。
スクリプトでは `--json` で型付き一覧を取得できます。登録情報だけから現在の実行基盤状態を推測しません。
`haco env status <name>` は実行基盤状態を問い合わせます。両方の人向け表示で外部メタデータの端末制御文字を escape します。
空の状態では作成コマンドを示し、一覧がある場合は `haco open <name>` と status 確認へ案内します。

## Environment の診断

状態: **実行基盤の前提確認を実装済み。導入済み環境の検証は d4aef8d で成功**。

`haco doctor [--json] <environment>` は既存コントローラーから対象 Workspace、
実行基盤状態、クライアント接続を読みます。/workspace の存在、管理された DNS サービスと
resolver 設定を確認し、SSH 接続がある場合は ssh.service も確認します。
guest 検査は固定の読み取り専用コマンドで、対象診断全体を 20 秒に制限します。
停止した Environment は起動せず、依存する検査を skipped として報告します。

Host 公開 key、接続用の提案コマンド、生の guest stdout/stderr やバックエンド error は
表示しません。local check の失敗は次の確認手順と exit 1 を返し、自動修復しません。
成功はこの local prerequisite のみを示します。外部 DNS、egress Policy、
デスクトップからの実到達、ブラウザー描画は別途確認が必要です。
対象指定のない Host 診断と既存の六項目は従来どおりです。

任意の guest AWS 操作入口は、管理ソケットではなく隔離付き Standard HTTP listener を
共有します。list/get 要求だけを受け付け、信頼されたな送信元作成 ID を使います。
承認決定・設定・ライフサイクルメソッドは公開しません。
[AWS 操作](aws-operations.ja.md)を参照してください。

## Environment export stream

Linux の管理コントローラーは `environment.export` を登録します。停止済み元データ名だけを受け取り、
クライアントが指定する Host パスは受け取りません。検証済み bundle を上限付き正規の frame と
明示的な count/digest 完了情報で転送します。切断で処理を取り消し、後始末が不明なら既存の
所有記録を残します。[Environment export](environment-transfer.ja.md)を参照してください。

内部 `storage.reclaim-linux` は管理接続先のみに登録します。導入済み WSL の正確な
識別が必要で、操作失敗を含む段階別の観測結果を返します。Windows disk の権限は
付与しません。[Linux 容量回収](storage-reclamation.ja.md#所有権とlinux側の処理)を参照してください。

回収の管理経路には、導入済みコントローラー自身の検証済み WSL 識別を返す読み取り専用
`storage.reclamation-target` もあります。呼び出し側の対象選択やバックエンドの生エラーは返さず、
Windows 操作権限も与えません。[対象識別の取得](storage-reclamation.ja.md#windowsの登録とファイル識別)を参照してください。

## Host setupの観測

`system.setup.progress`は既存の特権管理socketだけで利用する、上限付きJSONイベントstreamです。`system.setup`と一つの排他を共有し、同じserviceを呼びます。固定stage/state/reason、所要時間、controller生成の相関IDを返します。完了には最終frameを要求し、EOFを成功とみなしません。CLIは別の変更要求へfallbackしません。接続断でも時間制限付き処理が戻るまでserver側のlifecycle所有権を維持します。guest endpointや新たな管理権限は追加しません。[setup診断](trusted-host.ja.md#setupの進捗と失敗診断)を参照してください。
