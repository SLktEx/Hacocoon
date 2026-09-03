# Windows / WSL disk compaction

Hacocoon は WSL の experimental な sparse VHD mode を自動で有効化しません。Windows host 上で実際に使わなくなった WSL VHD の領域を回収したい場合は、明示的に次を実行します。

```powershell
haco maintenance compact
```

Windows installer 完了時に `%LOCALAPPDATA%\Hacocoon\bin` へ Windows-side `haco` launcher を配置し、user `PATH` に追加します。新しい terminal を開いた後に利用できます。この Windows launcher が担当するのは現在 `maintenance compact` だけです。通常の Hacocoon CLI 操作は専用 WSL / trusted `haco-host` 側で実行します。

## 処理順

`haco maintenance compact` は次の順序を守ります。

1. current Windows user に登録された Hacocoon 専用 WSL distribution を特定する
2. 利用可能なら guest の `fstrim` を実行する
3. `wsl --terminate <Hacocoon distribution>` で対象 distribution **だけ**を停止する
4. WSL の running list から消え、対象 `ext4.vhdx` の exclusive open が可能になるまで待つ
5. Windows host で `Optimize-VHD -Mode Full` が利用可能ならそれを使い、なければ system `diskpart.exe` の `compact vdisk` を使う
6. compaction 前後の VHD file size と回収量を表示する
7. 対象 distribution が再び mount / read-write できることを検証する
8. 検証後は対象 distribution を再度停止状態に戻す

Host-side compaction に administrator 権限が必要な場合、UAC elevation の対象は Windows の compaction process だけです。

## やらないこと

この command は background では実行されず、installer 実行時にも compaction しません。また global `.wslconfig`、他の WSL distribution、experimental な sparse VHD 設定を変更しません。

Compaction 前に対象 VHD が完全に offline にならない場合は処理を中止します。Guest trim、distribution stop、host compaction のいずれかが失敗した場合も、対象 distribution を unregister したり user data を削除したりしません。
