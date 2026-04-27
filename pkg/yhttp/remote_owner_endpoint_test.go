package yhttp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"yjs-go-bridge/pkg/yawareness"
	"yjs-go-bridge/pkg/ynodeproto"
	"yjs-go-bridge/pkg/yprotocol"
)

func TestRemoteOwnerEndpointSharesRoomWithLocalWebSocketPeers(t *testing.T) {
	t.Parallel()

	local := newLocalHTTPServer(t, nil)
	endpoint, err := NewRemoteOwnerEndpoint(RemoteOwnerEndpointConfig{
		Local:       local,
		LocalNodeID: "node-owner",
	})
	if err != nil {
		t.Fatalf("NewRemoteOwnerEndpoint() unexpected error: %v", err)
	}

	srv := newHTTPTestServerWithHandler(t, local)
	localPeer := dialWS(t, srv.URL+"/ws?doc=room-remote-owner&client=902&conn=local")
	writeBinary(t, localPeer, yprotocol.EncodeProtocolQueryAwareness())
	_ = readBinary(t, localPeer)

	stream := newFakeRemoteOwnerStream()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- endpoint.ServeStream(ctx, stream)
	}()

	key := testDocumentKey("room-remote-owner")
	stream.pushReceive(&ynodeproto.Handshake{
		NodeID:       "node-edge",
		DocumentKey:  key,
		ConnectionID: "remote-conn",
		ClientID:     901,
		Epoch:        41,
	})

	ack := readRemoteStreamMessage(t, stream)
	handshakeAck, ok := ack.(*ynodeproto.HandshakeAck)
	if !ok {
		t.Fatalf("first stream message = %T, want *ynodeproto.HandshakeAck", ack)
	}
	if handshakeAck.NodeID != "node-owner" {
		t.Fatalf("handshakeAck.NodeID = %q, want %q", handshakeAck.NodeID, "node-owner")
	}
	if handshakeAck.DocumentKey != key {
		t.Fatalf("handshakeAck.DocumentKey = %#v, want %#v", handshakeAck.DocumentKey, key)
	}
	if handshakeAck.ConnectionID != "remote-conn" {
		t.Fatalf("handshakeAck.ConnectionID = %q, want %q", handshakeAck.ConnectionID, "remote-conn")
	}
	if handshakeAck.ClientID != 901 {
		t.Fatalf("handshakeAck.ClientID = %d, want %d", handshakeAck.ClientID, 901)
	}
	if handshakeAck.Epoch != 41 {
		t.Fatalf("handshakeAck.Epoch = %d, want %d", handshakeAck.Epoch, 41)
	}

	remoteUpdate := buildGCOnlyUpdate(91, 2)
	stream.pushReceive(&ynodeproto.DocumentUpdate{
		DocumentKey:  key,
		ConnectionID: "remote-conn",
		Epoch:        41,
		UpdateV1:     remoteUpdate,
	})

	syncBroadcast := readBinary(t, localPeer)
	syncMessages, err := yprotocol.DecodeProtocolMessages(syncBroadcast)
	if err != nil {
		t.Fatalf("DecodeProtocolMessages(syncBroadcast) unexpected error: %v", err)
	}
	if len(syncMessages) != 1 || syncMessages[0].Sync == nil {
		t.Fatalf("syncMessages = %#v, want single sync message", syncMessages)
	}
	if syncMessages[0].Sync.Type != yprotocol.SyncMessageTypeUpdate {
		t.Fatalf("syncMessages[0].Sync.Type = %v, want %v", syncMessages[0].Sync.Type, yprotocol.SyncMessageTypeUpdate)
	}
	if !bytes.Equal(syncMessages[0].Sync.Payload, remoteUpdate) {
		t.Fatalf("syncMessages[0].Sync.Payload = %v, want %v", syncMessages[0].Sync.Payload, remoteUpdate)
	}

	remoteAwareness, err := yawareness.EncodeUpdate(&yawareness.Update{
		Clients: []yawareness.ClientState{{
			ClientID: 901,
			Clock:    1,
			State:    json.RawMessage(`{"name":"remote"}`),
		}},
	})
	if err != nil {
		t.Fatalf("yawareness.EncodeUpdate(remote) unexpected error: %v", err)
	}
	stream.pushReceive(&ynodeproto.AwarenessUpdate{
		DocumentKey:  key,
		ConnectionID: "remote-conn",
		Epoch:        41,
		Payload:      remoteAwareness,
	})

	remoteAwarenessBroadcast := readBinary(t, localPeer)
	assertProtocolAwarenessState(t, remoteAwarenessBroadcast, 901, 1, `{"name":"remote"}`, false)

	localAwareness, err := yprotocol.EncodeProtocolAwarenessUpdate(&yawareness.Update{
		Clients: []yawareness.ClientState{{
			ClientID: 902,
			Clock:    3,
			State:    json.RawMessage(`{"name":"local"}`),
		}},
	})
	if err != nil {
		t.Fatalf("EncodeProtocolAwarenessUpdate(local) unexpected error: %v", err)
	}
	writeBinary(t, localPeer, localAwareness)

	localAwarenessForwarded := readRemoteStreamMessage(t, stream)
	localAwarenessMessage, ok := localAwarenessForwarded.(*ynodeproto.AwarenessUpdate)
	if !ok {
		t.Fatalf("second stream message = %T, want *ynodeproto.AwarenessUpdate", localAwarenessForwarded)
	}
	assertTypedAwarenessState(t, localAwarenessMessage.Payload, 902, 3, `{"name":"local"}`, false)

	stream.pushReceive(&ynodeproto.QueryAwarenessRequest{
		DocumentKey:  key,
		ConnectionID: "remote-conn",
		Epoch:        41,
	})

	queryReply := readRemoteStreamMessage(t, stream)
	queryAwareness, ok := queryReply.(*ynodeproto.QueryAwarenessResponse)
	if !ok {
		t.Fatalf("query reply = %T, want *ynodeproto.QueryAwarenessResponse", queryReply)
	}
	assertTypedAwarenessContains(t, queryAwareness.Payload, map[uint32]string{
		901: `{"name":"remote"}`,
		902: `{"name":"local"}`,
	})

	stream.pushReceive(&ynodeproto.Disconnect{
		DocumentKey:  key,
		ConnectionID: "remote-conn",
		Epoch:        41,
	})

	tombstone := readBinary(t, localPeer)
	assertProtocolAwarenessState(t, tombstone, 901, 2, "", true)

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("ServeStream() unexpected error: %v", err)
		}
	case <-time.After(testIOTimeout):
		t.Fatal("ServeStream() did not return after disconnect")
	}

	select {
	case <-stream.closeCh:
	case <-time.After(testIOTimeout):
		t.Fatal("remote stream was not closed after disconnect")
	}
}

