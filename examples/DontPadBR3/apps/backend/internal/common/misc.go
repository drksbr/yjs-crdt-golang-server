package common

import (
	"net/http"
	"strings"
)

func SanitizeASCII(value string) string {
	if value == "" {
		return "download"
	}
	var out strings.Builder
	for _, r := range value {
		if r < 0x20 || r > 0x7E || r == '\\' || r == '"' || r == '/' {
			out.WriteRune('_')
			continue
		}
		out.WriteRune(r)
	}
	result := out.String()
	if result == "" {
		return "download"
	}
	return result
}

func NormalizeDisplayAddress(address string) string {
	if strings.HasPrefix(address, ":") {
		return "127.0.0.1" + address
	}
	return address
}

func QuoteIdentifier(value string) string {
	return `"` + value + `"`
}

func ClientIP(r *http.Request) string {
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		return strings.TrimSpace(parts[0])
	}
	if value := strings.TrimSpace(r.Header.Get("X-Real-IP")); value != "" {
		return value
	}
	return r.RemoteAddr
}

func MathRound(value float64, decimals int) float64 {
	if decimals <= 0 {
		return float64(int64(value + 0.5))
	}
	factor := mathPow10(decimals)
	return float64(int64(value*factor+0.5)) / factor
}

func mathPow10(exp int) float64 {
	result := 1.0
	for i := 0; i < exp; i++ {
		result *= 10
	}
	return result
}
