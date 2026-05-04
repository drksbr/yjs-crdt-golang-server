package yawareness

import (
	"bytes"
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"time"
)

var (
	// ErrLocalClientIDNotConfigured sinaliza que operações locais exigem um clientID local explícito.
	ErrLocalClientIDNotConfigured = errors.New("yawareness: clientID local nao configurado")
)

// OutdatedTimeout replica o timeout de inatividade usado pelo awareness do Yjs.
const OutdatedTimeout = 30 * time.Second

// ClientMeta mantém o relógio observado e o instante da última atualização.
// O meta é preservado mesmo após tombstones para permitir reemissão consistente.
type ClientMeta struct {
	Clock       uint32
	LastUpdated time.Time
}

// Change descreve o delta produzido por uma operação de awareness.
type Change struct {
	Added   []uint32
	Updated []uint32
	Removed []uint32
}

// Empty informa se a operação não alterou nenhum client conhecido.
func (c Change) Empty() bool {
	return len(c.Added) == 0 && len(c.Updated) == 0 && len(c.Removed) == 0
}

// StateManager mantém o estado mais recente de awareness por cliente.
// O manager preserva metadados de clock/tempo para seguir a semântica do Yjs.
type StateManager struct {
	mu               sync.RWMutex
	localClientID    uint32
	hasLocalClientID bool
	states           map[uint32]ClientState
	meta             map[uint32]ClientMeta
	now              func() time.Time
}

// NewStateManager cria um manager já associado a um clientID local.
func NewStateManager(localClientID uint32) *StateManager {
	return &StateManager{
		localClientID:    localClientID,
		hasLocalClientID: true,
	}
}

// SetLocalClientID configura o clientID local usado para proteção de presença e heartbeat.
func (m *StateManager) SetLocalClientID(clientID uint32) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.localClientID = clientID
	m.hasLocalClientID = true
}

// Apply mescla uma atualização awareness por cliente, removendo tombstones (null).
func (m *StateManager) Apply(update *Update) {
	_ = m.ApplyWithChangeAt(update, m.nowTime())
}

// ApplyAt é a variante determinística de Apply para testes e integração controlada.
func (m *StateManager) ApplyAt(update *Update, now time.Time) {
	_ = m.ApplyWithChangeAt(update, now)
}

// ApplyWithChange mescla uma atualização awareness e retorna o delta aceito.
func (m *StateManager) ApplyWithChange(update *Update) Change {
	return m.ApplyWithChangeAt(update, m.nowTime())
}

