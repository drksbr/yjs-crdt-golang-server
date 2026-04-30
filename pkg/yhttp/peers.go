package yhttp

import (
	"context"
	"fmt"
	"sync"

	"github.com/coder/websocket"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yawareness"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yjsbridge"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/ynodeproto"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yprotocol"
)

type websocketPeer struct {
	conn    *websocket.Conn
	writeMu sync.Mutex
}

func (p *websocketPeer) deliver(ctx context.Context, payload []byte) error {
	p.writeMu.Lock()
	defer p.writeMu.Unlock()

	return p.conn.Write(ctx, websocket.MessageBinary, payload)
}

func (p *websocketPeer) close(reason string) error {
	return p.conn.Close(websocket.StatusGoingAway, reason)
}

type quotaPeer struct {
	base  roomPeer
	quota QuotaLease
}

func (p quotaPeer) deliver(ctx context.Context, payload []byte) error {
	if p.quota != nil {
		if err := p.quota.AllowFrame(ctx, QuotaDirectionOutbound, len(payload)); err != nil {
			return err
		}
	}
	return p.base.deliver(ctx, payload)
}

func (p quotaPeer) close(reason string) error {
	return p.base.close(reason)
}

type remoteStreamPeer struct {
	stream       NodeMessageStream
	documentKey  storage.DocumentKey
	connectionID string
	epoch        uint64
	onDeliver    func(ynodeproto.Message)

	writeMu sync.Mutex
}

func (p *remoteStreamPeer) deliver(ctx context.Context, payload []byte) error {
	messages, err := protocolPayloadToRemoteMessages(p.documentKey, p.connectionID, p.epoch, payload)
	if err != nil {
		return err
	}

	p.writeMu.Lock()
	defer p.writeMu.Unlock()

	for _, message := range messages {
		if err := p.stream.Send(ctx, message); err != nil {
			return err
		}
		if p.onDeliver != nil {
			p.onDeliver(message)
		}
	}
	return nil
}

func (p *remoteStreamPeer) close(string) error {
	return p.stream.Close()
}

type ownerStreamPeer struct {
	stream       NodeMessageStream
	documentKey  storage.DocumentKey
	connectionID string
	epoch        uint64

	writeMu sync.Mutex
}

func (p *ownerStreamPeer) deliver(ctx context.Context, payload []byte) error {
	messages, err := protocolPayloadToOwnerMessages(p.documentKey, p.connectionID, p.epoch, payload)
	if err != nil {
		return err
	}

	p.writeMu.Lock()
	defer p.writeMu.Unlock()

	for _, message := range messages {
		if err := p.stream.Send(ctx, message); err != nil {
			return err
		}
	}
	return nil
}

func (p *ownerStreamPeer) close(string) error {
	return p.stream.Close()
}

func protocolPayloadToRemoteMessages(
	key storage.DocumentKey,
	connectionID string,
	epoch uint64,
	payload []byte,
) ([]ynodeproto.Message, error) {
	protocolMessages, err := yprotocol.DecodeProtocolMessages(payload)
	if err != nil {
		return nil, err
	}

	messages := make([]ynodeproto.Message, 0, len(protocolMessages))
	for idx, message := range protocolMessages {
		switch {
		case message.Sync != nil:
			switch message.Sync.Type {
			case yprotocol.SyncMessageTypeStep2:
				updateV1, err := yjsbridge.ConvertUpdateToV1(message.Sync.Payload)
				if err != nil {
					return nil, err
				}
				messages = append(messages, &ynodeproto.DocumentSyncResponse{
					DocumentKey:  key,
					ConnectionID: connectionID,
					Epoch:        epoch,
					UpdateV1:     updateV1,
				})
			case yprotocol.SyncMessageTypeUpdate:
				updateV1, err := yjsbridge.ConvertUpdateToV1(message.Sync.Payload)
				if err != nil {
					return nil, err
				}
				messages = append(messages, &ynodeproto.DocumentUpdate{
					DocumentKey:  key,
					ConnectionID: connectionID,
					Epoch:        epoch,
					UpdateV1:     updateV1,
				})
			default:
				return nil, fmt.Errorf("yhttp: sync remoto outbound nao suportado no indice %d: %v", idx, message.Sync.Type)
			}
		case message.Awareness != nil:
			encoded, err := yawareness.EncodeUpdate(message.Awareness)
			if err != nil {
				return nil, err
			}
			messages = append(messages, &ynodeproto.AwarenessUpdate{
				DocumentKey:  key,
				ConnectionID: connectionID,
				Epoch:        epoch,
				Payload:      encoded,
			})
		default:
			return nil, fmt.Errorf("yhttp: protocol outbound remoto nao suportado no indice %d", idx)
		}
	}
	return messages, nil
}

