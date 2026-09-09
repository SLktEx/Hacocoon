# Environment の持ち出し

状態: 公開 export/import は **planned** です。native custom volume の受入テストは内部の前提確認であり、
利用可能な Hacocoon importer ではありません。

## Incus を土台にする

rootfs と custom volume の独立 archive は Incus export/import を使います。instance archive には
接続した Workspace／OCI custom volume の内容が含まれません。Hacocoon は archive と用途の対応を管理し、
必要な対象がすべて保存されたことを確認してから export 完了を返します。Base の名前・fingerprint は由来だけで、
追加の Base filesystem component は不要です。
[instance backup の範囲](https://linuxcontainers.org/incus/docs/main/howto/instances_backup/)と
[custom volume backup](https://linuxcontainers.org/incus/docs/main/howto/storage_backup_volume/)を参照してください。

通常のファイル archive は pool 間で持ち出せます。optimized archive は対応する storage driver に依存し、
速度だけの違いとして扱いません。後続の壊れた storage からの退避は別の作業であり、optimized export や
新規 snapshot の成功を前提にしません。

## データと権限

将来の公開 import は canonical lifecycle の所有確認を使って新しい管理資源を作り、Workspace の Git 状態と
保持対象 OCI データを保存します。既存 Environment の上書きや、古い承認・接続・管理権限の復元は行いません。
Host の認証情報と control socket は Environment export の対象外です。archive checksum は変更検出であり、
設定を適用する権限ではありません。import された owner label も元の metadata であり、新しい lease ではありません。

専用実機の Incus 6.0.5 CLI には、最新 upstream 文書にある instance import の config/device 上書き引数がありません。
新しい flags を仮定したり、古い設定のまま起動してから修正したりしません。rootfs archive の検証、現行 security の
再構成、ファイル path の扱い、全対象の公開処理は実装・テストが必要です。

## Native custom volume の受入

`TestRealIncusVolumeTransferE2E` は専用 root Linux/WSL Incus+Btrfs 上で
`HACO_E2E_INCUS_VOLUME_TRANSFER=1` を指定した場合だけ実行します。新しいランダム名の2 pool と合成 `work`／`oci`
volume を使います。runner は Incus の mount を見られる必要があり、daemon が private mount namespace を使う場合は
同じ呼び出し内で確認した現在の namespace に入ります。作成前に `/var/lib` へ正確な所有対象を記録します。記録と archive は両 pool の外に保持します。
失敗時は明示的な確認用に資源を残します。cleanup は試験用所有 marker と pool の空を確認してから pool を削除します。
既存 Workspace／OCI Store／snapshot／共有 image／pool は選択しません。

non-optimized の `--volume-only --compression=none` export と第2 pool への import を使い、未push commit、
未commit・untracked、hardlink、symlink、file mode、独立した書き込み、archive 不変性、保存元削除後の独立性を確認します。
native import が古い user-config marker を引き継ぐことも確認し、Hacocoon で新しい所有権が必要な点を明示します。

rootfs import、UID/GID・拡張属性の網羅、実 Docker/containerd 内容、公開コマンド、Windows への成果物保存、cross-host は
未検証です。volume テストの成功を G1 全体の完了とは扱いません。

専用実機の初回は export 前に失敗しました。fixture が volume path の `default_` prefix を欠き、
daemon の mount namespace 外で実行していたためです。正確な試験資源を保持し、owner marker と空一覧の照合後に削除しました。修正後、現在の daemon namespace 内での
Incus 6.0.5／Btrfs 検証は11.24秒で成功し、両試験 pool の cleanup も成功しました。記録と2 archive は
`/var/lib/haco-volume-transfer-2481101147` に保持し、初回失敗の記録も `/var/lib/haco-volume-transfer-3035986437` に残しています。
