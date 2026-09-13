# PR の検証契約

日本語 | [English](ci-contracts.md)

main 向け PR は4系統の検証 workflow をすべて実行する。path/type filter を置かず、
共有 package、module、依存関係、配布物、client、新しいディレクトリの変更でも
installer 受入が漏れないようにする。否定例を含む同等の検証を備えた依存関係ベースの
選択方式ができるまでは、実行時間より対象漏れの防止を優先する。

実行可能な job 一覧は [ci_contracts.json](../../tools/ci_contracts.json) が所有する。
[検査ツール](../../tools/check_ci_contracts.py) は PR trigger の欠落、filter、必須 job の
欠落・条件分岐・失敗許容、必須契約 step の省略、証拠 job の依存漏れを拒否する。既存の workflow-policy と
local CI から実行する。権限境界は [CI trust boundary](../../.github/security/CI_TRUST_BOUNDARY.md) に従う。

## 必須検証と範囲

| 変更・契約 | PR workflow / 必須 job | 実際の実行境界 |
|---|---|---|
| Core、CLI、Policy、承認、Git broker、共有 package | `test`: test matrix、race、e2e | unit/component/integration と隔離 fixture を使う出荷 process。ローカル Git transport 拒否は認証済み GitHub push の成功を意味しない。 |
| 配布・release 構成 | `test`: release-config、build | Linux 両 architecture、GoReleaser、installer archive を merge 前に検証。 |
| Incus lifecycle、network、egress | `incus-core-e2e`: incus-standalone、incus-core-e2e | 独立 runner の native 基盤、provider、出荷 controller/CLI。standalone の Docker 共存ルールを製品 Environment の egress に適用しない。 |
| Btrfs、Base、snapshot、transfer、保持 OCI | `incus-core-e2e`: incus-owned-btrfs | native adapter と CLI/controller。fixture は保持データを準備し、pool の遅延作成も検証する。全 packaged 受入ではない。 |
| Ubuntu installer・隔離 | `ubuntu-installer-e2e`: ubuntu-user-path | 未改変の配布 installer、通常ユーザー、製品 CLI の run/create/status/stop/start/delete と Workspace 保持、移行中 CLI の installed journey と kernel 検証。 |
| Windows/WSL install・restart・reinstall | `windows-installer-e2e`: windows-user-path | BAT、ConPTY、通常 WSL/Host entry、reinstall 前の terminate、Host データ保持。phase 時間を記録する。初期 driver 単独では Environment のデータ保持を証明しない。 |
| SSH・IDE・network・transfer・reclamation・通知 | 同上 | 初期 BAT に続く installed egress と native Windows/OpenSSH/VS Code の実際の検証 step が必要。 |

branch protection には既存 check に加え `test-evidence`、`incus-core-e2e-evidence`、
`ubuntu-installer-e2e-evidence`、`windows-installer-e2e-evidence` を必須として設定する。
リポジトリ内のコードだけでは GitHub の保護設定は変更されない。証拠 job は `!cancelled()`
で必要 job と契約 step の実際の success を確認し、成功 job 内の skip・取消・欠落・失敗も成功として扱わない。履歴・artifact の
取得失敗も失敗であり、native 試験を focused probe の成功で代用しない。matrix の architecture や Go 系列ごとの成功記録も要求し、needs の集約成功で検証対象の削除を隠さない。

依存 job の失敗・skip 後も証拠検査を行う。workflow 全体の取消には従い、古い候補の
証拠 job が runner 待ちのまま concurrency 枠を保持しないようにする。既存の必須 check
も保持する。取消済み workflow は検証成功を証明しない。job-level の `always()` は
[GitHub の取消処理](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-cancellation)
でも継続する場合があるため、ここでは使用しない。

## 失敗と再実行

step の `[product]`、`[fixture]`、`[infrastructure]` は失敗した操作の責務境界を示す。
根本原因の断定ではない。product step で外部基盤障害が見つかる場合もあるため、診断を
調べて原因を決める。過去の分類のない step は `unclassified` のまま残す。

[ci_history.py](../../tools/ci_history.py) は読み取り専用 token で全 attempt とページ分割
された job metadata を取得する。各 workflow は workflow、source SHA、検証 SHA、run、
attempt、job、失敗 step、責務境界、同一 SHA の red→green を含む `ci-evidence.json` を
30日保持する。同じ workflow/event/source SHA の別 run や PR 再開も照合し、部分的な
再実行の成功で元の失敗を消さない。native Go コマンドは `ci-test-results.jsonl` に
期待するテスト名、PASS/FAIL/SKIP または結果欠落、終了コードと run の識別子も保持する。

