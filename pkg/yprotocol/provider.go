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
	// ErrAuthorityLost sinaliza que o owner local perdeu a autoridade sobre o documento.
	ErrAuthorityLost = errors.New("yprotocol: autoridade perdida para o documento")
	// ErrAuthorityFenceUnsupported sinaliza wiring inconsistente entre resolver e store.
	ErrAuthorityFenceUnsupported = errors.New("yprotocol: store nao suporta fencing autoritativo")
)

// ResolveAuthorityFenceFunc resolve o fence autoritativo atual do owner local
// para um documento antes das operações de escrita/persistência.
type ResolveAuthorityFenceFunc func(ctx context.Context, key storage.DocumentKey) (*storage.AuthorityFence, error)

// ProviderConfig define dependências opcionais do provider local.
type ProviderConfig struct {
	// Store permite hidratação e persistência explícita de snapshots do documento.
	//
	// Quando o store também implementa `storage.UpdateLogStore`, o provider
	// recupera `snapshot + tail` em `Open`, registra updates incrementais no log
	// e compacta esse estado em `Persist`.
	Store storage.SnapshotStore

	// ResolveAuthorityFence ativa fencing autoritativo opcional para runtimes
	// distribuídos, exigindo que o store suporte os contratos autoritativos.
	ResolveAuthorityFence ResolveAuthorityFenceFunc
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
	mu                    sync.Mutex
	store                 storage.SnapshotStore
	resolveAuthorityFence ResolveAuthorityFenceFunc
	rooms                 map[storage.DocumentKey]*providerRoom
}

