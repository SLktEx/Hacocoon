# Controller client transport

承認待ちは、この管理ソケットの approval.pending／approval.decide だけで確認・回答できます。実行や監査が失敗しても実際の capability 処理記録を返し、生のプロバイダー出力は除外します。読み取り専用の通知 bridge と guest Git ソケットには登録しません。[承認待ちの契約](pending-approval-review.ja.md)を参照してください。

日本語 | [**English**](controller-client-transport.md)

Status: **部分実装**。Local Unix domain プロトコル、Physical Host コントローラー、trusted-host 接続先投影、クライアント専用 `haco-host`、typed Environment API、対話ストリームは実装済み。製品の操作は[CLI参照](../reference/cli.ja.md)に集約します。ライフサイクル、スナップショット、転送、実行中のEnvironment内でのコマンド実行は実装済みです。PTY制御と同一PCのTCP転送は実装済みです。遠隔通信は今回の対象外です。

## 概要

現行product クライアントはBase一覧・確認と通常のEnvironment作成・削除を提供する。
`switch-base`は現在無効で、再導入時期は未定。SSH設定は生成IDを固定したProxyCommand targetを使い、既存UDSのbyte sessionと終了・cancel制御で接続する。[ポート不要SSH](client-and-interactive-access.md)を参照。
任意の`plugin.oci.store`は信頼されたコントローラーで永続OCIデータを管理し、Environmentの
Git専用接続先には登録しない。Environment作成はWorkspaceと追加永続資源の利用権を
同じtransactionで予約する。[Persistent OCI Store](persistent-oci-store.md)を参照。

製品 `haco` は[管理repo利用手順](../guides/git-workflow.md)で既存コントローラーを呼ぶ。型付き管理APIは `repository.add`、`workspace.copy`、`environment.stop`、`git.connect/pending/decide` を提供する。これらは信頼された管理接続先に限り、EnvironmentのGit専用ソケットには公開しない。受入は[実装状況](../IMPLEMENTATION_STATUS.ja.md)、コマンドとオプションは[CLI参照](../reference/cli.ja.md)を参照。

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

リスナーはGoのパス名だけに基づく自動削除を無効にし、権限を設定する前に、
bindしたソケットのファイル識別情報を記録します。権限設定失敗時を含む最初の
後始末では、パスの識別情報とファイル種別が記録時と一致する場合だけ削除します。
不存在・検査不能・置換済みのパスには触れず、下位リスナーのclose結果を維持します。
これは識別情報の記録後に置き換えられ、その後変化していないパスを保護するものです。
bindと識別情報の取得、パスに基づく権限設定、識別情報の照合と削除は、悪意ある
ディレクトリ書き込みに対して不可分ではありません。接続先の親ディレクトリは
信頼できる主体の管理下に保つ必要があります。この後始末の規則は、親の権限や
起動時の古いソケットの扱いを変更しません。

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

`HACO_CLIENT_MODE=controller` は実行場所の識別情報であり、認証情報ではありません。製品 `haco` に guest-local 構成経路はなく、認可と Policy の権限はコントローラーにあります。

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

`Server.Serve`の各呼び出しは、その待受から受け入れた接続を所有します。
キャンセル、待受の終了、接続受入のエラーで戻る前に、残っている接続を閉じ、
待受のキャンセル監視を終了します。同じserverの別の待受には影響しません。
共有する同時接続上限の256件には、通信を閉じた後も処理が続く場合を含め、
接続処理が実際に戻るまで数えます。終了時の切断とハンドラーの通常の切断は
同時に起こり得るため、接続実装は`net.Conn`の並行呼び出しとI/O中断の契約を
満たす必要があります。終了処理は、その`Close`が戻ることを前提とします。

これは通信の後始末であり、任意の操作が完了するまで待つ処理ではありません。
ハンドラーのcontextには呼出元のキャンセル条件をそのまま渡します。
クライアントが切断しただけでは、期限付きsetupをキャンセルしたり、その排他を
解除したりしません。強制終了では応答やストリームが途切れる場合があります。
EOFだけでは操作の成功を確認できず、既存のセッション完了確認や各APIの
最終結果の確認が引き続き必要です。

