package client

import "github.com/SLktEx/Hacocoon/internal/client/ssh/key"

func normalizePublicKey(raw string) (string, error) { return sshkey.NormalizePublicKey(raw) }