func TestRemoteOwnerEndpointRejectsMismatchedRoute(t *testing.T) {
	t.Parallel()

	local := newLocalHTTPServer(t, nil)
	endpoint, err := NewRemoteOwnerEndpoint(RemoteOwnerEndpointConfig{
		Local:       local,
		LocalNodeID: "node-owner",
	})
	if err != nil {
		t.Fatalf("NewRemoteOwnerEndpoint() unexpected error: %v", err)
	}

	stream := newFakeRemoteOwnerStream()
	errCh := make(chan error, 1)
	go func() {
		errCh <- endpoint.ServeStream(context.Background(), stream)
	}()

	key := testDocumentKey("room-route-mismatch")
	stream.pushReceive(&ynodeproto.Handshake{
		NodeID:       "node-edge",
		DocumentKey:  key,
		ConnectionID: "remote-conn",
		ClientID:     903,
		Epoch:        51,
	})
	if _, ok := readRemoteStreamMessage(t, stream).(*ynodeproto.HandshakeAck); !ok {
		t.Fatal("expected handshake ack before route mismatch")
	}

	stream.pushReceive(&ynodeproto.DocumentUpdate{
		DocumentKey:  key,
		ConnectionID: "wrong-conn",
		Epoch:        51,
		UpdateV1:     buildGCOnlyUpdate(77, 1),
	})

	closeMessage := readRemoteStreamMessage(t, stream)
	closeFrame, ok := closeMessage.(*ynodeproto.Close)
	if !ok {
		t.Fatalf("close message = %T, want *ynodeproto.Close", closeMessage)
	}
	if closeFrame.ConnectionID != "remote-conn" {
		t.Fatalf("closeFrame.ConnectionID = %q, want %q", closeFrame.ConnectionID, "remote-conn")
	}
	if closeFrame.DocumentKey != key {
		t.Fatalf("closeFrame.DocumentKey = %#v, want %#v", closeFrame.DocumentKey, key)
	}
	if closeFrame.Epoch != 51 {
		t.Fatalf("closeFrame.Epoch = %d, want %d", closeFrame.Epoch, 51)
	}

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("ServeStream() error = nil, want mismatch error")
		}
		if !strings.Contains(err.Error(), "mismatch") {
			t.Fatalf("ServeStream() error = %v, want route mismatch context", err)
		}
	case <-time.After(testIOTimeout):
		t.Fatal("ServeStream() did not fail after route mismatch")
	}
}

