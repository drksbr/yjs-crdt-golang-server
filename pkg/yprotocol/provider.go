package yprotocol

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"yjs-go-bridge/pkg/storage"
	"yjs-go-bridge/pkg/yawareness"
	"yjs-go-bridge/pkg/yjsbridge"
)

var (
	// ErrInvalidConnectionID sinaliza tentativa de abrir conexão sem identificador.
	ErrInvalidConnectionID = errors.New("yprotocol: connection id invalido")
	// ErrConnectionClosed sinaliza uso de conexão já encerrada.
	ErrConnectionClosed = errors.New("yprotocol: connection fechada")
	// ErrConnectionExists sinaliza duplicidade de connectionID dentro do mesmo documento.
	ErrConnectionExists = errors.New("yprotocol: connection ja existe para o documento")
	// ErrClientIDExists sinaliza duplicidade de localClientID dentro do mesmo documento.
	ErrClientIDExists = errors.New("yprotocol: client id ja existe para o documento")
	// ErrPersistenceDisabled sinaliza ausência de SnapshotStore no provider.
	ErrPersistenceDisabled = errors.New("yprotocol: persistencia desabilitada")
)

// ProviderConfig define dependências opcionais do provider local.
type ProviderConfig struct {
	// Store permite hidratação e persistência explícita de snapshots do documento.
	//
	// Quando o store também implementa `storage.UpdateLogStore`, o provider
	// recupera `snapshot + tail` em `Open`, registra updates incrementais no log
	// e compacta esse estado em `Persist`.
	Store storage.SnapshotStore
}

// DispatchResult representa a saída local de uma operação no provider.
//
// `Direct` é enviado apenas para a conexão chamadora.
// `Broadcast` pode ser reenviado para os demais peers do mesmo documento.
type DispatchResult struct {
	Direct    []byte
	Broadcast []byte
}

// Provider compõe múltiplas `Session` em torno do mesmo documento para um
// runtime single-process mínimo.
//
// O provider:
// - carrega o snapshot inicial do documento em `Open`;
// - mantém o update V1 autoritativo do room;
// - replica updates e awareness entre conexões do mesmo documento;
// - deixa transporte, fanout de rede e persistência automática fora de escopo.
type Provider struct {
	mu    sync.Mutex
	store storage.SnapshotStore
	rooms map[storage.DocumentKey]*providerRoom
}

type providerRoom struct {
	mu          sync.Mutex
	key         storage.DocumentKey
	snapshot    *yjsbridge.PersistedSnapshot
	lastOffset  storage.UpdateOffset
	compactedAt storage.UpdateOffset
	connections map[string]*Connection
}

// Connection representa uma conexão local anexada a um documento do provider.
type Connection struct {
	provider *Provider
	room     *providerRoom
	id       string
	clientID uint32
	session  *Session
	closed   bool
}

// NewProvider cria um provider local com store opcional.
func NewProvider(cfg ProviderConfig) *Provider {
	return &Provider{
		store: cfg.Store,
		rooms: make(map[storage.DocumentKey]*providerRoom),
	}
}

// Open cria ou reutiliza o room do documento e anexa uma conexão local.
func (p *Provider) Open(ctx context.Context, key storage.DocumentKey, connectionID string, localClientID uint32) (*Connection, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(connectionID) == "" {
		return nil, ErrInvalidConnectionID
	}
	if err := key.Validate(); err != nil {
		return nil, err
	}

	room, err := p.ensureRoom(ctx, key)
	if err != nil {
		return nil, err
	}

	room.mu.Lock()
	defer room.mu.Unlock()

	if _, exists := room.connections[connectionID]; exists {
		return nil, ErrConnectionExists
	}
	for _, existing := range room.connections {
		if existing.closed {
			continue
		}
		if existing.clientID == localClientID {
			return nil, ErrClientIDExists
		}
	}

	connection := &Connection{
		provider: p,
		room:     room,
		id:       connectionID,
		clientID: localClientID,
		session:  NewSession(localClientID),
	}
	if err := connection.session.LoadPersistedSnapshot(room.snapshot); err != nil {
		return nil, err
	}

	awareness := room.aggregateLocalAwarenessLocked(connectionID)
	if len(awareness.Clients) > 0 {
		if _, err := connection.session.HandleProtocolMessage(&ProtocolMessage{
			Protocol:  ProtocolTypeAwareness,
			Awareness: awareness,
		}); err != nil {
			return nil, err
		}
	}

	room.connections[connectionID] = connection
	return connection, nil
}

