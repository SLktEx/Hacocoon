# Hacocoon Adversarial Security & Bug Audit

この文書は Hacocoon に対して通常のコードレビューではなく、攻撃者・悪意のあるローカルユーザー・予想外の入力を与えるユーザー・障害を意図的に起こすテスターの立場から監査するための手順を定義する。

「普通に使えば問題ない」は安全とはみなさない。

## Core principle

コードを信用しないこと。

README、docs、コメント、設計資料に書かれた前提も安全性の証明にはならない。実際のコードとテストがその前提を保証しているか確認する。

特に次を疑う。

- 入力値は本当に検証されているか。
- 権限チェックは直接 API / internal path から回避できないか。
- TOCTOU が存在しないか。
- symlink / hardlink / path traversal が利用できないか。
- shell / argument / option injection ができないか。
- race condition がないか。
- partial failure 後に危険な状態が残らないか。
- cleanup 失敗時に resource や権限が残らないか。
- retry により処理が二重実行されないか。
- process crash / reboot / network failure 時に状態が壊れないか。
- privilege boundary が曖昧になっていないか。
- backend / plugin 境界から Core の制約を回避できないか。
- 「内部関数だから安全」「localhost だから安全」「VM 内だから安全」という前提がないか。

---

## 1. Command execution

`exec.Command`、shell、SSH、Incus CLI、git、gh、AWS CLI、nerdctl、systemd などの外部コマンド実行箇所を確認する。

確認項目:

- shell injection
- argument injection
- option injection
- `--` の不足
- PATH hijacking
- executable replacement
- environment variable injection
- inherited environment
- working directory
- stdin / stdout / stderr
- secret leakage
- timeout 不足
- cancellation 不備
- zombie process
- process group
- exit code の誤処理

文字列連結による shell 実行がある場合は特に厳しく確認する。

---

## 2. Filesystem

ファイル・ディレクトリ操作は攻撃者が path と filesystem state を操作できる前提で確認する。

- `../` path traversal
- absolute path
- symlink attack
- hardlink
- TOCTOU
- `/tmp`
- predictable temporary names
- file / directory permissions
- ownership
- atomic write
- rename
- concurrent access
- recursive delete
- mount boundary
- device file
- socket
- FIFO
- archive extraction

特に削除処理では、攻撃者が対象 path を細工した場合にホスト側の任意ファイルを削除できないか確認する。

canonicalization 前後で異なる path を検証・利用していないかも確認する。

---

## 3. Privilege boundary

Hacocoon が作る host、Environment、Workspace、backend、plugin、remote host 間の境界を確認する。

- root 権限
- sudo
- Incus socket
- Unix socket
- group permissions
- Linux capabilities
- mount
- UID / GID
- namespaces
- device access
- environment
- credentials

Environment 内で実行される command は host authority に対して untrusted とする。

Environment から host に影響を与える経路、host credential や control socket が露出する経路を探す。

---

## 4. Authentication / Authorization

特に authorization を重点的に確認する。

- caller が誰か。
- capability が何か。
- resource owner が誰か。
- requested operation が capability と一致するか。
- confused deputy が起きないか。
- horizontal privilege escalation ができないか。
- capability の取り違えがないか。
- policy bypass がないか。
- default allow / fail-open がないか。
- API を直接呼ぶことで UI 制約を回避できないか。

可能なら sensitive operation の直前で authorization を enforce し、UI / handler だけに依存しない。

---

## 5. Backend / Plugin architecture

backend / plugin を Core と同一 trust level だと仮定しない。

悪意またはバグのある backend / plugin が次を行えないか確認する。

- Core の内部状態を書き換える。
- 任意ファイルを読む / 書く。
- 任意コマンドを実行する。
- capability check を回避する。
- 他 backend / Workspace の resource を操作する。
- credential を取得する。
- cleanup を妨害する。
- Hacocoon を crash / hang させる。
- fabricated state を返して Core を誤動作させる。

新しい interface に authority を追加する場合は、悪意ある実装がその interface を自由に使った場合の最大被害を考える。

便利だからという理由だけで raw shell、host filesystem handle、root-equivalent API、credential、unrestricted network capability を渡さない。

---

## 6. Incus

Incus 操作では次を確認する。

