package yprotocol

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	ybinary "github.com/drksbr/yjs-crdt-golang-server/internal/binary"
	"github.com/drksbr/yjs-crdt-golang-server/internal/varint"
)

func TestReadProtocolMessagesReadsMixedStream(t *testing.T) {
	t.Parallel()

	stream := append(
		EncodeProtocolSyncStep1([]byte{0x00}),
		buildAwarenessProtocolMessage()...,
	)
	stream = append(stream, EncodeProtocolSyncUpdate([]byte{0x01, 0x02})...)

	reader := ybinary.NewReader(stream)
	messages, err := ReadProtocolMessages(reader)
	if err != nil {
		t.Fatalf("ReadProtocolMessages() unexpected error: %v", err)
	}

	if len(messages) != 3 {
		t.Fatalf("ReadProtocolMessages() len = %d, want 3", len(messages))
	}

	if messages[0].Protocol != ProtocolTypeSync || messages[0].Sync == nil || messages[0].Sync.Type != SyncMessageTypeStep1 {
		t.Fatalf("messages[0] = %#v, want protocolo sync step1", messages[0])
	}
	if messages[1].Protocol != ProtocolTypeAwareness || messages[1].Awareness == nil || len(messages[1].Awareness.Clients) != 1 {
		t.Fatalf("messages[1] = %#v, want protocolo awareness com 1 client", messages[1])
	}
	if messages[2].Protocol != ProtocolTypeSync || messages[2].Sync == nil || messages[2].Sync.Type != SyncMessageTypeUpdate {
		t.Fatalf("messages[2] = %#v, want protocolo sync update", messages[2])
	}

	if reader.Remaining() != 0 {
		t.Fatalf("reader.Remaining() = %d, want 0", reader.Remaining())
	}
}

func TestDecodeProtocolMessagesRejectsTruncatedMessageAtEnd(t *testing.T) {
	t.Parallel()

	stream := append(EncodeProtocolSyncStep1([]byte{0x00}), 0x80)

	_, err := DecodeProtocolMessages(stream)
	if !errors.Is(err, varint.ErrUnexpectedEOF) {
		t.Fatalf("DecodeProtocolMessages() error = %v, want varint.ErrUnexpectedEOF", err)
	}
}

func TestReadProtocolMessagesNRespectsLimit(t *testing.T) {
	t.Parallel()

	stream := append(
		EncodeProtocolSyncStep1([]byte{0x00}),
		EncodeProtocolSyncUpdate([]byte{0x01})...,
	)
	stream = append(stream, EncodeProtocolSyncStep2([]byte{0x02, 0x03})...)

	reader := ybinary.NewReader(stream)
	messages, err := ReadProtocolMessagesN(reader, 2)
	if err != nil {
		t.Fatalf("ReadProtocolMessagesN() unexpected error: %v", err)
	}

	if len(messages) != 2 {
		t.Fatalf("ReadProtocolMessagesN() len = %d, want 2", len(messages))
	}
	if messages[0].Protocol != ProtocolTypeSync || messages[1].Protocol != ProtocolTypeSync {
		t.Fatalf("mensagens inesperadas: %#v", messages)
	}
	if reader.Remaining() <= 0 {
		t.Fatalf("reader.Remaining() = %d, want > 0", reader.Remaining())
	}
}

func TestReadProtocolMessagesFromStreamReadsIncrementally(t *testing.T) {
	t.Parallel()

	stream := append(
		EncodeProtocolSyncStep1([]byte{0x00}),
		buildAwarenessProtocolMessage()...,
	)
	stream = append(stream, EncodeProtocolSyncUpdate([]byte{0x01, 0x02})...)

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
	if messages[0].Protocol != ProtocolTypeSync || messages[0].Sync == nil || messages[0].Sync.Type != SyncMessageTypeStep1 {
		t.Fatalf("messages[0] = %#v, want protocolo sync step1", messages[0])
	}
	if messages[1].Protocol != ProtocolTypeAwareness || messages[1].Awareness == nil || len(messages[1].Awareness.Clients) != 1 {
		t.Fatalf("messages[1] = %#v, want protocolo awareness", messages[1])
	}
}

func TestReadProtocolMessagesFromStreamRejectsTruncatedMessageAtEnd(t *testing.T) {
	t.Parallel()

	stream := append(EncodeProtocolSyncStep1([]byte{0x00}), 0x80)
	_, err := ReadProtocolMessagesFromStream(context.Background(), bytes.NewReader(stream))
	if !errors.Is(err, varint.ErrUnexpectedEOF) {
		t.Fatalf("ReadProtocolMessagesFromStream() error = %v, want varint.ErrUnexpectedEOF", err)
	}
}