// ID retorna o identificador estável da conexão no room.
func (c *Connection) ID() string {
	if c == nil {
		return ""
	}
	return c.id
}

// ClientID retorna o clientID awareness da conexão.
func (c *Connection) ClientID() uint32 {
	if c == nil {
		return 0
	}
	return c.clientID
}

// DocumentKey retorna a chave do documento associada à conexão.
func (c *Connection) DocumentKey() storage.DocumentKey {
	if c == nil || c.room == nil {
		return storage.DocumentKey{}
	}
	return c.room.key
}

// HandleEncodedMessages aplica um stream protocolado à conexão e retorna:
// - resposta direta para a conexão chamadora;
// - stream de broadcast uniforme para os demais peers do room.
func (c *Connection) HandleEncodedMessages(src []byte) (*DispatchResult, error) {
	if c == nil {
		return nil, ErrConnectionClosed
	}

	messages, err := DecodeProtocolMessages(src)
	if err != nil {
		return nil, err
	}

	c.room.mu.Lock()
	defer c.room.mu.Unlock()

	if c.closed {
		return nil, ErrConnectionClosed
	}

	result := &DispatchResult{}
	direct := make([]*ProtocolMessage, 0)

	for idx, message := range messages {
		directMessages, broadcast, err := c.room.handleMessageLocked(c, message)
		if err != nil {
			return nil, fmt.Errorf("provider handle message %d: %w", idx, err)
		}
		direct = append(direct, directMessages...)
		result.Broadcast = append(result.Broadcast, broadcast...)
	}

	encodedDirect, err := EncodeProtocolEnvelopes(direct...)
	if err != nil {
		return nil, err
	}
	result.Direct = encodedDirect
	return result, nil
}

// Persist grava o snapshot autoritativo atual do documento no store configurado.
func (c *Connection) Persist(ctx context.Context) (*storage.SnapshotRecord, error) {
	if c == nil {
		return nil, ErrConnectionClosed
	}
	if ctx == nil {
		ctx = context.Background()
	}

	c.room.mu.Lock()
	if c.closed {
		c.room.mu.Unlock()
		return nil, ErrConnectionClosed
	}
	if c.provider == nil || c.provider.store == nil {
		c.room.mu.Unlock()
		return nil, ErrPersistenceDisabled
	}
	key := c.room.key
	snapshot := c.room.snapshot.Clone()
	lastOffset := c.room.lastOffset
	shouldTrim := lastOffset > c.room.compactedAt
	c.room.mu.Unlock()

	record, err := c.provider.store.SaveSnapshot(ctx, key, snapshot)
	if err != nil {
		return nil, err
	}

	if updateStore := c.provider.updateLogStore(); updateStore != nil && shouldTrim {
		if err := updateStore.TrimUpdates(ctx, key, lastOffset); err != nil {
			return record, fmt.Errorf("trim compacted updates through %d: %w", lastOffset, err)
		}

		c.room.mu.Lock()
		if c.room.compactedAt < lastOffset {
			c.room.compactedAt = lastOffset
		}
		c.room.mu.Unlock()
	}
	return record, nil
}

// Close remove a conexão do room e, se existir presença local, gera um
// broadcast awareness tombstone para os peers restantes.
func (c *Connection) Close() (*DispatchResult, error) {
	if c == nil {
		return nil, ErrConnectionClosed
	}

	room := c.room
	room.mu.Lock()
	if c.closed {
		room.mu.Unlock()
		return nil, ErrConnectionClosed
	}

	result := &DispatchResult{}
	if tombstone := c.localAwarenessTombstone(); len(tombstone.Clients) > 0 {
		message := &ProtocolMessage{
			Protocol:  ProtocolTypeAwareness,
			Awareness: tombstone,
		}
		for _, peer := range room.sortedConnectionsLocked() {
			if peer.id == c.id || peer.closed {
				continue
			}
			if _, err := peer.session.HandleProtocolMessage(message); err != nil {
				room.mu.Unlock()
				return nil, err
			}
		}

		encoded, err := EncodeProtocolEnvelope(message)
		if err != nil {
			room.mu.Unlock()
			return nil, err
		}
		result.Broadcast = encoded
	}

	c.closed = true
	delete(room.connections, c.id)
	empty := len(room.connections) == 0
	room.mu.Unlock()

	if empty && c.provider != nil {
		c.provider.mu.Lock()
		if current := c.provider.rooms[room.key]; current == room {
			delete(c.provider.rooms, room.key)
		}
		c.provider.mu.Unlock()
	}

	return result, nil
}

