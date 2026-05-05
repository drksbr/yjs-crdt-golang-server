package common

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"
)

func RandomUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf(
		"%08x-%04x-%04x-%04x-%012x",
		b[0:4],
		b[4:6],
		b[6:8],
		b[8:10],
		b[10:16],
	), nil
}

func GenerateVersionID() string {
	suffix := randomBase36(7)
	return fmt.Sprintf("%d-%s", time.Now().UnixMilli(), suffix)
}

func randomBase36(n int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	if n <= 0 {
		return ""
	}
	builder := strings.Builder{}
	builder.Grow(n)
	max := big.NewInt(int64(len(alphabet)))
	for i := 0; i < n; i++ {
		value, err := rand.Int(rand.Reader, max)
		if err != nil {
			builder.WriteByte(alphabet[(time.Now().UnixNano()+int64(i))%int64(len(alphabet))])
			continue
		}
		builder.WriteByte(alphabet[value.Int64()])
	}
	return builder.String()
}