// ApplyWithChangeAt é a variante determinística de ApplyWithChange.
func (m *StateManager) ApplyWithChangeAt(update *Update, now time.Time) Change {
	if update == nil {
		return Change{}
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureMapsLocked()

	change := changeBuilder{}
	for _, client := range update.Clients {
		change.add(m.applyClientLocked(client, now))
	}
	return change.finish()
}

// ApplyJSON aplica uma atualização serialized em JSON.
func (m *StateManager) ApplyJSON(src []byte) {
	var update Update
	if err := json.Unmarshal(src, &update); err != nil {
		return
	}
	m.ApplyAt(&update, m.nowTime())
}

// Snapshot retorna uma cópia ordenada do estado corrente (ordenado por clientID).
func (m *StateManager) Snapshot() *Update {
	m.mu.RLock()
	defer m.mu.RUnlock()

	clientIDs := make([]uint32, 0, len(m.states))
	for clientID := range m.states {
		clientIDs = append(clientIDs, clientID)
	}
	sort.Slice(clientIDs, func(i, j int) bool {
		return clientIDs[i] < clientIDs[j]
	})

	clients := make([]ClientState, 0, len(clientIDs))
	for _, clientID := range clientIDs {
		clients = append(clients, cloneClientState(m.states[clientID]))
	}
	return &Update{Clients: clients}
}

// UpdateForClients materializa um awareness update a partir do estado/meta conhecido.
// Clientes tombstoned são emitidos com estado `null`.
func (m *StateManager) UpdateForClients(clientIDs []uint32) *Update {
	m.mu.RLock()
	defer m.mu.RUnlock()

	clients := make([]ClientState, 0, len(clientIDs))
	for _, clientID := range clientIDs {
		meta, ok := m.meta[clientID]
		if !ok {
			continue
		}

		client := ClientState{
			ClientID: clientID,
			Clock:    meta.Clock,
			State:    json.RawMessage("null"),
		}
		if state, hasState := m.states[clientID]; hasState {
			client.State = append([]byte(nil), state.State...)
		}
		clients = append(clients, client)
	}
	return &Update{Clients: clients}
}

// Get recupera o estado de um clientID.
func (m *StateManager) Get(clientID uint32) (ClientState, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	state, ok := m.states[clientID]
	if !ok {
		return ClientState{}, false
	}
	return cloneClientState(state), true
}

// Meta recupera o metadado conhecido para um clientID, incluindo clocks tombstoned.
func (m *StateManager) Meta(clientID uint32) (ClientMeta, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	meta, ok := m.meta[clientID]
	return meta, ok
}

// SetLocalState atualiza o estado local e incrementa o clock conforme o runtime do Yjs.
func (m *StateManager) SetLocalState(state json.RawMessage) error {
	return m.SetLocalStateAt(state, m.nowTime())
}

// SetLocalStateAt é a variante determinística de SetLocalState para testes.
func (m *StateManager) SetLocalStateAt(state json.RawMessage, now time.Time) error {
	_, err := m.SetLocalStateWithChangeAt(state, now)
	return err
}

// SetLocalStateWithChange atualiza o estado local e retorna o delta produzido.
func (m *StateManager) SetLocalStateWithChange(state json.RawMessage) (Change, error) {
	return m.SetLocalStateWithChangeAt(state, m.nowTime())
}

// SetLocalStateWithChangeAt é a variante determinística de
// SetLocalStateWithChange.
func (m *StateManager) SetLocalStateWithChangeAt(state json.RawMessage, now time.Time) (Change, error) {
	state, err := normalizeState(state)
	if err != nil {
		return Change{}, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.hasLocalClientID {
		return Change{}, ErrLocalClientIDNotConfigured
	}
	m.ensureMapsLocked()

	meta := m.meta[m.localClientID]
	_, hadState := m.states[m.localClientID]
	clock := uint32(0)
	if _, ok := m.meta[m.localClientID]; ok {
		clock = meta.Clock + 1
	}

	client := ClientState{
		ClientID: m.localClientID,
		Clock:    clock,
		State:    state,
	}
	if client.IsNull() {
		delete(m.states, client.ClientID)
	} else {
		m.states[client.ClientID] = cloneClientState(client)
	}
	m.meta[client.ClientID] = ClientMeta{
		Clock:       clock,
		LastUpdated: now,
	}
	return localChangeForState(client.ClientID, hadState, !client.IsNull()), nil
}

// SetLocalStateField atualiza um campo do estado local JSON object, criando o
// objeto quando ainda não há estado local.
func (m *StateManager) SetLocalStateField(key string, value json.RawMessage) error {
	_, err := m.SetLocalStateFieldWithChangeAt(key, value, m.nowTime())
	return err
}

// SetLocalStateFieldAt é a variante determinística de SetLocalStateField.
func (m *StateManager) SetLocalStateFieldAt(key string, value json.RawMessage, now time.Time) error {
	_, err := m.SetLocalStateFieldWithChangeAt(key, value, now)
	return err
}

// SetLocalStateFieldWithChange atualiza um campo local e retorna o delta
// produzido.
func (m *StateManager) SetLocalStateFieldWithChange(key string, value json.RawMessage) (Change, error) {
	return m.SetLocalStateFieldWithChangeAt(key, value, m.nowTime())
}

// SetLocalStateFieldWithChangeAt é a variante determinística de
// SetLocalStateFieldWithChange.
func (m *StateManager) SetLocalStateFieldWithChangeAt(key string, value json.RawMessage, now time.Time) (Change, error) {
	value, err := normalizeState(value)
	if err != nil {
		return Change{}, err
	}

	m.mu.RLock()
	state := json.RawMessage(nil)
	if m.states != nil {
		if current, ok := m.states[m.localClientID]; ok {
			state = append([]byte(nil), current.State...)
		}
	}
	m.mu.RUnlock()

	fields := make(map[string]json.RawMessage)
	if len(state) > 0 && !bytes.Equal(state, []byte("null")) {
		if err := json.Unmarshal(state, &fields); err != nil {
			return Change{}, ErrInvalidJSON
		}
	}
	fields[key] = append([]byte(nil), value...)

	next, err := json.Marshal(fields)
	if err != nil {
		return Change{}, ErrInvalidJSON
	}
	return m.SetLocalStateWithChangeAt(next, now)
}

// RenewLocalIfDue reaplica o estado local quando metade do timeout já passou.
func (m *StateManager) RenewLocalIfDue(timeout time.Duration) (bool, error) {
	return m.RenewLocalIfDueAt(m.nowTime(), timeout)
}

// RenewLocalIfDueAt é a variante determinística de RenewLocalIfDue.
func (m *StateManager) RenewLocalIfDueAt(now time.Time, timeout time.Duration) (bool, error) {
	m.mu.RLock()
	if !m.hasLocalClientID {
		m.mu.RUnlock()
		return false, nil
	}

	state, hasState := m.states[m.localClientID]
	meta, hasMeta := m.meta[m.localClientID]
	m.mu.RUnlock()

	if !hasState || !hasMeta || now.Sub(meta.LastUpdated) < timeout/2 {
		return false, nil
	}
	return true, m.SetLocalStateAt(state.State, now)
}

// ExpireStale remove estados remotos inativos, preservando os metadados.
func (m *StateManager) ExpireStale(timeout time.Duration) []uint32 {
	return m.ExpireStaleAt(m.nowTime(), timeout)
}

// ExpireStaleAt é a variante determinística de ExpireStale.
func (m *StateManager) ExpireStaleAt(now time.Time, timeout time.Duration) []uint32 {
	change := m.ExpireStaleWithChangeAt(now, timeout)
	return change.Removed
}

// ExpireStaleWithChange remove estados remotos inativos e retorna o delta.
func (m *StateManager) ExpireStaleWithChange(timeout time.Duration) Change {
	return m.ExpireStaleWithChangeAt(m.nowTime(), timeout)
}

// ExpireStaleWithChangeAt é a variante determinística de
// ExpireStaleWithChange.
func (m *StateManager) ExpireStaleWithChangeAt(now time.Time, timeout time.Duration) Change {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.states) == 0 {
		return Change{}
	}

	removed := make([]uint32, 0)
	for clientID, meta := range m.meta {
		if m.hasLocalClientID && clientID == m.localClientID {
			continue
		}
		if _, ok := m.states[clientID]; ok && now.Sub(meta.LastUpdated) >= timeout {
			delete(m.states, clientID)
			removed = append(removed, clientID)
		}
	}

	sort.Slice(removed, func(i, j int) bool {
		return removed[i] < removed[j]
	})
	return Change{Removed: removed}
}

type changeEntry struct {
	clientID uint32
	kind     changeKind
}

type changeKind uint8

const (
	changeNone changeKind = iota
	changeAdded
	changeUpdated
	changeRemoved
)

type changeBuilder struct {
	added   map[uint32]struct{}
	updated map[uint32]struct{}
	removed map[uint32]struct{}
}

func (b *changeBuilder) add(entry changeEntry) {
	switch entry.kind {
	case changeAdded:
		b.ensureAdded()
		b.added[entry.clientID] = struct{}{}
		delete(b.updated, entry.clientID)
		delete(b.removed, entry.clientID)
	case changeUpdated:
		if _, added := b.added[entry.clientID]; added {
			return
		}
		b.ensureUpdated()
		b.updated[entry.clientID] = struct{}{}
		delete(b.removed, entry.clientID)
	case changeRemoved:
		delete(b.added, entry.clientID)
		delete(b.updated, entry.clientID)
		b.ensureRemoved()
		b.removed[entry.clientID] = struct{}{}
	}
}

func (b *changeBuilder) ensureAdded() {
	if b.added == nil {
		b.added = make(map[uint32]struct{})
	}
}

func (b *changeBuilder) ensureUpdated() {
	if b.updated == nil {
		b.updated = make(map[uint32]struct{})
	}
}

func (b *changeBuilder) ensureRemoved() {
	if b.removed == nil {
		b.removed = make(map[uint32]struct{})
	}
}

func (b changeBuilder) finish() Change {
	return Change{
		Added:   sortedChangeIDs(b.added),
		Updated: sortedChangeIDs(b.updated),
		Removed: sortedChangeIDs(b.removed),
	}
}

func (m *StateManager) applyClientLocked(client ClientState, now time.Time) changeEntry {
	state, err := normalizeState(client.State)
	if err != nil {
		return changeEntry{}
	}
	client.State = state

	currentMeta, hasMeta := m.meta[client.ClientID]
	currentClock := uint32(0)
	if hasMeta {
		currentClock = currentMeta.Clock
	}

	currentState, hasState := m.states[client.ClientID]
	if hasMeta && currentClock >= client.Clock && (currentClock != client.Clock || !client.IsNull() || !hasState) {
		return changeEntry{}
	}

	nextHasState := !client.IsNull()
	if client.IsNull() {
		if m.hasLocalClientID && client.ClientID == m.localClientID && hasState {
			currentState.Clock = client.Clock + 1
			m.states[client.ClientID] = cloneClientState(currentState)
			client.Clock++
			nextHasState = true
		} else {
			delete(m.states, client.ClientID)
		}
	} else {
		m.states[client.ClientID] = cloneClientState(client)
	}

	m.meta[client.ClientID] = ClientMeta{
		Clock:       client.Clock,
		LastUpdated: now,
	}
	return changeEntry{
		clientID: client.ClientID,
		kind:     changeKindForStateTransition(hasState, nextHasState),
	}
}

func (m *StateManager) ensureMapsLocked() {
	if m.states == nil {
		m.states = make(map[uint32]ClientState)
	}
	if m.meta == nil {
		m.meta = make(map[uint32]ClientMeta)
	}
}

func (m *StateManager) nowTime() time.Time {
	if m.now != nil {
		return m.now()
	}
	return time.Now()
}

func cloneClientState(state ClientState) ClientState {
	state.State = append([]byte(nil), state.State...)
	return state
}

func localChangeForState(clientID uint32, hadState, hasState bool) Change {
	switch changeKindForStateTransition(hadState, hasState) {
	case changeAdded:
		return Change{Added: []uint32{clientID}}
	case changeUpdated:
		return Change{Updated: []uint32{clientID}}
	case changeRemoved:
		return Change{Removed: []uint32{clientID}}
	default:
		return Change{}
	}
}

func changeKindForStateTransition(hadState, hasState bool) changeKind {
	switch {
	case !hadState && hasState:
		return changeAdded
	case hadState && hasState:
		return changeUpdated
	case hadState && !hasState:
		return changeRemoved
	default:
		return changeNone
	}
}

func sortedChangeIDs(set map[uint32]struct{}) []uint32 {
	if len(set) == 0 {
		return nil
	}
	ids := make([]uint32, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		return ids[i] < ids[j]
	})
	return ids
}
