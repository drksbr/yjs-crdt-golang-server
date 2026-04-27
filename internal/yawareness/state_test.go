package yawareness

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
)

func TestStateManagerApplyMergesByClockAndSortsSnapshot(t *testing.T) {
	t.Parallel()

	var manager StateManager
	manager.Apply(&Update{
		Clients: []ClientState{
			{ClientID: 2, Clock: 1, State: json.RawMessage(`{"kind":"late"}`)},
			{ClientID: 1, Clock: 3, State: json.RawMessage(`{"kind":"winner"}`)},
			{ClientID: 1, Clock: 5, State: json.RawMessage(`{"kind":"newest"}`)},
			{ClientID: 2, Clock: 4, State: json.RawMessage(`{"kind":"newer"}`)},
		},
	})

	got := manager.Snapshot()
	if len(got.Clients) != 2 {
		t.Fatalf("len(got.Clients)=%d, want 2", len(got.Clients))
	}

	if got.Clients[0].ClientID != 1 || got.Clients[0].Clock != 5 {
		t.Fatalf("client 1 = %+v, want clock=5", got.Clients[0])
	}
	if !bytes.Equal(got.Clients[0].State, []byte(`{"kind":"newest"}`)) {
		t.Fatalf("client 1 state=%s, want compact state", got.Clients[0].State)
	}

	if got.Clients[1].ClientID != 2 || got.Clients[1].Clock != 4 {
		t.Fatalf("client 2 = %+v, want clock=4", got.Clients[1])
	}
}

func TestStateManagerRejectsRegressiveClockAndIgnoresSameClockState(t *testing.T) {
	t.Parallel()

	var manager StateManager
	manager.Apply(&Update{
		Clients: []ClientState{
			{ClientID: 10, Clock: 7, State: json.RawMessage(`{"active":true}`)},
		},
	})
	manager.Apply(&Update{
		Clients: []ClientState{
			{ClientID: 10, Clock: 4, State: json.RawMessage(`{"active":false}`)},
		},
	})
	client, ok := manager.Get(10)
	if !ok {
		t.Fatalf("esperado cliente 10 presente após atualização válida")
	}
	if client.Clock != 7 {
		t.Fatalf("clock=%d, want 7", client.Clock)
	}

	manager.Apply(&Update{
		Clients: []ClientState{
			{ClientID: 10, Clock: 7, State: json.RawMessage(`{"active":"latest-same-clock"}`)},
		},
	})
	client, ok = manager.Get(10)
	if !ok || string(client.State) != `{"active":true}` {
		t.Fatalf("estado do cliente 10 = %+v, want same-clock state ignored", client)
	}

	manager.Apply(&Update{
		Clients: []ClientState{
			{ClientID: 10, Clock: 9, State: nil},
		},
	})
	if _, ok := manager.Get(10); ok {
		t.Fatalf("esperado tombstone remove cliente 10")
	}
}

func TestStateManagerApplyJSONIsIdempotent(t *testing.T) {
	t.Parallel()

	var manager StateManager
	src := []byte(`{"clients":[
		{"clientID":3,"clock":2,"state":{"name":"alice"}},
		{"clientID":1,"clock":1,"state":{"name":"bob"}},
		{"clientID":2,"clock":1,"state":{"name":"carla"}}
	]}`)

	manager.ApplyJSON(src)
	manager.ApplyJSON(src)

	snapshot := manager.Snapshot()
	if len(snapshot.Clients) != 3 {
		t.Fatalf("len(snapshot.Clients)=%d, want 3", len(snapshot.Clients))
	}

	if snapshot.Clients[0].ClientID != 1 || snapshot.Clients[1].ClientID != 2 || snapshot.Clients[2].ClientID != 3 {
		t.Fatalf("snapshot clients=%v, want sorted IDs 1,2,3", []uint32{
			snapshot.Clients[0].ClientID,
			snapshot.Clients[1].ClientID,
			snapshot.Clients[2].ClientID,
		})
	}
	if !bytes.Equal(snapshot.Clients[1].State, []byte(`{"name":"carla"}`)) {
		t.Fatalf("snapshot.Clients[1].State=%s, want compact state", snapshot.Clients[1].State)
	}
}

func TestStateManagerConcurrentApply(t *testing.T) {
	t.Parallel()

	var manager StateManager
	const totalClients = 3
	const half = 300
	var wg sync.WaitGroup

	for clientID := uint32(1); clientID <= totalClients; clientID++ {
		c := clientID

		wg.Add(1)
		go func() {
			defer wg.Done()
			for clock := uint32(1); clock <= half*2; clock += 2 {
				manager.Apply(&Update{
					Clients: []ClientState{
						{
							ClientID: c,
							Clock:    clock,
							State:    json.RawMessage(fmt.Sprintf(`{"client":%d,"clock":%d}`, c, clock)),
						},
					},
				})
			}
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			for clock := uint32(2); clock <= half*2; clock += 2 {
				manager.Apply(&Update{
					Clients: []ClientState{
						{
							ClientID: c,
							Clock:    clock,
							State:    json.RawMessage(fmt.Sprintf(`{"client":%d,"clock":%d}`, c, clock)),
						},
					},
				})
			}
		}()
	}

	wg.Wait()

	snapshot := manager.Snapshot()
	if len(snapshot.Clients) != totalClients {
		t.Fatalf("len(snapshot.Clients)=%d, want %d", len(snapshot.Clients), totalClients)
	}
	for _, client := range snapshot.Clients {
		if client.Clock != half*2 {
			t.Fatalf("client=%d clock=%d, want %d", client.ClientID, client.Clock, half*2)
		}
	}
}
