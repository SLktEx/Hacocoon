# プロジェクトの setup

日本語 | [English](project-setup.md)

状態: **明示的 recipe の実装済み、ロードマップ C4 の acceptance は partial**。installed GHA の検証は未完了です。

## 通常の使い方

既存の setup command に Environment を明示します。

```bash
haco setup --script ./dev-setup.sh dev
haco setup dev
haco setup --clear-script dev
```

最初の command は上限付き snapshot を保存し、dev 内で実行します。次は保存内容を再実行し、
clear は実行せず削除します。手順は canonical な Workspace identity に属し、同じ Workspace
を使う新しい Environment でも setup 一回で再利用できます。元の script は repo に保存できますが、
その編集を保存済み snapshot へ暗黙反映しません。

Environment を省略すると、既存の trusted Host setup のままです。
Host の手順・credential・管理 channel を Environment に継承しません。
repo hook の自動探索・実行は行いません。Base の tooling、任意の OCI 内容、
project の依存導入は分離します。

## 所有権と実行

既存の private・atomic な手順保存を中立な internal component として再利用します。
Host と Workspace の保存先・実行 adapter・失敗境界は分けます。
script は明示入力の UTF-8、最大 1 MiB、NUL 不可で、symlink/FIFO と不安全な
保存先を拒否する既存の保護を維持します。

canonical な Environment/Workspace catalog で対象を解決し、lifecycle guard の中で
期待する Workspace identity を検証してから実行します。名前の再利用で別 Workspace に
手順を向けさせません。競合する lifecycle 操作は待機または busy とします。

script は上限付き stdin として渡し、Host の argv や shell code にしません。
provider はこの契約への対応を明示します。local adapter は所有 Environment の
/workspace 内で、固定 transient unit と独立した実行期限・子孫 cleanup を使います。
Host credential、controller socket、Windows drive・process 経路は追加しません。
既存の DNS と egress Policy が適用されます。

保存は実行前に永続化します。失敗時は手順と Environment を保持して retry 可能にし、
成功とは報告しません。clear は過去の file 変更を元に戻しません。実行出力は上限付き
command result とし、structured log field に入れません。error・audit に script 内容を漏らしません。

## 必要な検証

save/replay/update/clear、同じ Workspace の別 Environment での再利用、
別 Workspace・再利用名の拒否、同時 setup と逆向き lifecycle 操作、
非ゼロ終了、cancel と期限付き cleanup を検証します。不正 stdin・保存先・log の
回帰は最も低い忠実な層に置きます。installed GHA は通常の haco command から
明示した nonce 手順を実行し、package 導入は範囲を限定した Policy で別に報告します。
物理端末・VPN に依存する未確認部分は SKIP とし、成功と推測しません。

[Base の境界](base-images-and-custom-environments.md#project-setup-boundary)と
[trusted Host setup](trusted-host.ja.md)を参照してください。

Workspace ごとの保存・再実行・削除、失敗後の recipe 保持、起動前の所有先確認、
controller の不正引数拒否、stdin 転送は component test で確認済みです。
Windows GHA に保存・再実行・非ゼロ終了・更新・削除の検証を追加しましたが、
この変更ではまだ実行していません。package install、cancel 後の子 process cleanup、
実 Environment 再作成後の再利用は provider acceptance として未検証です。
