package yprotocol

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/yawareness"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yjsbridge"
)

func TestPublicProtocolSyncAndEnvelopeRoundTrip(t *testing.T) {
	t.Parallel()

	update := yjsbridge.NewPersistedSnapshot().UpdateV1

	step1, err := EncodeProtocolSyncStep1FromUpdate(update)
	if err != nil {
		t.Fatalf("EncodeProtocolSyncStep1FromUpdate() unexpected error: %v", err)
	}

	decoded, err := DecodeProtocolSyncMessage(step1)
	if err != nil {
		t.Fatalf("DecodeProtocolSyncMessage() unexpected error: %v", err)
	}
	if decoded.Type != SyncMessageTypeStep1 {
		t.Fatalf("decoded.Type = %v, want %v", decoded.Type, SyncMessageTypeStep1)
	}

	left := buildGCOnlyUpdate(1, 2)
	right := buildGCOnlyUpdate(2, 1)
	mergedStep2, err := EncodeProtocolSyncStep2FromUpdates(left, right)
	if err != nil {
		t.Fatalf("EncodeProtocolSyncStep2FromUpdates() unexpected error: %v", err)
	}
	decodedStep2, err := DecodeProtocolSyncMessage(mergedStep2)
	if err != nil {
		t.Fatalf("DecodeProtocolSyncMessage(step2) unexpected error: %v", err)
	}
	if decodedStep2.Type != SyncMessageTypeStep2 {
		t.Fatalf("decodedStep2.Type = %v, want %v", decodedStep2.Type, SyncMessageTypeStep2)
	}
	expectedMerged, err := yjsbridge.MergeUpdates(left, right)
	if err != nil {
		t.Fatalf("MergeUpdates() unexpected error: %v", err)
	}
	if !bytes.Equal(decodedStep2.Payload, expectedMerged) {
		t.Fatalf("decodedStep2.Payload = %v, want %v", decodedStep2.Payload, expectedMerged)
	}

	generic, err := DecodeProtocolMessage(step1)
	if err != nil {
		t.Fatalf("DecodeProtocolMessage() unexpected error: %v", err)
	}
	if generic.Protocol != ProtocolTypeSync || generic.Sync == nil || generic.Sync.Type != SyncMessageTypeStep1 {
		t.Fatalf("generic = %#v, want sync step1", generic)
	}
}

func TestPublicProtocolAwarenessAuthAndQueryRoundTrip(t *testing.T) {
	t.Parallel()

	awarenessPayload, err := EncodeProtocolAwarenessUpdate(&yawareness.Update{
		Clients: []yawareness.ClientState{
			{ClientID: 7, Clock: 3, State: json.RawMessage(`{"name":"ramon"}`)},
		},
	})
	if err != nil {
		t.Fatalf("EncodeProtocolAwarenessUpdate() unexpected error: %v", err)
	}

	authPayload := EncodeProtocolAuthPermissionDenied("forbidden")
	queryPayload := EncodeProtocolQueryAwareness()
	stream := append(append(awarenessPayload, authPayload...), queryPayload...)

	messages, err := DecodeProtocolMessages(stream)
	if err != nil {
		t.Fatalf("DecodeProtocolMessages() unexpected error: %v", err)
	}
	if len(messages) != 3 {
		t.Fatalf("len(messages) = %d, want 3", len(messages))
	}

	if messages[0].Protocol != ProtocolTypeAwareness || messages[0].Awareness == nil || len(messages[0].Awareness.Clients) != 1 {
		t.Fatalf("messages[0] = %#v, want awareness", messages[0])
	}
	if !bytes.Equal(messages[0].Awareness.Clients[0].State, []byte(`{"name":"ramon"}`)) {
		t.Fatalf("messages[0].Awareness.Clients[0].State = %s", messages[0].Awareness.Clients[0].State)
	}
	if messages[1].Protocol != ProtocolTypeAuth || messages[1].Auth == nil || messages[1].Auth.Reason != "forbidden" {
		t.Fatalf("messages[1] = %#v, want auth forbidden", messages[1])
	}
	if messages[2].Protocol != ProtocolTypeQueryAwareness || messages[2].QueryAwareness == nil {
		t.Fatalf("messages[2] = %#v, want query-awareness", messages[2])
	}

	decodedAwareness, err := DecodeProtocolAwarenessUpdate(awarenessPayload)
	if err != nil {
		t.Fatalf("DecodeProtocolAwarenessUpdate() unexpected error: %v", err)
	}
	if len(decodedAwareness.Clients) != 1 || decodedAwareness.Clients[0].ClientID != 7 {
		t.Fatalf("decodedAwareness = %#v, want clientID=7", decodedAwareness)
	}
}

func TestPublicProtocolStreamAndContextContract(t *testing.T) {
	t.Parallel()

	stream := append(
		EncodeProtocolSyncStep1([]byte{0x00}),
		EncodeProtocolQueryAwareness()...,
	)

	messages, err := ReadProtocolMessagesFromStreamN(context.Background(), bytes.NewReader(stream), 2)
	if err != nil {
		t.Fatalf("ReadProtocolMessagesFromStreamN() unexpected error: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("len(messages) = %d, want 2", len(messages))
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = ReadProtocolMessagesFromStream(ctx, bytes.NewReader(stream))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ReadProtocolMessagesFromStream() error = %v, want %v", err, context.Canceled)
	}

	blocked := &blockingReader{unblock: make(chan struct{})}
	ctx, cancel = context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, err := ReadProtocolMessagesFromStream(ctx, blocked)
		errCh <- err
	}()
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("ReadProtocolMessagesFromStream() error = %v, want %v", err, context.Canceled)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("ReadProtocolMessagesFromStream() did not respect context cancellation")
	}
}

type blockingReader struct {
	unblock chan struct{}
}

func (r *blockingReader) Read(_ []byte) (int, error) {
	<-r.unblock
	return 0, io.EOF
}

func (r *blockingReader) Close() error {
	select {
	case <-r.unblock:
	default:
		close(r.unblock)
	}
	return nil
}