type providerRoom struct {
	mu            sync.Mutex
	key           storage.DocumentKey
	snapshot      *yjsbridge.PersistedSnapshot
	lastOffset    storage.UpdateOffset
	compactedAt   storage.UpdateOffset
	authority     *storage.AuthorityFence
	authorityLost bool
	connections   map[string]*Connection
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
		store:                 cfg.Store,
		resolveAuthorityFence: cfg.ResolveAuthorityFence,
		rooms:                 make(map[storage.DocumentKey]*providerRoom),
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

	if room.authorityLost {
		return nil, ErrAuthorityLost
	}
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

// AuthorityLost informa se o room desta conexão já perdeu a autoridade local.
func (c *Connection) AuthorityLost() bool {
	if c == nil || c.room == nil {
		return false
	}

	c.room.mu.Lock()
	defer c.room.mu.Unlock()
	return c.room.authorityLost
}

// AuthorityEpoch retorna o epoch autoritativo atualmente anexado ao room.
//
// Quando o provider nao opera com fencing autoritativo, retorna zero.
func (c *Connection) AuthorityEpoch() uint64 {
	if c == nil || c.room == nil {
		return 0
	}

	c.room.mu.Lock()
	defer c.room.mu.Unlock()
	if c.room.authority == nil {
		return 0
	}
	return c.room.authority.Owner.Epoch
}

// RevalidateAuthority força uma nova checagem do fence autoritativo do room.
//
// Quando não há fencing configurado, a operação é no-op.
func (c *Connection) RevalidateAuthority(ctx context.Context) error {
	if c == nil {
		return ErrConnectionClosed
	}
	if ctx == nil {
		ctx = context.Background()
	}

	room := c.room
	if room == nil {
		return ErrConnectionClosed
	}

	room.mu.Lock()
	if c.closed {
		room.mu.Unlock()
		return ErrConnectionClosed
	}
	if room.authorityLost {
		room.mu.Unlock()
		return ErrAuthorityLost
	}
	provider := c.provider
	key := room.key
	current := room.authority.Clone()
	room.mu.Unlock()

	if provider == nil || provider.resolveAuthorityFence == nil || current == nil {
		return nil
	}

	resolved, err := provider.resolveRoomAuthority(ctx, key)
	if err != nil {
		if errors.Is(err, ErrAuthorityLost) || errors.Is(err, storage.ErrAuthorityLost) {
			room.mu.Lock()
			room.authorityLost = true
			room.mu.Unlock()
			return wrapAuthorityLost(err)
		}
		return err
	}
	if !authorityFenceEqual(current, resolved) {
		room.mu.Lock()
		room.authorityLost = true
		room.mu.Unlock()
		return wrapAuthorityLost(storage.ErrAuthorityLost)
	}

	room.mu.Lock()
	if !room.authorityLost {
		room.authority = resolved.Clone()
	}
	room.mu.Unlock()
	return nil
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
	if c.room.authorityLost {
		return nil, ErrAuthorityLost
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
	if c.room.authorityLost {
		c.room.mu.Unlock()
		return nil, ErrAuthorityLost
	}
	if c.provider == nil || c.provider.store == nil {
		c.room.mu.Unlock()
		return nil, ErrPersistenceDisabled
	}
	key := c.room.key
	snapshot := c.room.snapshot.Clone()
	lastOffset := c.room.lastOffset
	shouldTrim := lastOffset > c.room.compactedAt
	authority := c.room.authority.Clone()
	c.room.mu.Unlock()

	record, err := c.provider.saveSnapshot(ctx, key, snapshot, lastOffset, authority)
	if err != nil {
		if errors.Is(err, storage.ErrAuthorityLost) {
			c.room.mu.Lock()
			c.room.authorityLost = true
			c.room.mu.Unlock()
			return nil, wrapAuthorityLost(err)
		}
		return nil, err
	}

	if updateStore := c.provider.updateLogStore(); updateStore != nil && shouldTrim {
		if err := c.provider.trimUpdates(ctx, key, lastOffset, authority); err != nil {
			if errors.Is(err, storage.ErrAuthorityLost) {
				c.room.mu.Lock()
				c.room.authorityLost = true
				c.room.mu.Unlock()
				return record, wrapAuthorityLost(err)
			}
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

	authority, err := p.resolveRoomAuthority(ctx, key)
	if err != nil {
		p.mu.Unlock()
		return nil, err
	}

	snapshot, lastOffset, compactedAt, err := p.loadSnapshot(ctx, key)
	if err != nil {
		p.mu.Unlock()
		return nil, err
	}

	room = &providerRoom{
		key:         key,
		snapshot:    snapshot,
		lastOffset:  lastOffset,
		compactedAt: compactedAt,
		authority:   authority,
		connections: make(map[string]*Connection),
	}
	p.rooms[key] = room
	p.mu.Unlock()
	return room, nil
}

func (p *Provider) loadSnapshot(ctx context.Context, key storage.DocumentKey) (*yjsbridge.PersistedSnapshot, storage.UpdateOffset, storage.UpdateOffset, error) {
	if p == nil || p.store == nil {
		return yjsbridge.NewPersistedSnapshot(), 0, 0, nil
	}

	if updateStore := p.updateLogStore(); updateStore != nil {
		recovered, err := storage.RecoverSnapshot(ctx, p.store, updateStore, key, 0, 0)
		if err != nil {
			return nil, 0, 0, err
		}
		if recovered == nil || recovered.Snapshot == nil {
			return yjsbridge.NewPersistedSnapshot(), 0, 0, nil
		}
		return recovered.Snapshot.Clone(), recovered.LastOffset, recovered.CheckpointThrough, nil
	}

	record, err := p.store.LoadSnapshot(ctx, key)
	if err != nil {
		if errors.Is(err, storage.ErrSnapshotNotFound) {
			return yjsbridge.NewPersistedSnapshot(), 0, 0, nil
		}
		return nil, 0, 0, err
	}
	if record == nil || record.Snapshot == nil {
		if record == nil {
			return yjsbridge.NewPersistedSnapshot(), 0, 0, nil
		}
		return yjsbridge.NewPersistedSnapshot(), record.Through, record.Through, nil
	}
	return record.Snapshot.Clone(), record.Through, record.Through, nil
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

func (p *Provider) authoritativeUpdateLogStore() storage.AuthoritativeUpdateLogStore {
	if p == nil || p.store == nil {
		return nil
	}
	updateStore, ok := p.store.(storage.AuthoritativeUpdateLogStore)
	if !ok {
		return nil
	}
	return updateStore
}

func (p *Provider) authoritativeSnapshotStore() storage.AuthoritativeSnapshotStore {
	if p == nil || p.store == nil {
		return nil
	}
	snapshotStore, ok := p.store.(storage.AuthoritativeSnapshotStore)
	if !ok {
		return nil
	}
	return snapshotStore
}

func (p *Provider) snapshotCheckpointStore() storage.SnapshotCheckpointStore {
	if p == nil || p.store == nil {
		return nil
	}
	snapshotStore, ok := p.store.(storage.SnapshotCheckpointStore)
	if !ok {
		return nil
	}
	return snapshotStore
}

func (p *Provider) authoritativeSnapshotCheckpointStore() storage.AuthoritativeSnapshotCheckpointStore {
	if p == nil || p.store == nil {
		return nil
	}
	snapshotStore, ok := p.store.(storage.AuthoritativeSnapshotCheckpointStore)
	if !ok {
		return nil
	}
	return snapshotStore
}

func (p *Provider) resolveRoomAuthority(ctx context.Context, key storage.DocumentKey) (*storage.AuthorityFence, error) {
	if p == nil || p.resolveAuthorityFence == nil {
		return nil, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if p.authoritativeSnapshotStore() == nil || p.authoritativeUpdateLogStore() == nil {
		return nil, ErrAuthorityFenceUnsupported
	}

	fence, err := p.resolveAuthorityFence(ctx, key)
	if err != nil {
		if errors.Is(err, storage.ErrAuthorityLost) {
			return nil, wrapAuthorityLost(err)
		}
		return nil, err
	}
	if fence == nil {
		return nil, wrapAuthorityLost(storage.ErrAuthorityLost)
	}
	if err := fence.Validate(); err != nil {
		return nil, err
	}
	return fence.Clone(), nil
}

func (p *Provider) saveSnapshot(ctx context.Context, key storage.DocumentKey, snapshot *yjsbridge.PersistedSnapshot, through storage.UpdateOffset, authority *storage.AuthorityFence) (*storage.SnapshotRecord, error) {
	if authority == nil {
		if checkpointStore := p.snapshotCheckpointStore(); checkpointStore != nil {
			return checkpointStore.SaveSnapshotCheckpoint(ctx, key, snapshot, through)
		}
		return p.store.SaveSnapshot(ctx, key, snapshot)
	}
	if checkpointStore := p.authoritativeSnapshotCheckpointStore(); checkpointStore != nil {
		return checkpointStore.SaveSnapshotCheckpointAuthoritative(ctx, key, snapshot, through, *authority)
	}
	return p.authoritativeSnapshotStore().SaveSnapshotAuthoritative(ctx, key, snapshot, *authority)
}

func (p *Provider) trimUpdates(ctx context.Context, key storage.DocumentKey, through storage.UpdateOffset, authority *storage.AuthorityFence) error {
	updateStore := p.updateLogStore()
	if updateStore == nil {
		return nil
	}
	if authority == nil {
		return updateStore.TrimUpdates(ctx, key, through)
	}
	return p.authoritativeUpdateLogStore().TrimUpdatesAuthoritative(ctx, key, through, *authority)
}

func wrapAuthorityLost(err error) error {
	if err == nil {
		return ErrAuthorityLost
	}
	return fmt.Errorf("%w: %v", ErrAuthorityLost, err)
}

func authorityFenceEqual(left *storage.AuthorityFence, right *storage.AuthorityFence) bool {
	switch {
	case left == nil && right == nil:
		return true
	case left == nil || right == nil:
		return false
	default:
		return left.ShardID == right.ShardID &&
			left.Owner == right.Owner &&
			left.Token == right.Token
	}
}

func (r *providerRoom) handleMessageLocked(sender *Connection, message *ProtocolMessage) ([]*ProtocolMessage, []byte, error) {
	if r.authorityLost {
		return nil, nil, ErrAuthorityLost
	}
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
	if r.authorityLost {
		return ErrAuthorityLost
	}

	updateV1, err := yjsbridge.ConvertUpdateToV1(payload)
	if err != nil {
		return err
	}

	var appendedOffset storage.UpdateOffset
	if provider != nil {
		if updateStore := provider.updateLogStore(); updateStore != nil {
			var (
				record *storage.UpdateLogRecord
				err    error
			)
			if r.authority != nil {
				record, err = provider.authoritativeUpdateLogStore().AppendUpdateAuthoritative(context.Background(), key, updateV1, *r.authority)
			} else {
				record, err = updateStore.AppendUpdate(context.Background(), key, updateV1)
			}
			if err != nil {
				if errors.Is(err, storage.ErrAuthorityLost) {
					r.authorityLost = true
					return wrapAuthorityLost(err)
				}
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

		recovered, lastOffset, compactedAt, recoverErr := provider.loadSnapshot(context.Background(), key)
		if recoverErr != nil {
			return fmt.Errorf("rebuild room snapshot: %w (recover: %v)", err, recoverErr)
		}
		r.snapshot = recovered
		if lastOffset < appendedOffset {
			lastOffset = appendedOffset
		}
		r.lastOffset = lastOffset
		r.compactedAt = compactedAt
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
