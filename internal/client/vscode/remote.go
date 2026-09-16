package vscode

import (
	_ "embed"
	"encoding/base64"
	"fmt"
	"regexp"
)

//go:embed apply.js
var applyScript string

var commitPattern = regexp.MustCompile(`^[a-f0-9]{40}$`)

// RemoteCommand waits for the exact stable desktop build bootstrapped by
// Remote-SSH. The helper runs as the ordinary SSH guest user, with no Host data.
func RemoteCommand(commit string) (string, error) {
	if !commitPattern.MatchString(commit) {
		return "", fmt.Errorf("invalid VS Code commit")
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(applyScript))
	return `set -eu
export PATH=/usr/local/bin:/usr/bin:/bin
cd "$HOME"
for i in $(seq 1 120); do
  for server in "$HOME/.vscode-server/cli/servers/Stable-` + commit + `/server" "$HOME/.vscode-server/bin/` + commit + `"; do
    if test -x "$server/node" && test -f "$server/out/server-main.js"; then
      exec "$server/node" -e 'eval(Buffer.from("` + encoded + `", "base64").toString("utf8"))'
    fi
  done
  sleep 1
done
exit 1`, nil
}
