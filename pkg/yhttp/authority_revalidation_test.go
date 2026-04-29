package yhttp

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage/memory"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/ycluster"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yprotocol"
)

func TestNewRebalanceAuthorityRevalidationCallbackRevalidatesUniqueChangedDocuments(t *testing.T) {
	t.Parallel()

	duplicateKey := testDocumentKey("doc-authority-rebalance-dedupe")
	fallbackKey := testDocumentKey("doc-authority-rebalance-fallback")
	skippedKey := testDocumentKey("doc-authority-rebalance-skipped")

	duplicateErr := errors.New("duplicate revalidation failed")
	fallbackErr := errors.New("fallback revalidation failed")
	skippedErr := errors.New("skipped revalidation failed")

	resolver := newSequencedAuthorityResolver(map[storage.DocumentKey][]authorityResolverResponse{
		duplicateKey: {
			{fence: newTestAuthorityFence("node-a", 1, "token-duplicate")},
			{err: duplicateErr},
		},
		fallbackKey: {
			{fence: newTestAuthorityFence("node-a", 1, "token-fallback")},
			{err: fallbackErr},
		},
		skippedKey: {
			{fence: newTestAuthorityFence("node-a", 1, "token-skipped")},
			{err: skippedErr},
		},
	})

	server := newAuthorityRevalidationTestServer(t, resolver.resolve)
	registerAuthorityRevalidationSession(t, server, duplicateKey, "duplicate-conn", 801, nil, nil)
	registerAuthorityRevalidationSession(t, server, fallbackKey, "fallback-conn", 802, nil, nil)
	registerAuthorityRevalidationSession(t, server, skippedKey, "skipped-conn", 803, nil, nil)

	type onErrorCall struct {
		key storage.DocumentKey
		err error
	}
	var calls []onErrorCall
	callback, err := NewRebalanceAuthorityRevalidationCallback(RebalanceAuthorityRevalidationConfig{
		Server:  server,
		Timeout: time.Second,
		OnError: func(key storage.DocumentKey, err error) {
			calls = append(calls, onErrorCall{key: key, err: err})
		},
	})
	if err != nil {
		t.Fatalf("NewRebalanceAuthorityRevalidationCallback() unexpected error: %v", err)
	}

	callback(ycluster.RebalanceControllerRunResult{
		Results: []ycluster.RebalancePlanExecutionResult{
			{
				Planned: ycluster.PlannedRebalance{DocumentKey: duplicateKey},
				Result: &ycluster.RebalanceDocumentResult{
					DocumentKey: duplicateKey,
					Changed:     true,
				},
			},
			{
				Planned: ycluster.PlannedRebalance{DocumentKey: duplicateKey},
				Result: &ycluster.RebalanceDocumentResult{
					DocumentKey: duplicateKey,
					Changed:     true,
				},
			},
			{
				Planned: ycluster.PlannedRebalance{DocumentKey: fallbackKey},
				Result: &ycluster.RebalanceDocumentResult{
					DocumentKey: storage.DocumentKey{},
					Changed:     true,
				},
			},
			{
				Planned: ycluster.PlannedRebalance{DocumentKey: skippedKey},
				Result: &ycluster.RebalanceDocumentResult{
					DocumentKey: skippedKey,
					Changed:     true,
				},
				Err: errors.New("controller execution failed"),
			},
		},
	}, nil)

	if len(calls) != 2 {
		t.Fatalf("len(OnError calls) = %d, want 2", len(calls))
	}

	callsByKey := make(map[storage.DocumentKey]error, len(calls))
	for _, call := range calls {
		if _, exists := callsByKey[call.key]; exists {
			t.Fatalf("OnError called more than once for %#v", call.key)
		}
		callsByKey[call.key] = call.err
	}

	if err := callsByKey[duplicateKey]; !errors.Is(err, duplicateErr) {
		t.Fatalf("OnError(%#v) error = %v, want %v", duplicateKey, err, duplicateErr)
	}
	if err := callsByKey[fallbackKey]; !errors.Is(err, fallbackErr) {
		t.Fatalf("OnError(%#v) error = %v, want %v", fallbackKey, err, fallbackErr)
	}
	if _, ok := callsByKey[skippedKey]; ok {
		t.Fatalf("OnError called for execution.Err result %#v", skippedKey)
	}
}