func readRemoteStreamMessage(t *testing.T, stream *fakeRemoteOwnerStream) ynodeproto.Message {
	t.Helper()

	select {
	case message := <-stream.sends:
		return message
	case <-time.After(testIOTimeout):
		t.Fatal("remote stream did not receive message before timeout")
		return nil
	}
}

func assertProtocolAwarenessState(t *testing.T, payload []byte, wantClientID uint32, wantClock uint32, wantState string, wantNull bool) {
	t.Helper()

	messages, err := yprotocol.DecodeProtocolMessages(payload)
	if err != nil {
		t.Fatalf("DecodeProtocolMessages() unexpected error: %v", err)
	}
	if len(messages) != 1 || messages[0].Awareness == nil {
		t.Fatalf("messages = %#v, want single awareness message", messages)
	}
	assertAwarenessClientStateDecoded(t, messages[0].Awareness, wantClientID, wantClock, wantState, wantNull)
}

func assertTypedAwarenessState(t *testing.T, payload []byte, wantClientID uint32, wantClock uint32, wantState string, wantNull bool) {
	t.Helper()

	update, err := yawareness.DecodeUpdate(payload)
	if err != nil {
		t.Fatalf("yawareness.DecodeUpdate() unexpected error: %v", err)
	}
	assertAwarenessClientStateDecoded(t, update, wantClientID, wantClock, wantState, wantNull)
}

func assertTypedAwarenessContains(t *testing.T, payload []byte, want map[uint32]string) {
	t.Helper()

	update, err := yawareness.DecodeUpdate(payload)
	if err != nil {
		t.Fatalf("yawareness.DecodeUpdate() unexpected error: %v", err)
	}
	if len(update.Clients) != len(want) {
		t.Fatalf("len(update.Clients) = %d, want %d", len(update.Clients), len(want))
	}
	for _, client := range update.Clients {
		wantState, ok := want[client.ClientID]
		if !ok {
			t.Fatalf("unexpected client in awareness snapshot: %d", client.ClientID)
		}
		if client.IsNull() {
			t.Fatalf("client %d unexpectedly tombstoned in awareness snapshot", client.ClientID)
		}
		if string(client.State) != wantState {
			t.Fatalf("client %d state = %s, want %s", client.ClientID, client.State, wantState)
		}
	}
}

func assertAwarenessClientStateDecoded(t *testing.T, update *yawareness.Update, wantClientID uint32, wantClock uint32, wantState string, wantNull bool) {
	t.Helper()

	if update == nil {
		t.Fatal("awareness update = nil")
	}
	if len(update.Clients) != 1 {
		t.Fatalf("len(update.Clients) = %d, want 1", len(update.Clients))
	}

	client := update.Clients[0]
	if client.ClientID != wantClientID {
		t.Fatalf("client.ClientID = %d, want %d", client.ClientID, wantClientID)
	}
	if client.Clock != wantClock {
		t.Fatalf("client.Clock = %d, want %d", client.Clock, wantClock)
	}
	if client.IsNull() != wantNull {
		t.Fatalf("client.IsNull() = %v, want %v", client.IsNull(), wantNull)
	}
	if wantNull {
		return
	}
	if string(client.State) != wantState {
		t.Fatalf("client.State = %s, want %s", client.State, wantState)
	}
}
