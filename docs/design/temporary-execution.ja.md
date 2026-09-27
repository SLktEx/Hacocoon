# 内部の一時実行

日本語 | [English](temporary-execution.md)

一時実行はImage build・OCI保守の内部機構です。通常操作は
[open/create](environment-creation.ja.md)を使います。内部呼出しも正規作成経路・
creation identityによる所有確認・期限付き後始末を使います。

一時Workspaceには内部の破棄可能なデータだけを置き、既存ユーザーデータを名前だけで
流用・削除しません。provider不在を確認してから所有記録とleaseを解放します。
削除不明時は復旧が必要な所有記録を保持します。キャンセルとは独立した期限付きcontextで
後始末し、生存する所有者のlockが再起動時の誤削除を防ぎます。

出力は共有ログ規則に従って制限・秘匿し、コマンド失敗と後始末失敗を区別します。
[一時所有](../adr/0020-runtime-owned-temporary-workspaces.md)と
[ライフサイクル所有](../adr/0002-environment-lifecycle-ownership.md)を参照してください。