失敗 attempt がある source SHA の証拠 check は失敗し続ける。自動免除や green 化する
retry はない。原因を修正するか、調査済み基盤障害の解決根拠を新しい commit に残す。
runner image や入力の変化も比較する。同じ source SHA だけで同等環境とは判断しない。
取消は flake と断定しないが、今回の必須 job が取消なら合格しない。

読み取り専用 `GITHUB_TOKEN` と `GITHUB_REPOSITORY=SLktEx/Hacocoon` を環境に設定して実行する。

```bash
python3 tools/ci_history.py --recent 100 --output /tmp/ci-evidence.json
```

直近最大100 run と、それぞれの全 attempt/job を調べる。全履歴の完全性は主張しない。
重要な結果と Issue リンクは [受入証拠](../status/acceptance-evidence.ja.md) に保持し、
Actions の保存期限前に report を取得する。credential、任意の process log、config dump、
外部 artifact 本体は report にコピーしない。

## 同期と隔離

Incus job は独立した使い捨て runner で、共通の署名済み LTS source と上限時間付き
`admin waitready` を使用する。他 job の準備状態を引き継がない。#607 の並列化の形を
実装し、#463 の Windows 所要時間目標は引き続き別の責務とする。

必須 provider test は名前を明示し、ちょうど1回 PASS したことを検証する。空の test
選択、前提不足の skip、command 失敗は成功ではない。通常の repository test で opt-in
native test が skip されることと、専用の必須 native job は区別する。

PTY 回帰は foreground command の出力を待ってから resize する。前の command 出力だけ
では readline の端末状態復元完了を証明できない。guest の state/address/DNS は条件で
確認し、poll 間隔を同期の根拠にしない。deadline は最後の失敗上限とする。Windows の
Host entry エラーは操作を retry せず driver を終了する。installer が所有する WSL 再起動は固定750ms待ちを使わず、正常な停止状態一覧を観測する。project cleanup は正常な一覧取得と
削除後の不存在確認を必要とし、query 失敗を削除許可や cleanup 成功にしない。

## 外部依存

| 入力 | 制約・残る変動 |
|---|---|
| Actions | 完全 SHA 固定、PR は読み取り権限、secret を持つ workflow への信頼の橋渡しなし。 |
| Runner | Ubuntu 26.04 / Windows Server 2025 を明示。image build・kernel・同梱 package は GitHub が更新する。正確な image は Actions setup log に残る。VM 全体の不変性は保証しない。 |
| Go | 1.26.7 / 1.27.0、`GOTOOLCHAIN=local`、check-latest 無効。unit shuffle seed は615、race も実行。 |
| GoReleaser・OCI fixture・VS Code・ConPTY | 既存の正確な version、OCI archive SHA256、portable VS Code の検証、pywinpty 3.0.2。 |
| Incus CI 基盤 | 共通の署名済み Zabbly 7.0 LTS、鍵 fingerprint と server version 範囲の確認。LTS patch package は変動する。 |
| installer apt | 出荷 Ubuntu installer と repository をそのまま使用する。package mirror/version は外部入力であり、事前 install で製品の準備処理を隠さない。 |
| Ubuntu container / WSL image | 製品 Base と出荷 WSL metadata/hash 検証を使用。信頼された cache と検証付き download fallback を保持。alias/metadata は変動し、失敗時の比較対象となる。 |
| Internet・認証サービス | installed connectivity/Base の一部は外部可用性に依存する。認証 GitHub push は専用 credential が必要で、PR の非信頼コードには渡さない。 |

## その他の受入と残る制約

main の repository/native 検証、manual `real-git-push-e2e` と private-registry 検証は
互換性を追加する。`windows-wsl-image-cache` は download cache で、受入ではない。
公開・attestation・公開済み artifact の install は #370 の release 境界に残る。
互換性・stress の追加層を、直接変更した契約の PR 検証の代わりにしてはならない。

認証 GitHub push に相当する credential 不要の出荷 PR 成功経路はまだない。hosted image、
apt、製品 image alias も完全には固定されていない。Windows restart と maintenance の
既存失敗は調査が必要であり、成功と読み替えたり PR から外したりしない。静的検査と
repository test だけで Issue #615 の native 再現性条件を満たしたとは判断できない。
candidate/run に結び付いた結果を受入証拠に記録する。
