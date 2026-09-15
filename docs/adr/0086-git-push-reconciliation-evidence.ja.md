# Git pushの永続記録と読み取りによる照合

日本語 | [English](0086-git-push-reconciliation-evidence.md)

状態: 開発候補で採用。

## 決定

リモートの書き込みと手元の応答保存は同時に確定できない。Standard Git brokerは共通の同期・fsync付き監査へ送信前のgit-push-startedを保存し、対象refのporcelain結果を検証した後だけgit-push-confirmedを保存する。要求、Env作成、取得元所有権、登録remote、ref、旧新OIDを固定する。要求IDは照合用であり、許可ではない。

送信前記録が保存できなければ送信しない。送信応答・後続監査の失敗は保持し、自動再送しない。共通操作の完了とprovider確認記録は別の事実。中断した承認は復元しない。開始記録のない要求は未確認のままにし、照合先の権限には使わない。

信頼されたgit.statusとgit.reconcileはEnvの最新pushを既定とし、任意のrequest指定で過去の記録も確認する。既存のサイズを固定した監査読み取りを使い、完全な改行区切りJSONLを要求する。破損はエラーで、別のjournalやguest向け復旧endpointを作らない。

照合は実行中の要求を拒否し、元のEnv作成・取得元所有権・現在の接続を再検査する。対象refに対する新しいfetch Policy/承認を要求し、待機後も作成IDを確認して、共通の取得元registry lockとbackendでls-remote --headsのみを行う。観測は新しい読み取り要求として監査する。書き込み、承認復元、認証情報の持ち出し、ライフサイクル変更を含めない。

## 解釈と不採用の案

リモートOIDの一致だけでは誰が書いたか分からず、変更後に元へ戻った可能性もある。matches-new / matches-old / absent / divergedの観測と、元のconfirmed / unconfirmedを分ける。結果不明のpushを成功へ書き換えない。次のpushは通常の新しい提案・Policy・旧OIDまたは未存在の確認を使う。

切断後の自動再送、同じOIDだけで成功扱い、監査から承認を復元、過去の名前を現在の別所有対象へ結び付ける処理、別の変更可能な復旧状態は不採用。監査は事実、現在の登録とPolicyは権限を担う。force/複数ref push、削除済み対象の復旧、監査修復は対象外。

## 検証範囲

実Gitを使うローカルfixtureは、書き込み後の応答喪失、broker再作成、競合する同一commit作成、新しい読み取り拒否、作成ID・所有権、各監査保存失敗を確認する。認証付き外部Gitや導入済みWindows/WSL受入とは区別する。[Gitの案内](../guides/git-workflow.ja.md)を参照。
