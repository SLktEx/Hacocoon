#!/bin/sh
set -eu
printf '#!/bin/sh\ncat /usr/local/share/packer-tool-message\n' > /usr/local/bin/my-tool
printf '%s\n' "$TOOL_MESSAGE" > /usr/local/share/packer-tool-message
chmod 0755 /usr/local/bin/my-tool