- instance name injection
- project 間アクセス
- profile
- device
- mount
- disk
- image
- socket
- exec
- file push / pull
- privileged container / VM settings
- cleanup
- stale instance
- resource leak

特に host filesystem の bind mount、Incus socket、host HOME、`~/.ssh`、`~/.aws`、GitHub token、Hacocoon control state を Environment に露出させる変更は高リスクとする。

Incus CLI の引数として渡しているだけで安全だと判断しない。

---

## 7. Storage and destructive operations

filesystem、Btrfs、loopback、block device、mount、volume 等を扱う場合はデータ消失方向から確認する。

- device の取り違え
- wrong filesystem
- wrong mount
- resize failure
- partial resize
- balance failure
- detach failure
- loop device reuse
- concurrent operation
- stale state
- host filesystem 誤操作
- destructive retry

次を destructive operation とみなす。

- remove
- overwrite
- truncate
- resize
- block device manipulation
- detach
- mount / unmount
- Environment delete
- volume delete
- credential revoke
- remote resource delete

これらでは少なくとも対象 identity、ownership、current state、idempotency、retry、partial failure、rollback、concurrent operation を確認する。

---

## 8. Concurrency / State machine

状態を変更する処理は次の adversarial sequence を考える。

- same operation x2
- create + delete
- start + stop
- attach + detach
- timeout 直後の retry
- cancellation
- process kill
- daemon restart
- host reboot 相当の中断

探すもの:

- race condition
- deadlock
- double delete
- double attach
- duplicate side effect
- stale lock
- broken state
- leaked resource
- inconsistent metadata

lifecycle を変更する場合は少なくとも次の途中状態を考える。

```text
creating
created
starting
running
stopping
stopped
deleting
deleted
failed
unknown
```

任意の地点で process が死んでも再実行または reconciliation により安全な状態へ戻れるか確認する。

---

## 9. Error handling

error path を正常系と同じ重要度で確認する。

特に次の pattern を探す。

```go
_ = dangerousOperation()
```

```go
if err != nil {
    return nil
}
```

確認項目:

- ignored error
- wrong error propagation
- cleanup error ignored
- rollback failure
- partial success
- misleading success
- incorrect retry
- error wrapping による判定失敗
- fail-open

cleanup 自体が失敗した場合に何が残るかまで確認する。

---

## 10. Go-specific review

Go コードでは特に次を確認する。

- goroutine leak
- channel deadlock
- channel close race
- context cancellation
- `context.Background()` の乱用
- mutex misuse
- data race
- concurrent map access
- defer の誤用
- nil dereference
- integer conversion
- unsafe
- filesystem mode
- HTTP body close
- unbounded read
- unbounded goroutine
- panic
- recover の乱用

利用可能な範囲で `go test -race ./...` を実行する。

---

## 11. Dependency / Supply chain / CI

単純な CVE scan だけで終わらせない。

- GitHub Actions permissions
- floating action tags
- pinned revisions
- third-party actions
- `curl | sh`
- binary downloads
- checksum / signature validation
- archive extraction
- dependency confusion
- malicious archive path
- unsafe updater
- credentials exposed to CI
- excessive `GITHUB_TOKEN` permissions
- untrusted PR data reaching privileged jobs

---

## Adversarial test strategy

既存テストを実行するだけでは不十分。

疑わしい箇所を見つけたら可能な限り再現テストを追加する。

優先順位:

1. unit test
2. integration test
3. race test
4. adversarial regression test
5. fuzz test
6. E2E test

parser、path、identifier、API input、command argument は fuzzing 候補とする。

破壊的な reproduction は real host / real user data に対して行わず、fake、mock、temp directory、isolated test fixture を使う。

---

## Mandatory adversarial questions

security-sensitive な変更では最低限次を自問する。