現在のtyped Environment APIは次を含みます。

- 作成
- list
- status
- 上限付きの exec
- interactive シェルストリーム
- 削除
- コントローラー ping / doctor 診断

`haco` と client-only の `haco-host` companion は直接 Incus 権限を持たず、この API を使います。提供するコマンドは [CLI リファレンス](../reference/cli.ja.md)を参照してください。

## General `haco` client namespace

製品 `haco` はWSL Physical Hostと信頼された `haco-host` 内で共通の利用者入口となる。help/versionは単独で動作し、setup・診断・repo/Workspace/Environment管理・Git承認・WSL login aliasはコントローラーを直接呼ぶ。`hacoq` へ処理を委譲せず、未提供の `haco host ensure`・`haco host shell` も明示的に失敗する。

インストーラーは `haco setup` から既存コントローラーへ初期設定を依頼します。旧 CLI と専用 orchestration は [ADR 0107](../adr/0107-responsibility-layout-and-cli-retirement.ja.md)で削除しました。

## `haco-host` transition surface

Package済みクライアント専用バイナリは現在次を提供します。

```text
haco-host env list
haco-host env create --workspace <path> <environment>
haco-host env status <environment>
haco-host env exec <environment> -- <command...>
haco-host env shell <environment>
haco-host env delete [-f] <environment>
haco-host doctor
```

`haco-host env ...`は移行中のsurfaceとして有用ですが、通常のEnvironment ライフサイクルはgeneral `haco` UXへ移します。Long-termの`haco-host` コマンドは信頼されたツール、認証情報 broker、OCI/runtime administration、Windows/WSL 連携など、信頼された logical Host自体がexecution domainであるoperationへ寄せます。

`env create --workspace`はコントローラー側のWorkspace 元データ契約を使う。外部パスも維持し、`managed:<id>` は `WorkspaceProvider` 経由で登録済みの独立ボリュームへ解決する。登録上流 repoは信頼された logical Hostに置き、メタデータとプロバイダー所有権はコントローラーが保持する。

## Streaming

Stream handshakeでは可能な検証を成功 acknowledgementより前に行い、その後同じUnix-domain 通信上でbidirectional バイト列を流します。

