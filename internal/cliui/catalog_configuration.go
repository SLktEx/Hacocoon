package cliui

// Configuration messages describe receipts without translating Policy values.
var configurationCatalog = map[string]translation{
	"config.unconfirmed":     {"haco: configuration was not acknowledged; read current configuration with haco config before retrying:", "haco: 設定の完了を確認できません。再実行する前にhaco configで現在の設定を確認してください。詳細:"},
	"config.invalid_receipt": {"haco: controller returned an invalid configuration receipt. Inspect haco config before retrying.", "haco: 設定の応答を確認できません。再実行する前にhaco configで現在の設定を確認してください。"},
	"config.edit_retained":   {"Edited policy retained: %s", "編集した設定を残しています: %s"},
	"config.files_retained":  {"haco: saved; editor files retained: %s", "haco: 設定は保存済みです。エディタのファイルを残しています: %s"},
	"config.inspect":         {"Use haco config --edit to change these settings. Inspecting them does not grant permission.", "変更するにはhaco config --editを使います。設定を表示するだけでは、新たな許可は与えません。"},
	"config.saved":           {"Settings saved for subsequent requests. Existing connections are not revoked. Inspect the saved settings with haco config.", "以後の要求に使う設定を保存しました。接続中の通信は取り消していません。haco configで保存した内容を確認できます。"},
	"config.host_required":   {"haco config is available in the trusted Linux/WSL Host", "haco configは信頼されたLinux/WSLのHostから実行してください。"},
}
