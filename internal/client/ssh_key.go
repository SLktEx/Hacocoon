package client

import "github.com/SLktEx/Hacocoon/internal/sshkey"

func normalizePublicKey(raw string) (string, error) { return sshkey.NormalizePublicKey(raw) }
