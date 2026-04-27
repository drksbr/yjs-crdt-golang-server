package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"yjs-go-bridge/internal/varint"
	"yjs-go-bridge/internal/ytypes"
	"yjs-go-bridge/internal/yupdate"
	"yjs-go-bridge/pkg/storage"
	pgstore "yjs-go-bridge/pkg/storage/postgres"
	"yjs-go-bridge/pkg/yawareness"
	"yjs-go-bridge/pkg/yhttp"
	"yjs-go-bridge/pkg/yjsbridge"
	"yjs-go-bridge/pkg/yprotocol"
)

const smokeIOTimeout = 15 * time.Second

func TestPostgresWebSocketSmoke_FunctionalityPersistence(t *testing.T) {
	pg := startDockerPostgres(t)
	schema := newSmokeSchema(t.Name())

	server1, closeServer1 := newPostgresSmokeServer(t, pg.dsn, schema)
	left := dialSmokeWS(t, server1.URL+"/ws?doc=notes&client=101&conn=left&persist=1")
	right := dialSmokeWS(t, server1.URL+"/ws?doc=notes&client=202&conn=right&persist=1")

	update := buildGCOnlyUpdate(19, 2)
	writeSmokeBinary(t, left, yprotocol.EncodeProtocolSyncUpdate(update))

	syncBroadcast := readSmokeBinary(t, right)
	syncMessages, err := yprotocol.DecodeProtocolMessages(syncBroadcast)
	if err != nil {
		t.Fatalf("DecodeProtocolMessages(sync) unexpected error: %v", err)
	}
	if len(syncMessages) != 1 || syncMessages[0].Sync == nil {
		t.Fatalf("syncMessages = %#v, want single sync update", syncMessages)
	}
	if syncMessages[0].Sync.Type != yprotocol.SyncMessageTypeUpdate {
		t.Fatalf("sync type = %v, want %v", syncMessages[0].Sync.Type, yprotocol.SyncMessageTypeUpdate)
	}
	if !bytes.Equal(syncMessages[0].Sync.Payload, update) {
		t.Fatalf("sync payload = %v, want %v", syncMessages[0].Sync.Payload, update)
	}

	awarenessPayload, err := yprotocol.EncodeProtocolAwarenessUpdate(&yawareness.Update{
		Clients: []yawareness.ClientState{{
			ClientID: 202,
			Clock:    1,
			State:    json.RawMessage(`{"name":"right","cursor":2}`),
		}},
	})
	if err != nil {
		t.Fatalf("EncodeProtocolAwarenessUpdate() unexpected error: %v", err)
	}
	writeSmokeBinary(t, right, awarenessPayload)

	awarenessBroadcast := readSmokeBinary(t, left)
	awarenessMessages, err := yprotocol.DecodeProtocolMessages(awarenessBroadcast)
	if err != nil {
		t.Fatalf("DecodeProtocolMessages(awareness) unexpected error: %v", err)
	}
	if len(awarenessMessages) != 1 || awarenessMessages[0].Awareness == nil {
		t.Fatalf("awarenessMessages = %#v, want single awareness update", awarenessMessages)
	}
	if len(awarenessMessages[0].Awareness.Clients) != 1 {
		t.Fatalf("len(awareness clients) = %d, want 1", len(awarenessMessages[0].Awareness.Clients))
	}
	if awarenessMessages[0].Awareness.Clients[0].ClientID != 202 {
		t.Fatalf("awareness clientID = %d, want 202", awarenessMessages[0].Awareness.Clients[0].ClientID)
	}

	closeSmokeWS(t, left)
	closeSmokeWS(t, right)
	waitPersistedSnapshot(t, pg.dsn, schema, storage.DocumentKey{Namespace: "integration", DocumentID: "notes"})
	closeServer1()

	server2, closeServer2 := newPostgresSmokeServer(t, pg.dsn, schema)
	defer closeServer2()

	probe := dialSmokeWS(t, server2.URL+"/ws?doc=notes&client=303&conn=probe")
	writeSmokeBinary(t, probe, yprotocol.EncodeProtocolSyncStep1([]byte{0x00}))
	reply := readSmokeBinary(t, probe)

	replyMessages, err := yprotocol.DecodeProtocolMessages(reply)
	if err != nil {
		t.Fatalf("DecodeProtocolMessages(step2) unexpected error: %v", err)
	}
	if len(replyMessages) != 1 || replyMessages[0].Sync == nil {
		t.Fatalf("replyMessages = %#v, want single sync step2", replyMessages)
	}
	if replyMessages[0].Sync.Type != yprotocol.SyncMessageTypeStep2 {
		t.Fatalf("reply sync type = %v, want %v", replyMessages[0].Sync.Type, yprotocol.SyncMessageTypeStep2)
	}

	expectedStep2, err := yjsbridge.DiffUpdate(update, []byte{0x00})
	if err != nil {
		t.Fatalf("DiffUpdate() unexpected error: %v", err)
	}
	if !bytes.Equal(replyMessages[0].Sync.Payload, expectedStep2) {
		t.Fatalf("reply sync payload = %v, want %v", replyMessages[0].Sync.Payload, expectedStep2)
	}
}

