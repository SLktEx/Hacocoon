# Environmentのリソース制限

[English](sandbox-resource-limits.md) | 日本語

状態: **実装済み（プロバイダー・旧CLIの範囲）**。実Incusでの広い制限確認は未完了です。
製品`haco env create/run`にはこのbudget フラグはありません。
[CLI移行情報](../reference/cli-migration.md)で移行用の境界を確認してください。

ResourceBudgetはCPU、MemoryBytes、PIDs、RootBytesを持ちます。
Env内の消費量を制限するもので、Host境界を越えるCapabilityではありません。
各値は正の有限値または`unlimited`、省略時は無制限です。
不正・ゼロ・負数・overflow・曖昧・非対応の値は拒否します。

以下はPhysical Hostの旧CLI用であり、導入済み製品の通常手順ではありません。

```bash
hacoq create --cpu 4 --memory 8GiB --pids 1024 --root-size 40GiB --workspace /absolute/work dev
hacoq run --cpu 2 --memory 4GiB --workspace /absolute/work -- go test ./...
```

CPUとPIDは正の整数です。容量はparserの二進単位を使い、
曖昧な略記より`MiB`/`GiB`を明示します。
作成時の有効budgetを永続化し、Base選択や信頼しないエージェントで上限を上げることはできません。

Incus アダプターは停止中に有限値を適用し、読戻しで確認してから起動します。
正規の作成処理では、後続のデバイス・resource設定より**前**にnative所有記録を保存します。
後始末は不存在確認までリースを保持します。適用・確認・起動・永続化の失敗を、
制限付き作成の成功として報告しません。

プロバイダー固有のkeyはアダプター内に置きます。指定した有限値を強制できないプロバイダーは
作成を拒否し、無視や弱いplatform動作へ切り替えません。
プロバイダー境界は保持していますが具体的なcloud実装は延期中で、
稼働中のEC2/EBS制限契約はありません。

RootBytesは強制可能なrootfsを対象とし、任意のWorkspace マウントのquotaではありません。
cluster scheduling、自動増減、稼働中の上限変更、Host Workspace quota、
任意のプロバイダー設定引渡しは対象外です。
[所有権のライフサイクル](../adr/0002-environment-lifecycle-ownership.md)も参照してください。
