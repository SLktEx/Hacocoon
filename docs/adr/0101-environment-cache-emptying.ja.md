

# ADR 0101: Env内キャッシュの掃除

状態: 開発候補として採用。

## 判断

使い捨てキャッシュを空にしても、Envの所有関係・Workspace・OCI・共通世代・保存コピーは保持する。
正確な停止中Envを共通lifecycleで固定し、子resourceの`clearing`を先に永続化する。
providerによる内容と所有権の再確認後だけ`ready`へ戻す。失敗時も所有権を残し、明示的再試行まで
通常の起動・接続・snapshot・copy・削除を止める。同じEnvの逆操作は共通ロックと台帳で排他する。

LinuxはIncus volume file APIで子だけを削除し、リンクをたどらない。他のproviderも同じ契約を満たす必要がある。

## 採用しない案

volumeを削除して再作成すると、失敗時にattachmentと所有対象を失う。guestのrm実行は改変可能な
プログラムを管理側操作に使う。Host mount先推測はprovider境界を破る。曖昧な結果で状態を解除すると
中途半端な内容で作業が再開されるため採用しない。古い形式の互換機構は追加しない。

[所有仕様](../design/cache-generations.ja.md#env内のキャッシュを空にする)を参照。
