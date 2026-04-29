package yprotocol

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	ybinary "github.com/drksbr/yjs-crdt-golang-server/internal/binary"
	"github.com/drksbr/yjs-crdt-golang-server/internal/varint"
	"github.com/drksbr/yjs-crdt-golang-server/internal/ytypes"
	"github.com/drksbr/yjs-crdt-golang-server/internal/yupdate"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yawareness"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yjsbridge"
)

func TestPublicDecodeProtocolMessages_MixedEnvelope(t *testing.T) {
	t.Parallel()

	syncStep1 := EncodeProtocolSyncStep1([]byte{0x00})
	queryAwareness := EncodeProtocolQueryAwareness()
	authDenied := EncodeProtocolAuthPermissionDenied("forbidden")
	awarenessUpdate, err := EncodeProtocolAwarenessUpdate(&yawareness.Update{
		Clients: []yawareness.ClientState{
			{ClientID: 7, Clock: 3, State: json.RawMessage(`{"name":"ramon"}`)},
		},
	})
	if err != nil {
		t.Fatalf("EncodeProtocolAwarenessUpdate() unexpected error: %v", err)
	}

	stream := append(syncStep1, queryAwareness...)
	stream = append(stream, authDenied...)
	stream = append(stream, awarenessUpdate...)

	messages, err := DecodeProtocolMessages(stream)
	if err != nil {
		t.Fatalf("DecodeProtocolMessages() unexpected error: %v", err)
	}

	if len(messages) != 4 {
		t.Fatalf("DecodeProtocolMessages() len = %d, want 4", len(messages))
	}
	if messages[0].Protocol != ProtocolTypeSync || messages[0].Sync == nil || messages[0].Sync.Type != SyncMessageTypeStep1 {
		t.Fatalf("messages[0] = %#v, want sync step1", messages[0])
	}
	if messages[1].Protocol != ProtocolTypeQueryAwareness || messages[1].QueryAwareness == nil {
		t.Fatalf("messages[1] = %#v, want query-awareness", messages[1])
	}
	if messages[2].Protocol != ProtocolTypeAuth || messages[2].Auth == nil || messages[2].Auth.Type != AuthMessageTypePermissionDenied {
		t.Fatalf("messages[2] = %#v, want auth permission denied", messages[2])
	}
	if messages[3].Protocol != ProtocolTypeAwareness || messages[3].Awareness == nil || len(messages[3].Awareness.Clients) != 1 {
		t.Fatalf("messages[3] = %#v, want awareness with 1 client", messages[3])
	}
}

func TestPublicDecodeProtocolStream_ChunkedReaderAndNilContext(t *testing.T) {
	t.Parallel()

	stream := append(
		EncodeProtocolSyncStep1([]byte{0x00}),
		buildAwarenessProtocolMessageForTest(t)...,
	)

	t.Run("ReadProtocolMessagesFromStreamN reads incrementally", func(t *testing.T) {
		t.Parallel()

		reader := &chunkedReader{
			chunks: [][]byte{
				stream[:2],
				stream[2:8],
				stream[8:],
			},
		}

		messages, err := ReadProtocolMessagesFromStreamN(context.Background(), reader, 2)
		if err != nil {
			t.Fatalf("ReadProtocolMessagesFromStreamN() unexpected error: %v", err)
		}
		if len(messages) != 2 {
			t.Fatalf("ReadProtocolMessagesFromStreamN() len = %d, want 2", len(messages))
		}
	})

	t.Run("ReadProtocolMessagesFromStream accepts nil context", func(t *testing.T) {
		t.Parallel()

		var nilCtx context.Context
		messages, err := ReadProtocolMessagesFromStream(nilCtx, bytes.NewReader(stream))
		if err != nil {
			t.Fatalf("ReadProtocolMessagesFromStream() unexpected error: %v", err)
		}
		if len(messages) == 0 {
			t.Fatal("ReadProtocolMessagesFromStream() expected at least one message")
		}
	})
}

