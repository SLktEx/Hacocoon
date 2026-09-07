# ADR 0027: 読み取った版に結び付けた Policy 編集

状態: accepted。repository に実装済み。installed 受け入れは未確認です。

Policy の表示は正規化した JSON を使います。revision は元の実ファイルの正確な bytes に結び付くため、空の任意配列が往復で表示上増減することを避けつつ競合を検出します。

通常の trusted 利用者が Physical Host の復旧 shell に入らず承認方針を確認・編集できるように、
`haco config` で既存 Policy の管理者 rules と saved_decisions を取得します。
`--edit` は利用者の editor、`--file` は同じ snapshot 形式を使用します。

既存 management socket は管理者権限の境界です。設定の取得・置換はそこだけに登録し、
guest Git socket や表示専用の通知 bridge には公開しません。設定管理を workload
Capability や Environment の許可にしません。Core の provider、browser の必須依存、
管理 socket の公開、credential の追加はありません。

revision はファイルの正確な bytes を hash 化し、ファイルの不在も区別します。
承認保存と同じ private な advisory lock を保持して revision を比較します。
共通 writer が文書全体と容量を検証し、危険なファイルを拒否し、private 一時ファイルへの
書き込み・同期・元内容の再確認・atomic な置換を行います。協調する writer 同士の変更は
消しません。別の管理者 editor は同じ lock に従う必要があります。追加の bytes 比較は
lock を無視する任意の process との transaction を保証しません。

変更前の意図と永続化後の完了を監査します。操作 ID と新旧 revision だけを記録し、
設定内容は出しません。事前の監査失敗では置換せず、保存後の監査失敗では成功 receipt を
返さず recovery-required にします。現在の設定を確認してから再試行します。
競合した編集は保持し、自動 rebase・再試行しません。

却下した案: 承認と設定を別ファイルにする、後勝ちの import、暗黙の rule merge、
queue 受付を永続化の証明とする、縮小した通知 event に Policy 内容を載せる、
置換対象の Policy 自身に設定変更の承認を求める設計。