1. 攻撃者が自由に制御できる値はどれか？
2. 空文字、最大長、Unicode、制御文字、特殊文字では？
3. `../` を渡したら？
4. absolute path を渡したら？
5. `-` / `--xxx` から始めたら？
6. symlink / hardlink にしたら？
7. 同じ操作を同時に2回実行したら？
8. 逆操作と競合させたら？
9. 実行途中で process が死んだら？
10. timeout 後に retry したら？
11. cleanup が失敗したら？
12. backend が嘘や stale state を返したら？
13. plugin が悪意を持っていたら？
14. Environment が hostile だったら？
15. 別ユーザー / 別 Workspace の resource ID を指定したら？
16. credential が存在しなかったら？
17. credential が想定より強い権限を持っていたら？
18. disk が満杯なら？
19. filesystem が read-only なら？
20. network が途中で切れたら？
21. OS reboot が処理途中で発生したら？
22. request が再送されたら？

危険な結果になる場合は root cause まで追う。

---

## Dangerous-code review triggers

次のようなコードを追加・変更した場合は追加確認する。

```go
exec.Command("sh", "-c", ...)
exec.Command("bash", "-c", ...)
os.RemoveAll(...)
os.Chmod(..., 0777)
context.Background()
go func() { ... }()
_ = dangerousOperation()
```

さらに以下も trigger とする。

- command 周辺の string concatenation
- user-controlled filesystem path
- unrestricted environment inheritance
- raw socket exposure
- privileged Incus device
- host bind mount
- arbitrary backend / plugin callback
- credential materialization

これらを一律禁止するものではない。使用する場合は「なぜ攻撃者入力でも安全か」を説明できることを要求する。

---

## Finding validation

「怪しい」で終わらせない。

可能な限り各 finding について次を示す。

1. 攻撃条件
2. 入力
3. 実行経路
4. 問題が起きる理由
5. 実際の影響
6. reproduction
7. fix
8. regression test

最初は危険に見えたが実際には防止されていた場合も、その防御条件を記録する。

---

## Severity

### CRITICAL

host compromise、任意コード実行、重大な権限昇格、広範囲なデータ破壊。

### HIGH

authorization bypass、他 Workspace / Environment へのアクセス、credential leakage、限定的な host 操作。

### MEDIUM

DoS、resource leak、state corruption、重大な race condition、限定条件での情報漏洩。

### LOW

defense-in-depth 不足、危険な設計、将来脆弱性になりやすい実装。

単にベストプラクティス違反というだけで severity を上げない。

---

## Fix policy

明確な脆弱性・バグは可能な範囲で修正する。

ただし大規模 architecture redesign を監査のついでに行わない。必要なら次を分けて提示する。

- problem
- attack / failure path
- minimal fix
- fundamental design fix

security fix と無関係な refactor を同じ変更に混ぜない。

脆弱性や重大バグを修正した場合は、可能な限り regression test を追加する。

```text
bug discovered
  -> fix
  -> regression test
  -> generalize lesson
  -> update security guidance if necessary
```

---

## Final report format

### Executive summary

```text
CRITICAL: N
HIGH: N
MEDIUM: N
LOW: N
```

### Findings

各 finding:

```text
ID:
Severity:
Component:
File:
Lines:

Problem:

Attack / failure scenario:

Impact:

Reproduction:

Root cause:

Fix:

Regression test:

Status:
- fixed
- confirmed
- needs design decision
- false positive
```

### Architecture risks

直ちに exploit できなくても、将来問題になりそうな trust boundary、privilege boundary、API / interface design を記載する。

### Tested attacks

実際に試した adversarial input、failure injection、race test を一覧化する。

### Remaining uncertainty

環境不足などで検証できなかった項目を必ず記載する。

例:

- real Incus unavailable
- root operation not testable
- SSH backend unavailable
- Kubernetes unavailable
- EC2 unavailable
- kernel-specific behavior not testable

検証できなかったことを「安全」と判断しない。

---

## Second-pass hostile review

一通り調査・修正した後、同じコードベースをもう一度、最初のレビュー担当者が見落とした脆弱性を探す「2人目の攻撃者」として再レビューする。

最初の findings をなぞるだけではなく、別の attack surface から再出発する。

特に:

- 最初のレビューで安全と判断した箇所をもう一度疑う。
- 修正そのものが新しい脆弱性、race、DoS、compatibility issue を作っていないか確認する。
- 1つの finding が同じ class の別箇所に存在しないか横展開する。

目的はコードを綺麗にすることではない。

Hacocoon を壊す方法、権限境界を突破する方法、データを壊す方法、想定外状態に持ち込む方法を探し、それができないことをコードとテストで確認する。