func TestPublicDecodeSyncAuthAndQueryContracts(t *testing.T) {
	t.Parallel()

	t.Run("sync step wrappers produce decodable protocol messages", func(t *testing.T) {
		t.Parallel()

		left := buildGCOnlyUpdate(3, 2)
		right := buildGCOnlyUpdate(7, 1)

		expectedStateVector, err := yjsbridge.EncodeStateVectorFromUpdates(left, right)
		if err != nil {
			t.Fatalf("EncodeStateVectorFromUpdates() unexpected error: %v", err)
		}
		step1, err := EncodeProtocolSyncStep1FromUpdates(left, right)
		if err != nil {
			t.Fatalf("EncodeProtocolSyncStep1FromUpdates() unexpected error: %v", err)
		}
		decodedStep1, err := DecodeProtocolSyncMessage(step1)
		if err != nil {
			t.Fatalf("DecodeProtocolSyncMessage() unexpected error: %v", err)
		}
		if decodedStep1.Type != SyncMessageTypeStep1 || !bytes.Equal(decodedStep1.Payload, expectedStateVector) {
			t.Fatalf("DecodeProtocolSyncMessage(step1) = %+v, want type=%v payload=%v", decodedStep1, SyncMessageTypeStep1, expectedStateVector)
		}

		expectedStep2, err := yjsbridge.MergeUpdates(left, right)
		if err != nil {
			t.Fatalf("MergeUpdates() unexpected error: %v", err)
		}
		step2, err := EncodeProtocolSyncStep2FromUpdates(left, right)
		if err != nil {
			t.Fatalf("EncodeProtocolSyncStep2FromUpdates() unexpected error: %v", err)
		}
		decodedStep2, err := DecodeProtocolSyncMessage(step2)
		if err != nil {
			t.Fatalf("DecodeProtocolSyncMessage() unexpected error: %v", err)
		}
		if decodedStep2.Type != SyncMessageTypeStep2 {
			t.Fatalf("decodedStep2.Type = %v, want %v", decodedStep2.Type, SyncMessageTypeStep2)
		}
		if !bytes.Equal(decodedStep2.Payload, expectedStep2) {
			t.Fatalf("decodedStep2.Payload = %v, want %v", decodedStep2.Payload, expectedStep2)
		}
	})

	t.Run("auth decode rejects non-auth envelope", func(t *testing.T) {
		t.Parallel()

		_, err := DecodeProtocolAuthMessage(EncodeProtocolSyncStep1([]byte{0x00}))
		if !errors.Is(err, ErrUnexpectedProtocolType) {
			t.Fatalf("DecodeProtocolAuthMessage() error = %v, want %v", err, ErrUnexpectedProtocolType)
		}
	})

	t.Run("query-awareness rejects trailing bytes", func(t *testing.T) {
		t.Parallel()

		_, err := DecodeProtocolQueryAwareness(append(EncodeProtocolQueryAwareness(), 0xff))
		if !errors.Is(err, ErrTrailingBytes) {
			t.Fatalf("DecodeProtocolQueryAwareness() error = %v, want %v", err, ErrTrailingBytes)
		}
	})
}

