package yhttp

import (
	"context"
	"fmt"
	"sync"

	"github.com/coder/websocket"

	"yjs-go-bridge/pkg/storage"
	"yjs-go-bridge/pkg/yawareness"
	"yjs-go-bridge/pkg/ynodeproto"
	"yjs-go-bridge/pkg/yprotocol"
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

type remoteStreamPeer struct {
	stream       NodeMessageStream
	documentKey  storage.DocumentKey
	connectionID string
	epoch        uint64

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
	}
	return nil
}

func (p *remoteStreamPeer) close(string) error {
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
				messages = append(messages, &ynodeproto.DocumentSyncResponse{
					DocumentKey:  key,
					ConnectionID: connectionID,
					Epoch:        epoch,
					UpdateV1:     append([]byte(nil), message.Sync.Payload...),
				})
			case yprotocol.SyncMessageTypeUpdate:
				messages = append(messages, &ynodeproto.DocumentUpdate{
					DocumentKey:  key,
					ConnectionID: connectionID,
					Epoch:        epoch,
					UpdateV1:     append([]byte(nil), message.Sync.Payload...),
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
				messages = append(messages, &ynodeproto.DocumentUpdate{
					DocumentKey:  key,
					ConnectionID: connectionID,
					Epoch:        epoch,
					UpdateV1:     append([]byte(nil), message.Sync.Payload...),
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

func remoteMessageToProtocolPayload(message ynodeproto.Message) ([]byte, bool, error) {
	switch message := message.(type) {
	case *ynodeproto.HandshakeAck:
		return nil, false, nil
	case *ynodeproto.DocumentSyncResponse:
		return yprotocol.EncodeProtocolSyncStep2(message.UpdateV1), false, nil
	case *ynodeproto.DocumentUpdate:
		return yprotocol.EncodeProtocolSyncUpdate(message.UpdateV1), false, nil
	case *ynodeproto.AwarenessUpdate:
		payload, err := yprotocol.EncodeProtocolMessage(yprotocol.ProtocolTypeAwareness, message.Payload)
		return payload, false, err
	case *ynodeproto.QueryAwarenessResponse:
		payload, err := yprotocol.EncodeProtocolMessage(yprotocol.ProtocolTypeAwareness, message.Payload)
		return payload, false, err
	case *ynodeproto.Close:
		return nil, true, nil
	case *ynodeproto.Ping, *ynodeproto.Pong:
		return nil, false, nil
	default:
		return nil, false, fmt.Errorf("yhttp: message type remoto nao suportado: %T", message)
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