func TestRevalidateDocumentAuthorityChecksAllSessionsAndSignalsWithoutBlocking(t *testing.T) {
	t.Parallel()

	key := testDocumentKey("doc-authority-revalidate-multi")
	resolver := newSequencedAuthorityResolver(map[storage.DocumentKey][]authorityResolverResponse{
		key: {
			{fence: newTestAuthorityFence("node-a", 1, "token-before")},
			{fence: newTestAuthorityFence("node-b", 2, "token-after")},
		},
	})

	server := newAuthorityRevalidationTestServer(t, resolver.resolve)

	blockedSignals := make(chan remoteOwnerCloseSignal, 1)
	blockedSignals <- remoteOwnerCloseSignal{reason: "prefilled", retryable: true}
	deliveredSignals := make(chan remoteOwnerCloseSignal, 1)

	var cancelFirst atomic.Int32
	var cancelSecond atomic.Int32
	registerAuthorityRevalidationSession(t, server, key, "conn-first", 811, blockedSignals, func() {
		cancelFirst.Add(1)
	})
	registerAuthorityRevalidationSession(t, server, key, "conn-second", 812, deliveredSignals, func() {
		cancelSecond.Add(1)
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	done := make(chan struct{})
	var (
		result AuthorityRevalidationResult
		err    error
	)
	go func() {
		result, err = server.RevalidateDocumentAuthority(ctx, key)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("RevalidateDocumentAuthority() blocked with a full signal channel")
	}

	if err != nil {
		t.Fatalf("RevalidateDocumentAuthority() unexpected error: %v", err)
	}
	if result.DocumentKey != key {
		t.Fatalf("result.DocumentKey = %#v, want %#v", result.DocumentKey, key)
	}
	if result.Checked != 2 {
		t.Fatalf("result.Checked = %d, want 2", result.Checked)
	}
	if result.AuthorityLost != 2 {
		t.Fatalf("result.AuthorityLost = %d, want 2", result.AuthorityLost)
	}
	if cancelFirst.Load() != 1 {
		t.Fatalf("first cancel count = %d, want 1", cancelFirst.Load())
	}
	if cancelSecond.Load() != 1 {
		t.Fatalf("second cancel count = %d, want 1", cancelSecond.Load())
	}

	select {
	case signal := <-deliveredSignals:
		if signal.reason != authorityLostCloseReason || !signal.retryable {
			t.Fatalf("delivered signal = %#v, want retryable authority loss", signal)
		}
	default:
		t.Fatal("expected authority loss signal for second session")
	}

	select {
	case signal := <-blockedSignals:
		if signal.reason != "prefilled" || !signal.retryable {
			t.Fatalf("blocked signal channel value = %#v, want preserved prefilled value", signal)
		}
	default:
		t.Fatal("expected prefilled value to remain on blocked signal channel")
	}
}

func TestLocalConnectionPeerSignalsAuthorityLossOnClientWrite(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	key := testDocumentKey("doc-authority-local-write-loss")
	store, resolver, provider := newAuthoritativeHTTPProvider(t, "node-a")
	seedAuthoritativeHTTPDocument(t, ctx, store, resolver, key, "node-a", 1, "lease-node-a")

	server, err := NewServer(ServerConfig{
		Provider:       provider,
		ResolveRequest: resolveTestRequest,
	})
	if err != nil {
		t.Fatalf("NewServer() unexpected error: %v", err)
	}
	connection, err := provider.Open(ctx, key, "conn-local-write-loss", 821)
	if err != nil {
		t.Fatalf("provider.Open() unexpected error: %v", err)
	}
	t.Cleanup(func() {
		_, _ = connection.Close()
	})

	handoffAuthoritativeHTTPDocument(t, ctx, store, resolver, key, "lease-node-a", "node-b", 2, "lease-node-b")

	signals := make(chan remoteOwnerCloseSignal, 1)
	peer := &localConnectionPeer{
		server:     server,
		req:        Request{DocumentKey: key, ConnectionID: "conn-local-write-loss", ClientID: 821},
		connection: connection,
		peer:       noopRoomPeer{},
		onAuthorityLoss: func(signal remoteOwnerCloseSignal) {
			signalRemoteOwnerClose(signals, signal)
		},
	}

	err = peer.deliver(ctx, yprotocol.EncodeProtocolSyncUpdate(buildGCOnlyUpdate(821, 1)))
	if !errors.Is(err, yprotocol.ErrAuthorityLost) {
		t.Fatalf("localConnectionPeer.deliver() error = %v, want %v", err, yprotocol.ErrAuthorityLost)
	}
	if !connection.AuthorityLost() {
		t.Fatal("connection.AuthorityLost() = false, want true")
	}

	select {
	case signal := <-signals:
		if signal.reason != authorityLostCloseReason || !signal.retryable {
			t.Fatalf("signal = %#v, want retryable authority loss", signal)
		}
	default:
		t.Fatal("expected authority loss signal from localConnectionPeer")
	}
}

type authorityResolverResponse struct {
	fence *storage.AuthorityFence
	err   error
}

type sequencedAuthorityResolver struct {
	mu        sync.Mutex
	responses map[storage.DocumentKey][]authorityResolverResponse
}

func newSequencedAuthorityResolver(
	responses map[storage.DocumentKey][]authorityResolverResponse,
) *sequencedAuthorityResolver {
	return &sequencedAuthorityResolver{responses: responses}
}

func (r *sequencedAuthorityResolver) resolve(_ context.Context, key storage.DocumentKey) (*storage.AuthorityFence, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	queue := r.responses[key]
	if len(queue) == 0 {
		return nil, fmt.Errorf("test resolver missing response for %#v", key)
	}
	response := queue[0]
	if len(queue) > 1 {
		r.responses[key] = queue[1:]
	}
	if response.fence == nil {
		return nil, response.err
	}
	return response.fence.Clone(), response.err
}

func newAuthorityRevalidationTestServer(
	t *testing.T,
	resolve func(context.Context, storage.DocumentKey) (*storage.AuthorityFence, error),
) *Server {
	t.Helper()

	provider := yprotocol.NewProvider(yprotocol.ProviderConfig{
		Store:                 memory.New(),
		ResolveAuthorityFence: resolve,
	})
	server, err := NewServer(ServerConfig{
		Provider:       provider,
		ResolveRequest: resolveTestRequest,
	})
	if err != nil {
		t.Fatalf("NewServer() unexpected error: %v", err)
	}
	return server
}

func registerAuthorityRevalidationSession(
	t *testing.T,
	server *Server,
	key storage.DocumentKey,
	connectionID string,
	clientID uint32,
	signals chan remoteOwnerCloseSignal,
	cancel func(),
) {
	t.Helper()

	connection, err := server.provider.Open(context.Background(), key, connectionID, clientID)
	if err != nil {
		t.Fatalf("provider.Open(%#v, %q, %d) unexpected error: %v", key, connectionID, clientID, err)
	}
	if cancel == nil {
		cancel = func() {}
	}
	server.authorityRevalidations.add(key, connectionID, &authorityRevalidationSession{
		req: Request{
			DocumentKey:  key,
			ConnectionID: connectionID,
			ClientID:     clientID,
		},
		connection:    connection,
		cancelSession: cancel,
		signals:       signals,
	})
	t.Cleanup(func() {
		server.authorityRevalidations.remove(key, connectionID)
		_, _ = connection.Close()
	})
}

func newTestAuthorityFence(node storage.NodeID, epoch uint64, token string) *storage.AuthorityFence {
	return &storage.AuthorityFence{
		ShardID: storage.ShardID("test-shard"),
		Owner: storage.OwnerInfo{
			NodeID: node,
			Epoch:  epoch,
		},
		Token: token,
	}
}

type noopRoomPeer struct{}

func (noopRoomPeer) deliver(context.Context, []byte) error { return nil }
func (noopRoomPeer) close(string) error                    { return nil }
