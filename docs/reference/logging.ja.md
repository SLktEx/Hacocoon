# Logging

[English](logging.md) | **日本語**

HacocoonはCore、provider、network、storage、plugin、CI/E2Eを横断して障害を追跡するためにstructured loggingを使います。Loggingはobservability infrastructureであり、credentialやtrust boundaryを弱めずに「どのoperationが、どのlayerで失敗したか」を分かるようにするものです。

## 原則

1. 実装を逐語的に実況せず、意味のあるoperationとstate transitionを記録する。
2. 検索・filterしたい情報はfree-form messageだけへ埋め込まず、structured attributeを使う。
3. 通常の障害はDEBUGなしでもINFO/WARN/ERRORから大枠を判断できるようにする。
4. DEBUGだけsecretの扱いを緩めてはいけない。全levelで同じsecret ruleを適用する。
5. Loggingの成否によって本来のoperation結果を変えない。
6. 失敗したoperationは通常、そのoperationをreportするboundaryでERRORを1回だけ記録する。下位layerはerrorをreturn/wrapし、必要ならDEBUG diagnosticを追加する。

## Level

標準の `log/slog` levelを使います。

- `DEBUG`: sanitize済みHost command、retry、provider/backend step、内部state transitionなどの詳細diagnostic。
- `INFO`: Environment create/exec/deleteの開始・完了など、意味のあるlifecycle event。
- `WARN`: requested operationを継続できるが、fallbackやdegraded behaviorが発生した場合。
- `ERROR`: requested operationを成功完了できない場合。

defaultは `INFO` です。

現在のexecutable設定はenvironment variableで行います。

```bash
HACO_LOG_LEVEL=debug haco doctor
HACO_LOG_FORMAT=json HACO_LOG_LEVEL=debug haco create --workspace /work demo
```

`haco`、`haco-vscode`、`haco-agent-host`、`haco-notify` は同じ設定を使います。formatは `text`（default）と `json` をsupportします。Logはstderrへ出し、stdoutのcommand outputをmachine-consumableなまま保ちます。

## Stable structured field

同じ意味にpackage固有の別名を増やさず、既存fieldを使います。

| Field | 意味 |
|---|---|
| `component` | `core`、`incus`、`network`、`storage`、`git`、`oci`、`proxy`、`host` などのsubsystem |
| `operation` | `create_environment` などのstable operation名 |
| `environment_id` | Hacocoon Environment identity |
| `runtime_ref` | safeかつdiagnosticに有用なprovider/backend runtime reference |
| `backend` | disambiguationが必要な場合のprovider/backend |
| `duration_ms` | operationのwall-clock elapsed time |
| `attempt` | retry/attempt番号 |
| `request_id` | request/capability correlation identity |
| `error` | failure reportを所有するlayerでのsanitize対象error |
| `exit_code` | child/Environment commandのexit code |
| `target_host` / `target_port` | normalize済みegress target。full URL/path/queryは記録しない |

任意object dump、filesystem全体、unbounded provider outputよりstable identifierを優先します。

## Logger ownershipとpropagation

process root loggerはexecutable entrypointがconfigureします。internal packageが無関係なglobal loggerを個別に作ってはいけません。

operation-specific attributeは `context.Context` でpropagateします。下位layerはそのcontextからloggerをderiveし、自分の `component` などを追加します。これによりdomain contractへloggerを持ち込まず、同じoperationをCoreからprovider/Host commandまでcorrelateできます。

```text
root logger
  -> operation context
      -> core
          -> incus / network / storage / git / oci / proxy / host
```

## Secretとsensitive data

DEBUGを含む全levelでsecretをlogしてはいけません。

対象には次を含みます。

- password / passphrase;
- access / refresh / bearer / approval / session token;
- Git credential / credential-helper output;
- SSH private key;
- API key;
- cookie;
- `Authorization` / `Proxy-Authorization` value;
- proxy credential;
- credential-bearing URL;
- secretを含むenvironment variable / config value。

HTTP header全体、process environment全体、任意config object、request/response body、raw child stdout/stderrをdebug目的で丸ごとlogしてはいけません。

shared logging handlerはknown secret-shaped valueをdefense-in-depthでredactします。ただし、これはarbitrary sensitive objectをloggerへ渡してよいという意味ではありません。call site側でもsafeなfieldだけを明示的に選びます。

安全か判断できない値はlogしません。

## Host command logging

trusted Host commandはdiagnosticに有用な場合のみDEBUGで記録できます。shared Host runnerは次を行います。

- executableとsanitize済みargvを記録する。
- common commandを `incus` / `network` / `storage` / `git` / `oci` / `host` componentへ分類する。
- durationとexit codeを記録する。
- captured stdout/stderrは自動でlogしない。

sanitize済みlogの横にraw command lineを追加してはいけません。credentialを持つ可能性があるargumentはomitまたはredactしてからemitします。

## Error

同じfailureをcall stackの各layerでERRORにしません。

推奨flow:

```text
provider/Host layer -> errorをreturn/wrap + 必要ならDEBUG diagnostic
Core operation boundary -> operation/environment/duration付きでERRORを1回
CLI -> returnされたerrorをuserへ表示
```

retry、fallback変換、その他のhandlingで回復したerrorは自動的にERRORではありません。fallbackでbehaviorが意味的に変わる場合はWARNが適切です。

error value自体にuntrusted backend textが含まれる場合があります。shared loggerはcommon credential patternをredactしますが、そもそもsecretを含むerrorを組み立てないことを優先します。

## Durationとexternal operation

latencyがfailure classificationに役立つoperationでは `duration_ms` を記録します。特に:

- Environment create/exec/delete;
- Incus lifecycle;
- image acquisition / Seed construction;
- network/storage initialization;
- Git fetch/push;
- cleanup / recovery。

trivialなin-memory helperへhigh-cardinality timing logを追加しません。

## CI / E2E

CI logから最低限、次を切り分けられることを目標にします。

1. runner/setup failure;
2. Incus substrate failure;
3. Hacocoon provider/backend integration failure;
4. Core Environment lifecycle failure;
5. network/proxy/DNS failure;
6. storage / optional plugin failure。

automation向けにJSON outputを利用できますが、testはhuman-readable text formatへ依存してはいけません。specialized CI diagnostic artifactは引き続き有用で、application loggingで置き換えません。

CIでDEBUGを有効にしてもredaction/secret handlingを弱めません。

## Logを追加・変更するとき

追加前に確認します。

- operationalに役立つeventか。
- levelはこのdocumentと一致しているか。
- message textではなくstructured fieldにできないか。
- 他layerが所有するERRORと重複しないか。
- credential、request body、private key、token、header、arbitrary subprocess outputが混ざらないか。
- field nameをCI/debugging toolが使える程度にstableに保てるか。

新しいredaction rule、field contract、format behavior、failure boundaryを導入するlogging changeにはfocused testを追加します。
