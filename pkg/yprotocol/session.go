package yprotocol

import (
	"fmt"
	"sync"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/yawareness"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yjsbridge"
)

// Session mantém um estado mínimo em-processo para handshake e ingestão de
// envelopes y-protocols com saída de documento em V1 canônico.
//
// O runtime cobre apenas:
// - update V1 canônico do documento;
// - normalização de updates V2 válidos para V1 canônico;
// - awareness local/remoto via `StateManager`;
// - resposta a `SyncStep1` com `SyncStep2`;
// - resposta a `query-awareness` com snapshot de awareness.
//
// O tipo não implementa websocket server, auth policy distribuída nem provider
// completo.
type Session struct {
	mu        sync.RWMutex
	updateV1  []byte
	awareness *yawareness.StateManager
}

// NewSession cria um runtime in-process vazio para um client local.
func NewSession(localClientID uint32) *Session {
	return &Session{
		updateV1:  append([]byte(nil), yjsbridge.NewPersistedSnapshot().UpdateV1...),
		awareness: yawareness.NewStateManager(localClientID),
	}
}

// Awareness expõe o runtime thread-safe de awareness associado à sessão.
func (s *Session) Awareness() *yawareness.StateManager {
	if s == nil {
		return nil
	}
	return s.awareness
}

// UpdateV1 retorna uma cópia do update V1 canônico atual.
func (s *Session) UpdateV1() []byte {
	if s == nil {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]byte(nil), s.updateV1...)
}

// LoadUpdate substitui o estado atual pelo update suportado informado,
// normalizado para V1 canônico.
func (s *Session) LoadUpdate(update []byte) error {
	if s == nil {
		return ErrNilSession
	}

	updateV1, err := yjsbridge.ConvertUpdateToV1(update)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.updateV1 = append([]byte(nil), updateV1...)
	return nil
}

// LoadPersistedSnapshot substitui o documento atual usando um snapshot já
// persistido. Snapshot `nil` reinicia a sessão para o estado vazio.
func (s *Session) LoadPersistedSnapshot(snapshot *yjsbridge.PersistedSnapshot) error {
	if s == nil {
		return ErrNilSession
	}
	if snapshot == nil || snapshot.IsEmpty() {
		s.mu.Lock()
		s.updateV1 = append([]byte(nil), yjsbridge.NewPersistedSnapshot().UpdateV1...)
		s.mu.Unlock()
		return nil
	}
	return s.LoadUpdate(snapshot.UpdateV1)
}

// PersistedSnapshot materializa um snapshot persistível a partir do estado atual.
func (s *Session) PersistedSnapshot() (*yjsbridge.PersistedSnapshot, error) {
	if s == nil {
		return nil, ErrNilSession
	}
	return yjsbridge.PersistedSnapshotFromUpdate(s.UpdateV1())
}

// HandleProtocolMessage aplica uma mensagem inbound e retorna as respostas
// necessárias do runtime mínimo.
func (s *Session) HandleProtocolMessage(message *ProtocolMessage) ([]*ProtocolMessage, error) {
	if s == nil {
		return nil, ErrNilSession
	}
	if err := validateProtocolMessage(message); err != nil {
		return nil, err
	}

	switch message.Protocol {
	case ProtocolTypeSync:
		return s.handleSyncMessage(message.Sync)
	case ProtocolTypeAwareness:
		s.awareness.Apply(message.Awareness)
		return nil, nil
	case ProtocolTypeAuth:
		// Auth policy fica fora do escopo do runtime mínimo.
		return nil, nil
	case ProtocolTypeQueryAwareness:
		return []*ProtocolMessage{{
			Protocol:  ProtocolTypeAwareness,
			Awareness: s.awareness.Snapshot(),
		}}, nil
	default:
		return nil, fmt.Errorf("%w: %d", ErrUnknownProtocolType, message.Protocol)
	}
}

// HandleProtocolMessages processa uma sequência de mensagens inbound e agrega as
// respostas no mesmo ordenamento.
func (s *Session) HandleProtocolMessages(messages ...*ProtocolMessage) ([]*ProtocolMessage, error) {
	if s == nil {
		return nil, ErrNilSession
	}

	out := make([]*ProtocolMessage, 0)
	for idx, message := range messages {
		responses, err := s.HandleProtocolMessage(message)
		if err != nil {
			return nil, fmt.Errorf("handle protocol message %d: %w", idx, err)
		}
		out = append(out, responses...)
	}
	return out, nil
}

// HandleEncodedMessages decodifica um stream inbound, aplica as mensagens à
// sessão e retorna o stream outbound concatenado.
func (s *Session) HandleEncodedMessages(src []byte) ([]byte, error) {
	if s == nil {
		return nil, ErrNilSession
	}

	messages, err := DecodeProtocolMessages(src)
	if err != nil {
		return nil, err
	}
	responses, err := s.HandleProtocolMessages(messages...)
	if err != nil {
		return nil, err
	}
	return EncodeProtocolEnvelopes(responses...)
}

func (s *Session) handleSyncMessage(message *SyncMessage) ([]*ProtocolMessage, error) {
	switch message.Type {
	case SyncMessageTypeStep1:
		diff, err := yjsbridge.DiffUpdate(s.UpdateV1(), message.Payload)
		if err != nil {
			return nil, err
		}
		return []*ProtocolMessage{{
			Protocol: ProtocolTypeSync,
			Sync: &SyncMessage{
				Type:    SyncMessageTypeStep2,
				Payload: diff,
			},
		}}, nil
	case SyncMessageTypeStep2, SyncMessageTypeUpdate:
		current := s.UpdateV1()
		updateV1, err := yjsbridge.ConvertUpdateToV1(message.Payload)
		if err != nil {
			return nil, err
		}
		merged, err := yjsbridge.MergeUpdates(current, updateV1)
		if err != nil {
			return nil, err
		}

		s.mu.Lock()
		s.updateV1 = append([]byte(nil), merged...)
		s.mu.Unlock()
		return nil, nil
	default:
		return nil, fmt.Errorf("%w: %d", ErrUnknownSyncMessageType, message.Type)
	}
}
