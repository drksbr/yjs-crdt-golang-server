package yawareness

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestStateManagerIgnoresSameClockStateButAcceptsSameClockTombstone(t *testing.T) {
	t.Parallel()

	var manager StateManager
	start := time.Unix(1700000000, 0)

	manager.ApplyAt(&Update{
		Clients: []ClientState{
			{ClientID: 7, Clock: 4, State: json.RawMessage(`{"name":"alice"}`)},
		},
	}, start)

	manager.ApplyAt(&Update{
		Clients: []ClientState{
			{ClientID: 7, Clock: 4, State: json.RawMessage(`{"name":"bob"}`)},
		},
	}, start.Add(5*time.Second))

	client, ok := manager.Get(7)
	if !ok || !bytes.Equal(client.State, []byte(`{"name":"alice"}`)) {
		t.Fatalf("client = %+v, want original state alice preserved", client)
	}

	meta, ok := manager.Meta(7)
	if !ok || meta.LastUpdated != start || meta.Clock != 4 {
		t.Fatalf("meta = %+v, want clock=4 lastUpdated=start", meta)
	}

	tombstoneAt := start.Add(10 * time.Second)
	manager.ApplyAt(&Update{
		Clients: []ClientState{
			{ClientID: 7, Clock: 4, State: nil},
		},
	}, tombstoneAt)

	if _, ok := manager.Get(7); ok {
		t.Fatal("expected client 7 removed by same-clock tombstone")
	}

	meta, ok = manager.Meta(7)
	if !ok || meta.Clock != 4 || meta.LastUpdated != tombstoneAt {
		t.Fatalf("meta = %+v, want preserved clock=4 with tombstone timestamp", meta)
	}

	update := manager.UpdateForClients([]uint32{7})
	if len(update.Clients) != 1 || !update.Clients[0].IsNull() || update.Clients[0].Clock != 4 {
		t.Fatalf("update = %+v, want single tombstone clock=4", update.Clients)
	}
}

func TestStateManagerProtectsLocalStateAgainstRemoteTombstone(t *testing.T) {
	t.Parallel()

	manager := NewStateManager(5)
	start := time.Unix(1700000010, 0)
	if err := manager.SetLocalStateAt(json.RawMessage(`{"cursor":1}`), start); err != nil {
		t.Fatalf("SetLocalStateAt() unexpected error: %v", err)
	}

	manager.ApplyAt(&Update{
		Clients: []ClientState{
			{ClientID: 5, Clock: 0, State: nil},
		},
	}, start.Add(time.Second))

	client, ok := manager.Get(5)
	if !ok || client.Clock != 1 || !bytes.Equal(client.State, []byte(`{"cursor":1}`)) {
		t.Fatalf("client = %+v, want local state preserved with rebased clock=1", client)
	}

	meta, ok := manager.Meta(5)
	if !ok || meta.Clock != 1 {
		t.Fatalf("meta = %+v, want clock rebased to 1", meta)
	}

	update := manager.UpdateForClients([]uint32{5})
	if len(update.Clients) != 1 || update.Clients[0].Clock != 1 || update.Clients[0].IsNull() {
		t.Fatalf("update = %+v, want live local state clock=1", update.Clients)
	}
}

func TestStateManagerRenewsLocalStateAfterHalfTimeout(t *testing.T) {
	t.Parallel()

	manager := NewStateManager(9)
	start := time.Unix(1700000020, 0)
	if err := manager.SetLocalStateAt(json.RawMessage(`{"name":"alice"}`), start); err != nil {
		t.Fatalf("SetLocalStateAt() unexpected error: %v", err)
	}

	renewed, err := manager.RenewLocalIfDueAt(start.Add(OutdatedTimeout/2-time.Millisecond), OutdatedTimeout)
	if err != nil {
		t.Fatalf("RenewLocalIfDueAt() early unexpected error: %v", err)
	}
	if renewed {
		t.Fatal("expected no renewal before half timeout")
	}

	renewed, err = manager.RenewLocalIfDueAt(start.Add(OutdatedTimeout/2), OutdatedTimeout)
	if err != nil {
		t.Fatalf("RenewLocalIfDueAt() unexpected error: %v", err)
	}
	if !renewed {
		t.Fatal("expected renewal at half timeout")
	}

	client, ok := manager.Get(9)
	if !ok || client.Clock != 1 || !bytes.Equal(client.State, []byte(`{"name":"alice"}`)) {
		t.Fatalf("client = %+v, want renewed local state with clock=1", client)
	}
}

