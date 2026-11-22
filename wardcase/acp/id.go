package acp

import (
	"crypto/rand"
	"encoding/hex"
)

const codeInvalidParams = -32602

func randomSuffix() string {
	b := make([]byte, 6) //nolint:mnd // 12 hex characters
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
