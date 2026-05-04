package yhttp

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/yjsbridge"
)

const (
	// SyncOutputFormatQueryParam é o query param de referência para negociar
	// egress sync V2 no WebSocket HTTP.
	SyncOutputFormatQueryParam = "sync"
	// SyncOutputFormatHeader é o header de referência para negociar egress sync V2.
	SyncOutputFormatHeader = "X-Yjs-Sync-Output-Format"
	// SyncOutputFormatSubprotocolV1 representa opt-in explícito V1 em subprotocol.
	SyncOutputFormatSubprotocolV1 = "yjs-sync-v1"
	// SyncOutputFormatSubprotocolV2 representa opt-in explícito V2 em subprotocol.
	SyncOutputFormatSubprotocolV2 = "yjs-sync-v2"
)

// SyncOutputFormatFromHTTPRequest resolve o formato de egress sync solicitado
// por uma request HTTP/WebSocket.
//
// A ausência de negociação retorna UpdateFormatV1. Query tem precedência sobre
// header, e header tem precedência sobre subprotocols.
func SyncOutputFormatFromHTTPRequest(r *http.Request) (yjsbridge.UpdateFormat, error) {
	if r == nil {
		return yjsbridge.UpdateFormatV1, nil
	}
	if r.URL != nil {
		if value := strings.TrimSpace(r.URL.Query().Get(SyncOutputFormatQueryParam)); value != "" {
			return parseSyncOutputFormat(value)
		}
	}
	if value := strings.TrimSpace(r.Header.Get(SyncOutputFormatHeader)); value != "" {
		return parseSyncOutputFormat(value)
	}
	return syncOutputFormatFromSubprotocols(r.Header.Values("Sec-WebSocket-Protocol"))
}

func parseSyncOutputFormat(value string) (yjsbridge.UpdateFormat, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "v1", "1", "update-v1", SyncOutputFormatSubprotocolV1:
		return yjsbridge.UpdateFormatV1, nil
	case "v2", "2", "update-v2", SyncOutputFormatSubprotocolV2:
		return yjsbridge.UpdateFormatV2, nil
	default:
		return yjsbridge.UpdateFormatUnknown, fmt.Errorf("%w: %s", yjsbridge.ErrUnknownUpdateFormat, value)
	}
}

func syncOutputFormatFromSubprotocols(values []string) (yjsbridge.UpdateFormat, error) {
	for _, value := range values {
		for _, token := range strings.Split(value, ",") {
			format, err := parseSyncOutputFormat(token)
			if err != nil {
				continue
			}
			if format == yjsbridge.UpdateFormatV2 || strings.EqualFold(strings.TrimSpace(token), SyncOutputFormatSubprotocolV1) {
				return format, nil
			}
		}
	}
	return yjsbridge.UpdateFormatV1, nil
}