func (p *Provider) ensureRoom(ctx context.Context, key storage.DocumentKey) (*providerRoom, error) {
	p.mu.Lock()
	room, ok := p.rooms[key]
	if ok {
		p.mu.Unlock()
		return room, nil
	}

	snapshot, lastOffset, err := p.loadSnapshot(ctx, key)
	if err != nil {
		p.mu.Unlock()
		return nil, err
	}

	room = &providerRoom{
		key:         key,
		snapshot:    snapshot,
		lastOffset:  lastOffset,
		connections: make(map[string]*Connection),
	}
	p.rooms[key] = room
	p.mu.Unlock()
	return room, nil
}

func (p *Provider) loadSnapshot(ctx context.Context, key storage.DocumentKey) (*yjsbridge.PersistedSnapshot, storage.UpdateOffset, error) {
	if p == nil || p.store == nil {
		return yjsbridge.NewPersistedSnapshot(), 0, nil
	}

	if updateStore := p.updateLogStore(); updateStore != nil {
		recovered, err := storage.RecoverSnapshot(ctx, p.store, updateStore, key, 0, 0)
		if err != nil {
			return nil, 0, err
		}
		if recovered == nil || recovered.Snapshot == nil {
			return yjsbridge.NewPersistedSnapshot(), 0, nil
		}
		return recovered.Snapshot.Clone(), recovered.LastOffset, nil
	}

	record, err := p.store.LoadSnapshot(ctx, key)
	if err != nil {
		if errors.Is(err, storage.ErrSnapshotNotFound) {
			return yjsbridge.NewPersistedSnapshot(), 0, nil
		}
		return nil, 0, err
	}
	if record == nil || record.Snapshot == nil {
		return yjsbridge.NewPersistedSnapshot(), 0, nil
	}
	return record.Snapshot.Clone(), 0, nil
}

func (p *Provider) updateLogStore() storage.UpdateLogStore {
	if p == nil || p.store == nil {
		return nil
	}
	updateStore, ok := p.store.(storage.UpdateLogStore)
	if !ok {
		return nil
	}
	return updateStore
}

func (r *providerRoom) handleMessageLocked(sender *Connection, message *ProtocolMessage) ([]*ProtocolMessage, []byte, error) {
	if err := validateProtocolMessage(message); err != nil {
		return nil, nil, err
	}

	switch message.Protocol {
	case ProtocolTypeQueryAwareness:
		return []*ProtocolMessage{{
			Protocol:  ProtocolTypeAwareness,
			Awareness: r.aggregateLocalAwarenessLocked(""),
		}}, nil, nil
	case ProtocolTypeSync:
		if message.Sync.Type == SyncMessageTypeStep2 || message.Sync.Type == SyncMessageTypeUpdate {
			if err := r.applyDocumentPayloadLocked(sender.provider, r.key, message.Sync.Payload); err != nil {
				return nil, nil, err
			}
			encoded, err := EncodeProtocolEnvelope(message)
			if err != nil {
				return nil, nil, err
			}
			return nil, encoded, nil
		}
		if message.Sync.Type != SyncMessageTypeStep1 {
			return nil, nil, fmt.Errorf("%w: %d", ErrUnknownSyncMessageType, message.Sync.Type)
		}

		diff, err := yjsbridge.DiffUpdate(r.snapshot.UpdateV1, message.Sync.Payload)
		if err != nil {
			return nil, nil, err
		}
		return []*ProtocolMessage{{
			Protocol: ProtocolTypeSync,
			Sync: &SyncMessage{
				Type:    SyncMessageTypeStep2,
				Payload: diff,
			},
		}}, nil, nil
	case ProtocolTypeAwareness:
		if _, err := sender.session.HandleProtocolMessage(message); err != nil {
			return nil, nil, err
		}
		for _, peer := range r.sortedConnectionsLocked() {
			if peer.id == sender.id || peer.closed {
				continue
			}
			if _, err := peer.session.HandleProtocolMessage(message); err != nil {
				return nil, nil, err
			}
		}
		encoded, err := EncodeProtocolEnvelope(message)
		if err != nil {
			return nil, nil, err
		}
		return nil, encoded, nil
	case ProtocolTypeAuth:
		_, err := sender.session.HandleProtocolMessage(message)
		return nil, nil, err
	default:
		return nil, nil, fmt.Errorf("%w: %d", ErrUnknownProtocolType, message.Protocol)
	}
}

