package cliui

// Network guidance never changes Policy values or connection authority.
var networkCatalog = map[string]translation{
	"network.failed":              {"haco: network operation failed. Check haco doctor and haco network --help. Details:", "haco: 通信の操作に失敗しました。haco doctorとhaco network --helpで状態と使い方を確認してください。詳細:"},
	"network.connections_empty":   {"No active connections. A local listener requests permission when an application connects.", "接続中の通信はありません。ローカルの接続口をアプリが使うときに、通信の許可を判定します。"},
	"network.revoked":             {"Connection revoked. This does not remove saved network rules.", "接続の許可を取り消しました。保存済みの通信ルールは削除していません。"},
	"network.service_exists":      {"haco: service already exists; remove it before registering a replacement", "haco: 同じ名前のHostサービスが登録されています。haco network host listで確認し、置き換える場合はhaco network host remove <name>で登録を解除してください。"},
	"network.host_added":          {"Host service registered. Registration alone does not grant access; configure permission with haco network rule.", "Hostサービスを登録しました。登録だけでは通信を許可しません。許可の設定にはhaco network ruleを使います。"},
	"network.hosts_empty":         {"No Host services registered. Use haco network host add --help to register a destination.", "Hostサービスは未登録です。haco network host add --helpで宛先の登録方法を確認できます。"},
	"network.host_removed":        {"Host service registration removed. Inspect remaining settings with haco network host list.", "Hostサービスの登録を解除しました。haco network host listで残りの登録を確認できます。"},
	"network.rule_added":          {"Network rule saved. Connections still follow the complete Policy and this rule's scope and expiry.", "通信ルールを保存しました。接続時は、このルールの対象・期限と、ほかの設定も合わせて許可を判定します。"},
	"network.invalid_destination": {"haco: invalid destination or lifetime", "haco: 宛先または有効時間が無効です。haco network tcp --helpまたはhaco network udp --helpで指定方法を確認してください。"},
	"network.loopback":            {"haco: listener must use a numeric loopback address", "haco: 接続口には127.0.0.1などの数値のloopbackアドレスを指定してください。ほかのPCから受け付けるアドレスは使えません。"},
	"network.listener_ready":      {"Local listener ready. Each application connection requires permission; press Ctrl+C to stop listening.", "ローカルの接続口を用意しました。アプリの接続ごとに通信の許可を判定します。終了するにはCtrl+Cを押してください。"},
	"network.listener_failed":     {"haco: local forwarding failed. Check the listener address and haco network --help. Details:", "haco: ローカル転送に失敗しました。接続口のアドレスとhaco network --helpを確認してください。詳細:"},
}
