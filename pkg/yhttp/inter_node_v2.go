package yhttp

import (
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yjsbridge"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/ynodeproto"
)

func requestWantsInterNodeV2(req Request) bool {
	return req.SyncOutputFormat == yjsbridge.UpdateFormatV2
}

func handshakeRequestsInterNodeV2(handshake *ynodeproto.Handshake) bool {
	return handshake != nil && handshake.Flags&ynodeproto.FlagSupportsUpdateV2 != 0
}

func handshakeAckConfirmsInterNodeV2(ack *ynodeproto.HandshakeAck) bool {
	return ack != nil && ack.Flags&ynodeproto.FlagSupportsUpdateV2 != 0
}

func ownerToEdgeFormatFromAck(req Request, ack *ynodeproto.HandshakeAck) yjsbridge.UpdateFormat {
	if requestWantsInterNodeV2(req) && handshakeAckConfirmsInterNodeV2(ack) {
		return yjsbridge.UpdateFormatV2
	}
	return yjsbridge.UpdateFormatV1
}