func TestStateManagerExpireStaleRemotesKeepsMetaForTombstones(t *testing.T) {
	t.Parallel()

	manager := NewStateManager(99)
	start := time.Unix(1700000030, 0)

	if err := manager.SetLocalStateAt(json.RawMessage(`{"self":true}`), start); err != nil {
		t.Fatalf("SetLocalStateAt() unexpected error: %v", err)
	}
	manager.ApplyAt(&Update{
		Clients: []ClientState{
			{ClientID: 1, Clock: 2, State: json.RawMessage(`{"name":"alice"}`)},
			{ClientID: 2, Clock: 7, State: json.RawMessage(`{"name":"bob"}`)},
		},
	}, start)

	removed := manager.ExpireStaleAt(start.Add(OutdatedTimeout-time.Millisecond), OutdatedTimeout)
	if len(removed) != 0 {
		t.Fatalf("removed = %v, want none before timeout", removed)
	}

	removed = manager.ExpireStaleAt(start.Add(OutdatedTimeout), OutdatedTimeout)
	if len(removed) != 2 || removed[0] != 1 || removed[1] != 2 {
		t.Fatalf("removed = %v, want [1 2]", removed)
	}

	if _, ok := manager.Get(1); ok {
		t.Fatal("expected client 1 removed from active states")
	}
	if _, ok := manager.Get(99); !ok {
		t.Fatal("expected local client preserved during remote expiration")
	}

	update := manager.UpdateForClients([]uint32{1, 2})
	if len(update.Clients) != 2 || !update.Clients[0].IsNull() || !update.Clients[1].IsNull() {
		t.Fatalf("update = %+v, want tombstones for expired remotes", update.Clients)
	}
	if update.Clients[0].Clock != 2 || update.Clients[1].Clock != 7 {
		t.Fatalf("update clocks = %+v, want preserved clocks 2 and 7", update.Clients)
	}
}

func TestStateManagerSetLocalStateRequiresConfiguredClientID(t *testing.T) {
	t.Parallel()

	var manager StateManager
	err := manager.SetLocalStateAt(json.RawMessage(`{"cursor":1}`), time.Unix(1700000040, 0))
	if !errors.Is(err, ErrLocalClientIDNotConfigured) {
		t.Fatalf("SetLocalStateAt() error = %v, want ErrLocalClientIDNotConfigured", err)
	}
}

func TestStateManagerApplyAcceptsFirstRemoteClockZero(t *testing.T) {
	t.Parallel()

	manager := NewStateManager(9)
	at := time.Unix(1700000050, 0)

	manager.ApplyAt(&Update{
		Clients: []ClientState{
			{ClientID: 3, Clock: 0, State: json.RawMessage(`{"name":"remote-zero"}`)},
		},
	}, at)

	state, ok := manager.Get(3)
	if !ok {
		t.Fatal("Get(3) = missing, want first remote clock zero to be accepted")
	}
	if !bytes.Equal(state.State, []byte(`{"name":"remote-zero"}`)) {
		t.Fatalf("state.State = %s, want remote-zero payload", state.State)
	}

	meta, ok := manager.Meta(3)
	if !ok || meta.Clock != 0 || meta.LastUpdated != at {
		t.Fatalf("Meta(3) = (%+v, %v), want clock=0 lastUpdated=at", meta, ok)
	}
}
