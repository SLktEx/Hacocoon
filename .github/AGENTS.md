# Hacocoon Agent Rules

Hacocoon の変更では、正常系だけでなく「どう壊せるか」を必ず考える。

## Adversarial security review

特に以下を確認すること。

- 外部入力はすべて untrusted とみなす。
- path traversal / symlink / TOCTOU を疑う。
- shell / argument / option injection を疑う。
- privilege / capability / ownership bypass を疑う。
- host / guest / backend / plugin 間の trust boundary を越える処理を重点確認する。
- secret を argv / env / log / tempfile に残さない。
- delete / overwrite / resize / detach / mount など destructive operation は対象取り違え・partial failure・retry を考える。
- 同じ操作の同時実行、逆操作との競合、timeout、cancellation、process crash 後の状態を確認する。
- backend / plugin / guest が不正・異常な値を返す可能性を考える。
- UI や上位 caller の validation だけに依存しない。

最低限、security-sensitive な変更では次を自問する。

1. `../`、absolute path、symlink を渡したら？
2. `-` / `--xxx` から始まる値を渡したら？
3. 同じ操作を同時に2回実行したら？
4. 処理途中で失敗・timeout・process crash したら？
5. retry されたら二重実行されないか？
6. cleanup が失敗したら？
7. 別ユーザー・別 resource の ID を指定したら？
8. backend / plugin / guest が hostile だったら？

Security-sensitive operation は原則 fail closed とする。

「internal API」「localhost」「VM 内」「single-user」「caller が検証済み」を安全性の根拠にしない。

脆弱性や重大バグを修正した場合は、可能な限り regression test を追加する。

詳細な敵対的監査を行う場合は [`security/ADVERSARIAL_AUDIT.md`](security/ADVERSARIAL_AUDIT.md) に従うこと。