func TestPublicReadProtocolMessagesFromStream_UsesCancellation(t *testing.T) {
	t.Parallel()

	stream := &streamBlockingReader{
		unblock: make(chan struct{}),
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	errCh := make(chan error, 1)
	go func() {
		_, err := ReadProtocolMessagesFromStream(ctx, stream)
		errCh <- err
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("ReadProtocolMessagesFromStream() error = %v, want %v", err, context.Canceled)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("ReadProtocolMessagesFromStream() did not honor context cancellation")
	}
}

func TestPublicAwarenessProtocolRoundTripAndCopy(t *testing.T) {
	t.Parallel()

	update := &yawareness.Update{
		Clients: []yawareness.ClientState{
			{
				ClientID: 42,
				Clock:    11,
				State:    json.RawMessage(`{"status":"ok"}`),
			},
		},
	}

	enveloped, err := EncodeProtocolAwarenessUpdate(update)
	if err != nil {
		t.Fatalf("EncodeProtocolAwarenessUpdate() unexpected error: %v", err)
	}

	decodedViaEnvelope, err := DecodeProtocolAwarenessUpdate(enveloped)
	if err != nil {
		t.Fatalf("DecodeProtocolAwarenessUpdate() unexpected error: %v", err)
	}
	if len(decodedViaEnvelope.Clients) != 1 {
		t.Fatalf("DecodeProtocolAwarenessUpdate() clients len = %d, want 1", len(decodedViaEnvelope.Clients))
	}
	if string(decodedViaEnvelope.Clients[0].State) != `{"status":"ok"}` {
		t.Fatalf("decoded state = %s, want {\"status\":\"ok\"}", decodedViaEnvelope.Clients[0].State)
	}

	decodedMessage, err := DecodeProtocolMessage(enveloped)
	if err != nil {
		t.Fatalf("DecodeProtocolMessage() unexpected error: %v", err)
	}
	originalState := append([]byte(nil), decodedMessage.Awareness.Clients[0].State...)
	stateOffset := awarenessStateOffset(t, enveloped)
	enveloped[stateOffset] = 0x30

	if !bytes.Equal(decodedMessage.Awareness.Clients[0].State, originalState) {
		t.Fatalf("decoded awareness state changed after input mutation, want copy-on-wrap behavior")
	}
}

func TestPublicDecodeProtocolMessageRejectsMalformedEnvelope(t *testing.T) {
	t.Parallel()

	t.Run("unknown protocol", func(t *testing.T) {
		t.Parallel()

		src := varint.Append(nil, 127)
		src = append(src, 0x00)
		_, err := DecodeProtocolMessage(src)
		if !errors.Is(err, ErrUnknownProtocolType) {
			t.Fatalf("DecodeProtocolMessage() error = %v, want %v", err, ErrUnknownProtocolType)
		}
	})

	t.Run("trailing bytes on protocol message", func(t *testing.T) {
		t.Parallel()

		src := append(EncodeProtocolAuthPermissionDenied("nope"), 0x00)
		_, err := DecodeProtocolAuthMessage(src)
		if !errors.Is(err, ErrTrailingBytes) {
			t.Fatalf("DecodeProtocolAuthMessage() error = %v, want %v", err, ErrTrailingBytes)
		}
	})
}

func buildAwarenessProtocolMessageForTest(t *testing.T) []byte {
	t.Helper()

	message, err := EncodeProtocolAwarenessUpdate(&yawareness.Update{
		Clients: []yawareness.ClientState{
			{ClientID: 1, Clock: 1, State: json.RawMessage(`{"online":true}`)},
		},
	})
	if err != nil {
		t.Fatalf("buildAwarenessProtocolMessageForTest() unexpected error: %v", err)
	}
	return message
}

func awarenessStateOffset(t *testing.T, src []byte) int {
	t.Helper()

	r := ybinary.NewReader(src)
	// protocol type
	_, _, err := varint.Read(r)
	if err != nil {
		t.Fatalf("awarenessStateOffset() protocol read error: %v", err)
	}
	// client count
	_, _, err = varint.Read(r)
	if err != nil {
		t.Fatalf("awarenessStateOffset() count read error: %v", err)
	}
	// client id
	_, _, err = varint.Read(r)
	if err != nil {
		t.Fatalf("awarenessStateOffset() client id read error: %v", err)
	}
	// clock
	_, _, err = varint.Read(r)
	if err != nil {
		t.Fatalf("awarenessStateOffset() clock read error: %v", err)
	}
	// state len
	_, _, err = varint.Read(r)
	if err != nil {
		t.Fatalf("awarenessStateOffset() state len read error: %v", err)
	}
	return r.Offset()
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

type chunkedReader struct {
	chunks [][]byte
	index  int
}

func (r *chunkedReader) Read(p []byte) (int, error) {
	if r.index >= len(r.chunks) {
		return 0, io.EOF
	}

	chunk := r.chunks[r.index]
	r.index++

	n := copy(p, chunk)
	return n, nil
}

type streamBlockingReader struct {
	unblock chan struct{}
	once    sync.Once
}

func (r *streamBlockingReader) Read(_ []byte) (int, error) {
	<-r.unblock
	return 0, io.EOF
}

func (r *streamBlockingReader) Close() error {
	r.once.Do(func() {
		close(r.unblock)
	})
	return nil
}
