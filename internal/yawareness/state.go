package yawareness

import (
	"encoding/json"
	"sort"
	"sync"
)

// StateManager mantém o estado mais recente de awareness por cliente.
// A política de merge prioriza maior clock e usa ordem de chegada quando há empate.
type StateManager struct {
	mu     sync.RWMutex
	states map[uint32]ClientState
}

// Apply mescla uma atualização awareness por cliente, removendo tombstones (null).
func (m *StateManager) Apply(update *Update) {
	if update == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.states == nil {
		m.states = make(map[uint32]ClientState)
	}

	for _, client := range update.Clients {
		m.applyClientLocked(client)
	}
}

// ApplyJSON aplica uma atualização serialized em JSON.
func (m *StateManager) ApplyJSON(src []byte) {
	var update Update
	if err := json.Unmarshal(src, &update); err != nil {
		return
	}
	m.Apply(&update)
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

func (m *StateManager) applyClientLocked(client ClientState) {
	state, err := normalizeState(client.State)
	if err != nil {
		return
	}
	client.State = state

	current, hasCurrent := m.states[client.ClientID]
	if hasCurrent && client.Clock < current.Clock {
		return
	}

	if client.IsNull() {
		delete(m.states, client.ClientID)
		return
	}

	m.states[client.ClientID] = cloneClientState(client)
}

func cloneClientState(state ClientState) ClientState {
	state.State = append([]byte(nil), state.State...)
	return state
}