func protocolPayloadToQueryAwarenessMessages(
	key storage.DocumentKey,
	connectionID string,
	epoch uint64,
	payload []byte,
) ([]ynodeproto.Message, error) {
	protocolMessages, err := yprotocol.DecodeProtocolMessages(payload)
	if err != nil {
		return nil, err
	}

	messages := make([]ynodeproto.Message, 0, len(protocolMessages))
	for idx, message := range protocolMessages {
		if message.Awareness == nil {
			return nil, fmt.Errorf("yhttp: query-awareness direto nao suportado no indice %d", idx)
		}

		encoded, err := yawareness.EncodeUpdate(message.Awareness)
		if err != nil {
			return nil, err
		}
		messages = append(messages, &ynodeproto.QueryAwarenessResponse{
			DocumentKey:  key,
			ConnectionID: connectionID,
			Epoch:        epoch,
			Payload:      encoded,
		})
	}
	return messages, nil
}

func protocolPayloadToOwnerMessages(
	key storage.DocumentKey,
	connectionID string,
	epoch uint64,
	payload []byte,
) ([]ynodeproto.Message, error) {
	protocolMessages, err := yprotocol.DecodeProtocolMessages(payload)
	if err != nil {
		return nil, err
	}

	messages := make([]ynodeproto.Message, 0, len(protocolMessages))
	for idx, message := range protocolMessages {
		switch {
		case message.Sync != nil:
			switch message.Sync.Type {
			case yprotocol.SyncMessageTypeStep1:
				messages = append(messages, &ynodeproto.DocumentSyncRequest{
					DocumentKey:  key,
					ConnectionID: connectionID,
					Epoch:        epoch,
					StateVector:  append([]byte(nil), message.Sync.Payload...),
				})
			case yprotocol.SyncMessageTypeStep2, yprotocol.SyncMessageTypeUpdate:
				updateV1, err := yjsbridge.ConvertUpdateToV1(message.Sync.Payload)
				if err != nil {
					return nil, err
				}
				messages = append(messages, &ynodeproto.DocumentUpdate{
					DocumentKey:  key,
					ConnectionID: connectionID,
					Epoch:        epoch,
					UpdateV1:     updateV1,
				})
			default:
				return nil, fmt.Errorf("yhttp: sync remoto inbound nao suportado no indice %d: %v", idx, message.Sync.Type)
			}
		case message.Awareness != nil:
			encoded, err := yawareness.EncodeUpdate(message.Awareness)
			if err != nil {
				return nil, err
			}
			messages = append(messages, &ynodeproto.AwarenessUpdate{
				DocumentKey:  key,
				ConnectionID: connectionID,
				Epoch:        epoch,
				Payload:      encoded,
			})
		case message.QueryAwareness != nil:
			messages = append(messages, &ynodeproto.QueryAwarenessRequest{
				DocumentKey:  key,
				ConnectionID: connectionID,
				Epoch:        epoch,
			})
		default:
			return nil, fmt.Errorf("yhttp: protocol inbound remoto nao suportado no indice %d", idx)
		}
	}
	return messages, nil
}

func remoteMessageToProtocolPayload(message ynodeproto.Message) ([]byte, *ynodeproto.Close, error) {
	switch message := message.(type) {
	case *ynodeproto.HandshakeAck:
		return nil, nil, nil
	case *ynodeproto.DocumentSyncResponse:
		return yprotocol.EncodeProtocolSyncStep2(message.UpdateV1), nil, nil
	case *ynodeproto.DocumentUpdate:
		return yprotocol.EncodeProtocolSyncUpdate(message.UpdateV1), nil, nil
	case *ynodeproto.AwarenessUpdate:
		payload, err := yprotocol.EncodeProtocolMessage(yprotocol.ProtocolTypeAwareness, message.Payload)
		return payload, nil, err
	case *ynodeproto.QueryAwarenessResponse:
		payload, err := yprotocol.EncodeProtocolMessage(yprotocol.ProtocolTypeAwareness, message.Payload)
		return payload, nil, err
	case *ynodeproto.Close:
		return nil, message, nil
	case *ynodeproto.Ping, *ynodeproto.Pong:
		return nil, nil, nil
	default:
		return nil, nil, fmt.Errorf("yhttp: message type remoto nao suportado: %T", message)
	}
}

func ownerMessageToProtocolPayload(message ynodeproto.Message) ([]byte, error) {
	switch message := message.(type) {
	case *ynodeproto.DocumentSyncRequest:
		return yprotocol.EncodeProtocolSyncStep1(message.StateVector), nil
	case *ynodeproto.DocumentUpdate:
		return yprotocol.EncodeProtocolSyncUpdate(message.UpdateV1), nil
	case *ynodeproto.AwarenessUpdate:
		return yprotocol.EncodeProtocolMessage(yprotocol.ProtocolTypeAwareness, message.Payload)
	case *ynodeproto.QueryAwarenessRequest:
		return yprotocol.EncodeProtocolQueryAwareness(), nil
	default:
		return nil, fmt.Errorf("yhttp: message type owner nao suportado: %T", message)
	}
}