現在は対話シェル、非対話実行の標準入出力・標準エラーと終了情報、TCPの双方向転送に
利用します。プロセスと双方向転送の管理セッションでは、接続時の完了確認用識別子を
必須とします。peerが識別子を省略した場合、clientは接続を閉じてプロトコルエラーを
返します。EOFだけをプロセス終了の証拠にはしません。pre-1.0のclientとcontrollerは
一緒に更新してください。[互換処理を廃止する判断](../adr/0107-responsibility-layout-and-cli-retirement.ja.md#維持する境界)を参照してください。

Envのシェル実行は、Workspace、provider router、Incus adapterを通る、呼出元指定の
ストリーム経路に一本化します。この任意機能を持たないproviderは未対応エラーを返します。
未使用の標準入出力への直接接続とadapterが保持する標準ストリームは削除し、CLIや
通信APIは変更しません。シェル準備はcatalogへのアクセス前に不正な名前を拒否します。
[廃止の判断](../adr/0107-responsibility-layout-and-cli-retirement.ja.md#維持する境界)を参照してください。

Linux/WSLの共通端末ブリッジは、ファイルとして渡されたパイプと端末に
専用の入力記述子を用意します。接続先のEOF、出力失敗、中止では、端末を復元してから
専用記述子を閉じ、入力転送処理の終了を待って戻ります。呼出元の記述子は閉じず、
ファイル状態フラグも変更しません。
パイプと端末は、保持した記述子の`/proc/self/fd`経由で同一性を確認して開き直し、
制御端末を新たに取得しません。実際の端末deviceの同一性も別に確認します。
PTYのmasterは開き直すと別の端末になるため拒否します。
開き直せない場合や読み取り待ちを中断できない場合は
明示的に失敗します。開き直す際は元のinodeの読み取り権限も確認されるため、
資格情報の変更前から引き継いだ記述子でも拒否される場合があります。
通常ファイルの読み取り位置、呼出元が所有する任意のreader、socket、端末以外のdeviceと
他のplatformは従来の入力動作を維持し、ブロックする読み取りの中断は呼出元が担当します。
読み取り待ちを監視できない通常ファイルでは、closeでもファイルシステム内の待ちを中断できません。

setup進捗やデータ転送など、結果・イベントの形式を独自に定めるAPIには生の
ストリームを使います。通信層が返すのはバイト列とEOFであり、完了確認は各APIの
プロトコルが担当します。

`Session`を新しい公開 domain conceptにはしません。StreamはExecutionまたはクライアント接続の実装詳細です。

### 対話端末の画面サイズ

Host-shell要求は任意の`display_language`を受け付け、Host準備前に空・`en`・`ja`だけに
限定します。クライアントで選んだ値を、そのHostセッションの`HACO_UI_LANGUAGE`へ渡します。
任意の環境変数やOSのlocaleは転送せず、通常Envのshell要求に言語フィールドはありません。
[ADR 0079](../adr/0079-host-presentation-language.md)を参照してください。

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
- クライアント専用 companionでguest コマンドのexit status/stdout/stderrが保持されること
- 生の Incus control ソケット非露出
- 通常Environmentに信頼されたコントローラー接続先とclient-mode 識別情報が存在しないこと

今後のfollow-up:

- 残る`haco` コマンドをclassifyし、適切なものをコントローラークライアント interfaceへ移行
- replacementが確立したcompatibility aliasをremoveまたは明示deprecate
- 信頼された Host-local ツールをlong-termの`haco-host` 名前空間へ移行
- 実需が出た場合のみremote 通信
- profilingで必要性が示された場合のみFD passing / zero-copy

## Environment実行とキャンセル

`environment.process` は `haco exec` と `haco env exec` の共通経路です。
既存の起動中Environmentだけを選び、所有権・generation・Workspace leaseを確認し、
削除と共通のlifecycle lockを実行終了まで保持します。停止中は失敗し、自動起動しません。
切断時はコマンドを中断しますがEnvironmentを削除・停止しません。

実行開始後にローカルで中止を確認した場合、同時に返された通信エラーや実行結果よりも
CLIの終了値130を優先します。ローカルの中止がなければ、コマンドの終了値と通常の
エラーをそのまま扱います。

実 Incus の保守対象試験は、起動した shell が `sleep` に置き換わる前に PID と Linux の
開始時刻を記録します。中止は終了値 130 を返し、既存の共通 30 秒期限内にその実行個体が
消えたことを、Environment の停止や後片付けより前に確認する必要があります。
通常の `haco exec` で不在（PID の再利用を含む）を観測し、未回収の zombie は残存と扱い、
観測不明は失敗にします。Environment の登録情報、起動状態、guest の PID1 開始時刻が
変わっていないことも確認し、停止・再起動による消失と区別します。`sleep` への移行完了や
任意の子孫プロセスの回収を保証する試験ではありません。
試験補助の単体試験だけでは実 Incus の受入成功とは扱いません。
[ソースを特定した実機結果](../status/acceptance-evidence.ja.md#environment-exec-cancellation)を参照してください。
後片付けは保存した作成記録を再確認し、不明・変更済みの対象は残します。名前を指定する
CLI に比較と削除を一体で実行する機能はないため、この照合はその時点の確認に限ります。
後片付けの失敗が起きても、先に失敗した観測結果は保持します。

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

## Environmentの双方向実行

上限付きstdin/stdout/stderr、EOF・受信可能量、明示的な終了結果を転送します。
`-i` は標準入力、`-t` はPTYを選び、`-it`も使えます。`-w`は作業ディレクトリを指定します。
端末のサイズ変更と復元を共通処理で行い、guestへIncus管理権限を渡しません。
旧一時実行の公開RPCは撤去しました。[作成仕様](environment-creation.ja.md)を参照してください。

## client側TCP待受

状態: private UDSを使うLinux clientの**実装候補**。
`haco env tunnel --target-port 8080 demo`は、通常のLinuxでは実行場所で、WSL／trusted Hostの
入口では導入済みWindowsクライアントへ自動委譲して待ち受けます。接続ごとにcontroller byte sessionへ流します。出力は接続先と次の操作を
案内します。`--listen`の既定は`127.0.0.1:0`（自動port）、`--address`はEnv内の
`127.0.0.1`、`--duration`は`1h`（`1s`〜`1h`）です。数値ループバックIPv4/IPv6を
指定でき、hostname・zone・mapped IPv6・非ループバック宛先は拒否します。
Ctrl+Cと期限切れで待受と転送中の接続を終了します。同時16接続を超える接続は
upstreamを開かずに閉じます。

`environment.forward.prepare`が作成世代付きの選択を返し、
`environment.forward.stream`がready metadata・active lease・実行状態・正確な
provider世代を検証して接続します。guest endpointには登録しません。管理socketの
既存アクセス制御を使用し、guest発のnetwork Capability権限とは独立しています。
adapterはEnvのIPを使い回さず対象namespaceを固定します。controllerは所有された
stream callback内でsocketを開き、アプリbyteより先に接続結果を返し、最終session
結果も区別します。半切断は直ちに伝え、両方向終了後に完了を待ちます。
途中のcloseはprivate管理endpointの`_control.session.cancel`で終了を求め、
stream回収後の応答を最大6秒待ちます。失敗・キャンセル時はsocketを閉じてcopy
workerの終了を待ちます。controllerへ到達できなければ遠隔の回収は未確認であり、
controller側の1時間の絶対期限が上限です。EOFを無視する接続先も、応答済みの
キャンセル後には残りません。

永続Incus転送deviceは作りません。既存の`env forward`とSSH／previewは利用できます。
Windows側待受から`wsl.exe`を通す公開companionは以下の実装候補です。導入済み
環境での結果と検証範囲は[コミットごとの受入記録](../status/acceptance-evidence.ja.md#incus-key-download)を参照してください。
通常のLinuxのtrusted Host clientはHost内で待ち受けます。対応するシェルクライアントは
管理ストリームを使い、未使用のEnvシェルの標準入出力への直接接続は削除済みです。
[ADR 0107](../adr/0107-responsibility-layout-and-cli-retirement.ja.md#維持する境界)を参照してください。
転送の契約は[ADR 0091](../adr/0091-client-stream-forwarding.ja.md)に従います。

## Windowsのプロセス転送

状態: **開発候補としてpartial**。`internal/platform/wsl/launch`が選択した同一PCのWSLに対する
固定の非表示`System32\wsl.exe`起動を組み立てます。controller転送の入力検証・最小限の環境変数をここで管理します。通知回答との起動処理の共通化は別途進めます。`haco _control-stdio`は継承した接続先設定を
使わず、Physical Hostの固定UDSだけへ接続します。通常のWSL利用者とsocketの
アクセス制御を維持し、rootへの変更・通常Envへの管理接続投影・TCP管理待受は
追加しません。型付きclientは操作と別接続のsession制御の双方で同じdialerを使います。

pipeでは上限付きデータと明示的な半切断frameを使い、pipe自体のEOFは異常切断です。
headerはkind 1 byteとbig-endian長4 byteです。データはkind `0x10`・長さ1〜32768、
EOFはkind `0x11`・長さ0です。不明kind・過大・途中切断・EOF後のframeは拒否します。
期限切れは接続全体を終了し、期限更新後に古いtimerが接続を閉じる競合を防ぎます。
キャンセルで所有pipeと起動した子だけを回収します。pipeの明示的な所有により、
子の終了時にも受信済み応答を欠落させません。接続は最大10秒、橋渡しは最大1時間です。

Windows→WSLの実fixtureによるbyte配送と導入済み製品の受入は別です。Windows側で
待ち受ける公開companion・配置とWSLの通常入口からの自動委譲は**実装候補**です。内部stdio入口を
利用者向けCLIとは扱いません。[ADR 0092](../adr/0092-wsl-process-transport.ja.md)を参照してください。

## Windowsの公開転送クライアント

`haco-tunnel.exe --distribution <WSL名> --target-port <ポート> [options] <Env名>`は
Windows上で待ち受け、固定のWSL controller接続を使います。distribution指定は転送
optionより先に置きます。Linux/Windowsで引数・対象準備・同時16接続・半切断・
最大1時間・中断回収を共有します。Incus操作、管理待受、rootへのfallback、認証情報の
取り込みは追加しません。日英の共通縦ヘルプはcontrollerなしで表示でき、失敗時は
対象WSLの`haco doctor`と接続先アプリの確認を案内します。

インストーラがamd64/arm64に対応したclientを恒久配置し、絶対path付きヘルプコマンドを
表示します。利用者のPATH変更は不要です。`haco-wsl.exe`は導入・容量回収の責務を
維持します。[配置と所有権](installer.md#windows-client-placement)を参照してください。
明示的な公開入口も利用できます。WSLの`haco env tunnel`は導入済みWindowsクライアントを
自動選択します。導入済みWindows/WSL/Incusの転送手順は`1054688e`で成功しました。
検証範囲は[受入記録](../status/acceptance-evidence.ja.md#incus-key-download)を参照してください。
より広いプラットフォームの受入と過去の未解決の失敗は別途残ります。

## SSHとTCP転送の共通処理

SSHと明示TCP転送は、上限付きの準備完了通知、byte relay、半切断、cancel、独立した最終結果を共用します。
権限判断は各serviceに残します。SSHは保存されたgrantを照合して同じEnvを再開でき、
明示TCP転送は稼働中Envを必要とします。準備期限はSSHが100秒、TCPが10秒です。
準備後の通信に準備期限を引き継ぎません。一方向終了後の応答排出は共通relayで30秒に制限します。

## Windows転送の自動起動

状態: **実装候補**。WSLの環境情報は表示先を選ぶ手掛かりであり、権限ではありません。
CLIはEnvの正確な作成世代を準備し、controllerから読み取り専用のWSL登録先・導入世代を取得して、
インストーラが管理する利用者領域のクライアントを探します。通常のLinuxはローカル待受を維持します。
連携機構・クライアント・所有記録の不足は日英で次の確認を案内し、別の場所で代わりに待ち受けません。

内部要求には選択済みEnv、ループバック待受、言語、元の絶対期限だけを渡します。
コマンド文字列・認証情報・providerの接続先は渡しません。Windows側は`wsl.exe --distribution-id`を使い、
導入世代の識別子と現在のEnvの正確な選択を照合してから待ち受けます。以後の各接続でもcontrollerの世代・lease確認を維持します。
手動の`--distribution`入口は利用者が明示した選択を使います。

要求は4 byteのbig-endian長と最大8192 byteの厳密なJSONで、読み取りは10秒までです。追加の要求は受け付けません。
親が保持する入力pipeを寿命の目印とし、EOFで待受・転送先を終了、余分なbyteは失敗にします。
親のキャンセルはpipeを閉じ、終了を待ち、10秒応答しない場合は起動した子だけを終了します。
Linux側のinterop子を端末の前面process groupから分け、Ctrl+Cは所有するCLIが受けてpipe経由で中断します。
親による直接の終了待ちは維持し、子の非zero終了を成功へ置換しません。
委譲しても元の期限を延ばしません。結果と診断出力を分離します。実Windowsの構成要素試験と、
`test/e2e/windows/tunnel.py`の通常導入経路は別の証拠です。
[ADR 0093](../adr/0093-windows-tunnel-delegation.ja.md)を参照してください。

通知のprivate reviewとWindowsの固定stdio接続も、要求や転送データを読む前に
通常ログインと同じcontroller準備待ちを使います。WSLプロセスの起動だけでは接続口の
準備完了を証明できません。接続不能時の読み取り専用pingだけを再試行し、拒否や実際の
操作は再送しません。親の中止は起動したprivateな子プロセスだけを終了します。

## リポジトリ登録の進捗

`repository.add`は管理用のストリームで、上限付きの進捗フレームと明示的な完了応答を使います。
ブランチを指定せずに取得元を登録します。`--json`でも進捗はstderr、最終結果だけをstdoutへ出します。
接続断は無出力のGit処理もキャンセルし、Incusアダプターが信頼されたagentへ割り込みを転送します。
作成に失敗した場合は復旧のための所有記録を保持します。[Gitの契約](git-and-github-capability.md)を参照してください。
