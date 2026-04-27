package yhttp

import (
	"fmt"

	"yjs-go-bridge/pkg/ynodeproto"
	"yjs-go-bridge/pkg/yprotocol"
)

const (
	remoteOwnerMetricsRoleEdge  = "edge"
	remoteOwnerMetricsRoleOwner = "owner"

	remoteOwnerMetricsDirectionIn  = "in"
	remoteOwnerMetricsDirectionOut = "out"
)

func nodeMessageMetricKind(message ynodeproto.Message) string {
	switch message.(type) {
	case *ynodeproto.Handshake:
		return "handshake"
	case *ynodeproto.HandshakeAck:
		return "handshake_ack"
	case *ynodeproto.Ping:
		return "ping"
	case *ynodeproto.Pong:
		return "pong"
	case *ynodeproto.DocumentSyncRequest:
		return "document_sync_request"
	case *ynodeproto.DocumentSyncResponse:
		return "document_sync_response"
	case *ynodeproto.DocumentUpdate:
		return "document_update"
	case *ynodeproto.AwarenessUpdate:
		return "awareness_update"
	case *ynodeproto.QueryAwarenessRequest:
		return "query_awareness_request"
	case *ynodeproto.QueryAwarenessResponse:
		return "query_awareness_response"
	case *ynodeproto.Disconnect:
		return "disconnect"
	case *ynodeproto.Close:
		return "close"
	default:
		return "unknown"
	}
}

func protocolPayloadMetricKindsForOwner(payload []byte) ([]string, error) {
	protocolMessages, err := yprotocol.DecodeProtocolMessages(payload)
	if err != nil {
		return nil, err
	}

	kinds := make([]string, 0, len(protocolMessages))
	for idx, message := range protocolMessages {
		switch {
		case message.Sync != nil:
			switch message.Sync.Type {
			case yprotocol.SyncMessageTypeStep1:
				kinds = append(kinds, "document_sync_request")
			case yprotocol.SyncMessageTypeStep2, yprotocol.SyncMessageTypeUpdate:
				kinds = append(kinds, "document_update")
			default:
				return nil, fmt.Errorf("yhttp: sync remoto inbound nao suportado no indice %d: %v", idx, message.Sync.Type)
			}
		case message.Awareness != nil:
			kinds = append(kinds, "awareness_update")
		case message.QueryAwareness != nil:
			kinds = append(kinds, "query_awareness_request")
		default:
			return nil, fmt.Errorf("yhttp: protocol outbound owner metrics nao suportado no indice %d", idx)
		}
	}
	return kinds, nil
}
