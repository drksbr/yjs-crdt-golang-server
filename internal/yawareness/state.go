package yawareness

import (
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
	m.ApplyAt(update, m.nowTime())
}

// ApplyAt é a variante determinística de Apply para testes e integração controlada.
func (m *StateManager) ApplyAt(update *Update, now time.Time) {
	if update == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureMapsLocked()

	for _, client := range update.Clients {
		m.applyClientLocked(client, now)
	}
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
	state, err := normalizeState(state)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.hasLocalClientID {
		return ErrLocalClientIDNotConfigured
	}
	m.ensureMapsLocked()

	meta := m.meta[m.localClientID]
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
	return nil
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
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.states) == 0 {
		return nil
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
	return removed
}

func (m *StateManager) applyClientLocked(client ClientState, now time.Time) {
	state, err := normalizeState(client.State)
	if err != nil {
		return
	}
	client.State = state

	currentMeta, hasMeta := m.meta[client.ClientID]
	currentClock := uint32(0)
	if hasMeta {
		currentClock = currentMeta.Clock
	}

	currentState, hasState := m.states[client.ClientID]
	if hasMeta && !(currentClock < client.Clock || (currentClock == client.Clock && client.IsNull() && hasState)) {
		return
	}

	if client.IsNull() {
		if m.hasLocalClientID && client.ClientID == m.localClientID && hasState {
			currentState.Clock = client.Clock + 1
			m.states[client.ClientID] = cloneClientState(currentState)
			client.Clock++
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