func (r *providerRoom) applyDocumentPayloadLocked(provider *Provider, key storage.DocumentKey, payload []byte) error {
	updateV1, err := yjsbridge.ConvertUpdateToV1(payload)
	if err != nil {
		return err
	}

	var appendedOffset storage.UpdateOffset
	if provider != nil {
		if updateStore := provider.updateLogStore(); updateStore != nil {
			record, err := updateStore.AppendUpdate(context.Background(), key, updateV1)
			if err != nil {
				return err
			}
			if record != nil {
				appendedOffset = record.Offset
			}
		}
	}

	nextSnapshot, err := storage.ReplaySnapshot(context.Background(), r.snapshot, &storage.UpdateLogRecord{
		Key:      key,
		UpdateV1: updateV1,
	})
	if err != nil {
		if appendedOffset == 0 || provider == nil {
			return err
		}

		recovered, lastOffset, recoverErr := provider.loadSnapshot(context.Background(), key)
		if recoverErr != nil {
			return fmt.Errorf("rebuild room snapshot: %w (recover: %v)", err, recoverErr)
		}
		r.snapshot = recovered
		if lastOffset < appendedOffset {
			lastOffset = appendedOffset
		}
		r.lastOffset = lastOffset
		return r.syncSessionsToSnapshotLocked()
	}

	r.snapshot = nextSnapshot
	if appendedOffset > 0 {
		r.lastOffset = appendedOffset
	}
	return r.syncSessionsToSnapshotLocked()
}

func (r *providerRoom) syncSessionsToSnapshotLocked() error {
	for _, peer := range r.sortedConnectionsLocked() {
		if peer.closed {
			continue
		}
		if err := peer.session.LoadPersistedSnapshot(r.snapshot); err != nil {
			return err
		}
	}
	return nil
}

func (r *providerRoom) aggregateLocalAwarenessLocked(excludeConnectionID string) *yawareness.Update {
	clients := make([]yawareness.ClientState, 0)
	for _, connection := range r.sortedConnectionsLocked() {
		if connection.id == excludeConnectionID || connection.closed {
			continue
		}
		update := connection.session.Awareness().UpdateForClients([]uint32{connection.clientID})
		if update == nil || len(update.Clients) == 0 {
			continue
		}
		for _, client := range update.Clients {
			clients = append(clients, yawareness.ClientState{
				ClientID: client.ClientID,
				Clock:    client.Clock,
				State:    append([]byte(nil), client.State...),
			})
		}
	}
	return &yawareness.Update{Clients: clients}
}

func (r *providerRoom) sortedConnectionsLocked() []*Connection {
	keys := make([]string, 0, len(r.connections))
	for key := range r.connections {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := make([]*Connection, 0, len(keys))
	for _, key := range keys {
		out = append(out, r.connections[key])
	}
	return out
}

func (c *Connection) localAwarenessTombstone() *yawareness.Update {
	if c == nil || c.session == nil || c.session.Awareness() == nil {
		return &yawareness.Update{}
	}

	if _, ok := c.session.Awareness().Meta(c.clientID); !ok {
		return &yawareness.Update{}
	}
	if err := c.session.Awareness().SetLocalState(nil); err != nil {
		return &yawareness.Update{}
	}
	update := c.session.Awareness().UpdateForClients([]uint32{c.clientID})
	if update == nil {
		return &yawareness.Update{}
	}
	return update
}
