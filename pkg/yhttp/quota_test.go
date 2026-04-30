package yhttp

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage/memory"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yprotocol"
)

func TestLocalQuotaLimiterRejectsSecondDocumentConnectionAndReleases(t *testing.T) {
	t.Parallel()

	limiter, err := NewLocalQuotaLimiter(LocalQuotaLimiterConfig{
		MaxConnectionsPerDocument: 1,
	})
	if err != nil {
		t.Fatalf("NewLocalQuotaLimiter() unexpected error: %v", err)
	}
	handler := newQuotaHTTPServer(t, limiter)
	srv := newHTTPTestServerWithHandler(t, handler)

	first := dialWS(t, srv.URL+"/ws?doc=quota-room&client=930&conn=first")
	_, response, err := dialWSForAuth(t, srv.URL+"/ws?doc=quota-room&client=931&conn=second", nil)
	if err == nil {
		t.Fatal("websocket.Dial() error = nil, want quota rejection")
	}
	if response == nil || response.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("response status = %v, want %d", responseStatus(response), http.StatusTooManyRequests)
	}

	if err := first.Close(websocket.StatusNormalClosure, "done"); err != nil {
		t.Fatalf("first.Close() unexpected error: %v", err)
	}
	waitForCondition(t, time.Second, func() bool {
		next, response, err := dialWSForAuth(t, srv.URL+"/ws?doc=quota-room&client=932&conn=third", nil)
		if err != nil {
			return false
		}
		if response == nil || response.StatusCode != http.StatusSwitchingProtocols {
			_ = next.CloseNow()
			return false
		}
		_ = next.Close(websocket.StatusNormalClosure, "done")
		return true
	})
}

func TestLocalQuotaLimiterFrameBudgets(t *testing.T) {
	t.Parallel()

	limiter, err := NewLocalQuotaLimiter(LocalQuotaLimiterConfig{
		MaxInboundFrameBytes:          2,
		MaxOutboundBytesPerConnection: 3,
	})
	if err != nil {
		t.Fatalf("NewLocalQuotaLimiter() unexpected error: %v", err)
	}
	lease, err := limiter.OpenQuota(context.Background(), nil, Request{
		DocumentKey: storage.DocumentKey{Namespace: "tenant-a", DocumentID: "doc-a"},
	})
	if err != nil {
		t.Fatalf("OpenQuota() unexpected error: %v", err)
	}
	if err := lease.AllowFrame(context.Background(), QuotaDirectionInbound, 3); !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("AllowFrame(inbound too large) error = %v, want %v", err, ErrQuotaExceeded)
	}
	if err := lease.AllowFrame(context.Background(), QuotaDirectionOutbound, 2); err != nil {
		t.Fatalf("AllowFrame(outbound first) unexpected error: %v", err)
	}
	if err := lease.AllowFrame(context.Background(), QuotaDirectionOutbound, 2); !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("AllowFrame(outbound cumulative) error = %v, want %v", err, ErrQuotaExceeded)
	}
	if err := lease.Close(context.Background()); err != nil {
		t.Fatalf("Close() unexpected error: %v", err)
	}
}

func TestHTTPServerClosesConnectionWhenInboundQuotaExceeded(t *testing.T) {
	t.Parallel()

	limiter, err := NewLocalQuotaLimiter(LocalQuotaLimiterConfig{
		MaxInboundFrameBytes: 1,
	})
	if err != nil {
		t.Fatalf("NewLocalQuotaLimiter() unexpected error: %v", err)
	}
	handler := newQuotaHTTPServer(t, limiter)
	srv := newHTTPTestServerWithHandler(t, handler)

	conn := dialWS(t, srv.URL+"/ws?doc=quota-room&client=933")
	ctx, cancel := context.WithTimeout(context.Background(), testIOTimeout)
	defer cancel()
	if err := conn.Write(ctx, websocket.MessageBinary, yprotocol.EncodeProtocolSyncStep1([]byte{0x00})); err != nil {
		t.Fatalf("conn.Write() unexpected error: %v", err)
	}
	_, _, err = conn.Read(ctx)
	if status := websocket.CloseStatus(err); status != websocket.StatusPolicyViolation {
		t.Fatalf("conn.Read() status = %v err=%v, want policy violation", status, err)
	}
}

func TestHTTPServerReturnsServiceUnavailableWhenQuotaBackendUnavailable(t *testing.T) {
	t.Parallel()

	handler := newQuotaHTTPServer(t, QuotaLimiterFunc(func(context.Context, *http.Request, Request) (QuotaLease, error) {
		return nil, ErrQuotaUnavailable
	}))
	srv := newHTTPTestServerWithHandler(t, handler)

	_, response, err := dialWSForAuth(t, srv.URL+"/ws?doc=quota-room&client=936", nil)
	if err == nil {
		t.Fatal("websocket.Dial() error = nil, want quota backend rejection")
	}
	if response == nil || response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("response status = %v, want %d", responseStatus(response), http.StatusServiceUnavailable)
	}
}

func TestHTTPServerAppliesOutboundQuotaToBroadcastPeer(t *testing.T) {
	t.Parallel()

	handler := newQuotaHTTPServer(t, broadcastQuotaLimiter{
		blockConnectionID: "right",
		maxOutboundBytes:  1,
	})
	srv := newHTTPTestServerWithHandler(t, handler)

	right := dialWS(t, srv.URL+"/ws?doc=quota-broadcast&client=934&conn=right")
	left := dialWS(t, srv.URL+"/ws?doc=quota-broadcast&client=935&conn=left")

	writeBinary(t, left, yprotocol.EncodeProtocolSyncUpdate(buildGCOnlyUpdate(935, 3)))

	ctx, cancel := context.WithTimeout(context.Background(), testIOTimeout)
	defer cancel()
	_, _, err := right.Read(ctx)
	if err == nil {
		t.Fatal("right.Read() error = nil, want close after outbound quota exceeded")
	}
}

func newQuotaHTTPServer(t *testing.T, limiter QuotaLimiter) *Server {
	t.Helper()

	handler, err := NewServer(ServerConfig{
		Provider:       yprotocol.NewProvider(yprotocol.ProviderConfig{Store: memory.New()}),
		ResolveRequest: resolveTestRequest,
		QuotaLimiter:   limiter,
	})
	if err != nil {
		t.Fatalf("NewServer() unexpected error: %v", err)
	}
	return handler
}

type broadcastQuotaLimiter struct {
	blockConnectionID string
	maxOutboundBytes  int
}

func (l broadcastQuotaLimiter) OpenQuota(_ context.Context, _ *http.Request, req Request) (QuotaLease, error) {
	if req.ConnectionID != l.blockConnectionID {
		return QuotaLeaseFunc(func(context.Context, QuotaDirection, int) error { return nil }), nil
	}
	return QuotaLeaseFunc(func(_ context.Context, direction QuotaDirection, bytes int) error {
		if direction == QuotaDirectionOutbound && bytes > l.maxOutboundBytes {
			return ErrQuotaExceeded
		}
		return nil
	}), nil
}