func TestReadProtocolMessagesFromStreamRespectsByteLimit(t *testing.T) {
	t.Parallel()

	stream := append(EncodeProtocolSyncStep1([]byte{0x00, 0x01}), buildAwarenessProtocolMessage()...)

	_, err := ReadProtocolMessagesFromStreamNWithLimit(context.Background(), bytes.NewReader(stream), 1, 4)
	if !errors.Is(err, ErrProtocolStreamByteLimitExceeded) {
		t.Fatalf("ReadProtocolMessagesFromStreamNWithLimit() error = %v, want ErrProtocolStreamByteLimitExceeded", err)
	}
}

func TestReadProtocolMessagesFromStreamRespectsContextCancel(t *testing.T) {
	t.Parallel()

	reader := &blockingReader{unblock: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		_, err := ReadProtocolMessagesFromStream(ctx, reader)
		errCh <- err
	}()

	cancel()
	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("ReadProtocolMessagesFromStream() error = %v, want context.Canceled", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("ReadProtocolMessagesFromStream() não respeitou cancelamento do contexto")
	}
}

func TestReadProtocolMessagesFromStreamUsesReaderContextWhenAvailable(t *testing.T) {
	t.Parallel()

	stream := &contextAwareReader{
		payload: EncodeProtocolSyncStep1([]byte{0x00}),
	}

	_, err := ReadProtocolMessagesFromStream(context.Background(), stream)
	if err != nil {
		t.Fatalf("ReadProtocolMessagesFromStream() unexpected error: %v", err)
	}
	if !stream.readContextCalled {
		t.Fatal("esperava leitura usando ReadContext, mas foi usado o caminho padrão")
	}
}

func TestReadProtocolMessagesFromStreamUsesReadDeadlineWhenAvailable(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	stream := &deadlineAwareReader{
		payload: EncodeProtocolSyncStep1([]byte{0x00}),
	}

	_, err := ReadProtocolMessagesFromStreamN(ctx, stream, 1)
	if err != nil {
		t.Fatalf("ReadProtocolMessagesFromStreamN() unexpected error: %v", err)
	}
	if !stream.deadlineSet {
		t.Fatal("esperava SetReadDeadline para reader com suporte de deadline")
	}
	if stream.deadlineCalls == 0 {
		t.Fatal("esperava chamada de SetReadDeadline para reader com suporte de deadline")
	}
	if stream.setDeadlinePayload.IsZero() {
		t.Fatal("esperava um deadline valido em SetReadDeadline")
	}
}

func TestReadProtocolMessagesFromStreamReturnsContextCanceledBeforeReadForNonCancelableReader(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	stream := &blockingReaderNoClose{unblock: make(chan struct{})}
	_, err := ReadProtocolMessagesFromStream(ctx, stream)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ReadProtocolMessagesFromStream() error = %v, want context.Canceled", err)
	}
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

type blockingReader struct {
	unblock chan struct{}
	once    sync.Once
}

func (r *blockingReader) Read(p []byte) (int, error) {
	<-r.unblock
	return 0, io.EOF
}

func (r *blockingReader) Close() error {
	r.once.Do(func() {
		close(r.unblock)
	})
	return nil
}

type contextAwareReader struct {
	payload           []byte
	offset            int
	readContextCalled bool
}

func (r *contextAwareReader) ReadContext(_ context.Context, p []byte) (int, error) {
	r.readContextCalled = true
	return r.read(p)
}

func (r *contextAwareReader) Read(p []byte) (int, error) {
	return r.read(p)
}

func (r *contextAwareReader) read(p []byte) (int, error) {
	if r.offset >= len(r.payload) {
		return 0, io.EOF
	}

	n := copy(p, r.payload[r.offset:])
	r.offset += n

	if r.offset >= len(r.payload) {
		return n, io.EOF
	}

	return n, nil
}

type deadlineAwareReader struct {
	payload            []byte
	offset             int
	deadlineSet        bool
	deadlineCalls      int
	setDeadlinePayload time.Time
}

func (r *deadlineAwareReader) SetReadDeadline(t time.Time) error {
	r.deadlineSet = true
	r.deadlineCalls++
	if !t.IsZero() {
		r.setDeadlinePayload = t
	}
	return nil
}

func (r *deadlineAwareReader) Read(p []byte) (int, error) {
	if r.offset >= len(r.payload) {
		return 0, io.EOF
	}

	n := copy(p, r.payload[r.offset:])
	r.offset += n

	if r.offset >= len(r.payload) {
		return n, io.EOF
	}

	return n, nil
}

type blockingReaderNoClose struct {
	unblock chan struct{}
}

func (r *blockingReaderNoClose) Read(p []byte) (int, error) {
	<-r.unblock
	return 0, io.EOF
}
