package cliui

// Configuration messages describe receipts without translating Policy values.
var configurationCatalog = map[string]translation{
	"config.capabilities_hint":        {"Use haco config capabilities (or add --json) to discover capability actions and required Policy attributes.", "haco config capabilities（必要に応じて --json）でCapabilityの操作とPolicyの必須属性を確認できます。"},
	"config.capabilities_help":        {"List registered Policy capability schemas without granting permission", "登録済みCapabilityのPolicyスキーマを表示する（許可は与えません）"},
	"config.capabilities_unavailable": {"haco: capability metadata is unavailable; check the trusted Host controller version and connection.", "haco: Capabilityの定義を取得できません。信頼されたHostのコントローラーのバージョンと接続を確認してください。"},
	"config.capabilities_readonly":    {"Metadata only: no Policy changes, approvals or operations. Schemas describe required authority attributes, not credentials or current availability. JSON and schema descriptions are language-independent.", "定義の表示のみです。Policyの変更・承認・操作の実行は行いません。必須の権限属性を示すもので、認証情報や現在の利用可否ではありません。JSONとスキーマの説明は言語によらず共通です。"},
	"config.unconfirmed":              {"haco: configuration was not acknowledged; read current configuration with haco config before retrying:", "haco: 設定の完了を確認できません。再実行する前にhaco configで現在の設定を確認してください。詳細:"},
	"config.invalid_receipt":          {"haco: controller returned an invalid configuration receipt. Inspect haco config before retrying.", "haco: 設定の応答を確認できません。再実行する前にhaco configで現在の設定を確認してください。"},
	"config.edit_retained":            {"Edited policy retained: %s", "編集した設定を残しています: %s"},
	"config.files_retained":           {"haco: saved; editor files retained: %s", "haco: 設定は保存済みです。エディタのファイルを残しています: %s"},
	"config.inspect":                  {"Use haco config --edit to change these settings. Inspecting them does not grant permission.", "変更するにはhaco config --editを使います。設定を表示するだけでは、新たな許可は与えません。"},
	"config.saved":                    {"Settings saved for subsequent requests. Existing connections are not revoked. Inspect the saved settings with haco config.", "以後の要求に使う設定を保存しました。接続中の通信は取り消していません。haco configで保存した内容を確認できます。"},
	"config.host_required":            {"haco config is available in the trusted Linux/WSL Host", "haco configは信頼されたLinux/WSLのHostから実行してください。"},
}