func TestPostgresWebSocketSmoke_Performance(t *testing.T) {
	pg := startDockerPostgres(t)
	schema := newSmokeSchema(t.Name())

	server, closeServer := newPostgresSmokeServer(t, pg.dsn, schema)
	defer closeServer()

	left := dialSmokeWS(t, server.URL+"/ws?doc=bench&client=401&conn=left")
	right := dialSmokeWS(t, server.URL+"/ws?doc=bench&client=402&conn=right")

	const iterations = 200
	start := time.Now()
	for i := 0; i < iterations; i++ {
		update := buildGCOnlyUpdate(uint32(1000+i), 1)
		payload := yprotocol.EncodeProtocolSyncUpdate(update)

		sender := left
		receiver := right
		if i%2 == 1 {
			sender = right
			receiver = left
		}

		writeSmokeBinary(t, sender, payload)
		broadcast := readSmokeBinary(t, receiver)
		messages, err := yprotocol.DecodeProtocolMessages(broadcast)
		if err != nil {
			t.Fatalf("DecodeProtocolMessages(iteration=%d) unexpected error: %v", i, err)
		}
		if len(messages) != 1 || messages[0].Sync == nil {
			t.Fatalf("messages(iteration=%d) = %#v, want single sync update", i, messages)
		}
	}

	elapsed := time.Since(start)
	throughput := float64(iterations) / elapsed.Seconds()
	t.Logf("performance smoke: %d updates em %s (%.1f updates/s)", iterations, elapsed, throughput)
	if elapsed > 10*time.Second {
		t.Fatalf("performance smoke levou %s, want <= 10s", elapsed)
	}
}

func newPostgresSmokeServer(t *testing.T, dsn, schema string) (*httptest.Server, func()) {
	t.Helper()

	store, err := pgstore.New(context.Background(), pgstore.Config{
		ConnectionString: dsn,
		Schema:           schema,
	})
	if err != nil {
		t.Fatalf("pgstore.New() unexpected error: %v", err)
	}

	handler, err := yhttp.NewServer(yhttp.ServerConfig{
		Provider:       yprotocol.NewProvider(yprotocol.ProviderConfig{Store: store}),
		ResolveRequest: resolveSmokeRequest,
	})
	if err != nil {
		store.Close()
		t.Fatalf("yhttp.NewServer() unexpected error: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/ws", handler)
	server := httptest.NewServer(mux)

	return server, func() {
		server.Close()
		store.Close()
	}
}

func resolveSmokeRequest(r *http.Request) (yhttp.Request, error) {
	query := r.URL.Query()
	documentID := strings.TrimSpace(query.Get("doc"))
	if documentID == "" {
		return yhttp.Request{}, errors.New("doc obrigatorio")
	}

	clientRaw := strings.TrimSpace(query.Get("client"))
	if clientRaw == "" {
		return yhttp.Request{}, errors.New("client obrigatorio")
	}
	clientValue, err := strconv.ParseUint(clientRaw, 10, 32)
	if err != nil {
		return yhttp.Request{}, err
	}

	return yhttp.Request{
		DocumentKey: storage.DocumentKey{
			Namespace:  "integration",
			DocumentID: documentID,
		},
		ConnectionID:   strings.TrimSpace(query.Get("conn")),
		ClientID:       uint32(clientValue),
		PersistOnClose: query.Get("persist") == "1",
	}, nil
}

func dialSmokeWS(t *testing.T, rawURL string) *websocket.Conn {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), smokeIOTimeout)
	defer cancel()
	wsURL := "ws" + strings.TrimPrefix(rawURL, "http")
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("websocket.Dial(%q) unexpected error: %v", wsURL, err)
	}
	t.Cleanup(func() { _ = conn.CloseNow() })
	return conn
}

func writeSmokeBinary(t *testing.T, conn *websocket.Conn, payload []byte) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), smokeIOTimeout)
	defer cancel()
	if err := conn.Write(ctx, websocket.MessageBinary, payload); err != nil {
		t.Fatalf("conn.Write() unexpected error: %v", err)
	}
}

func readSmokeBinary(t *testing.T, conn *websocket.Conn) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), smokeIOTimeout)
	defer cancel()
	msgType, payload, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("conn.Read() unexpected error: %v", err)
	}
	if msgType != websocket.MessageBinary {
		t.Fatalf("msgType = %v, want %v", msgType, websocket.MessageBinary)
	}
	return payload
}

func closeSmokeWS(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	if err := conn.Close(websocket.StatusNormalClosure, "done"); err != nil {
		t.Fatalf("conn.Close() unexpected error: %v", err)
	}
}

func waitPersistedSnapshot(t *testing.T, dsn, schema string, key storage.DocumentKey) {
	t.Helper()

	store, err := pgstore.New(context.Background(), pgstore.Config{
		ConnectionString: dsn,
		Schema:           schema,
	})
	if err != nil {
		t.Fatalf("pgstore.New(wait) unexpected error: %v", err)
	}
	defer store.Close()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		record, err := store.LoadSnapshot(context.Background(), key)
		if err == nil && record != nil && record.Snapshot != nil && !record.Snapshot.IsEmpty() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("snapshot persistido nao apareceu para %s/%s", key.Namespace, key.DocumentID)
}

func buildGCOnlyUpdate(client, length uint32) []byte {
	update := varint.Append(nil, 1)
	update = varint.Append(update, 1)
	update = varint.Append(update, client)
	update = varint.Append(update, 0)
	update = append(update, 0)
	update = varint.Append(update, length)
	return append(update, yupdate.EncodeDeleteSetBlockV1(ytypes.NewDeleteSet())...)
}
