package yhttp

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage/memory"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/ycluster"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yprotocol"
)

func TestHTTPServerBearerTenantAuth(t *testing.T) {
	t.Parallel()

	handler := newAuthHTTPServer(t)
	srv := newHTTPTestServerWithHandler(t, handler)

	conn := dialWSWithHeader(t, srv.URL+"/ws?doc=secure-room&client=901", bearerHeader("token-a"))
	writeBinary(t, conn, yprotocol.EncodeProtocolSyncStep1([]byte{0x00}))
	_ = readBinary(t, conn)
}

func TestHTTPServerRejectsMissingBearerToken(t *testing.T) {
	t.Parallel()

	handler := newAuthHTTPServer(t)
	srv := newHTTPTestServerWithHandler(t, handler)

	_, response, err := dialWSForAuth(t, srv.URL+"/ws?doc=secure-room&client=902", nil)
	if err == nil {
		t.Fatal("websocket.Dial() error = nil, want unauthorized")
	}
	if response == nil || response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("response status = %v, want %d", responseStatus(response), http.StatusUnauthorized)
	}
}

func TestHTTPServerRejectsCrossTenantDocument(t *testing.T) {
	t.Parallel()

	handler := newAuthHTTPServer(t)
	srv := newHTTPTestServerWithHandler(t, handler)

	_, response, err := dialWSForAuth(t, srv.URL+"/ws?doc=secure-room&client=903", bearerHeader("token-b"))
	if err == nil {
		t.Fatal("websocket.Dial() error = nil, want forbidden")
	}
	if response == nil || response.StatusCode != http.StatusForbidden {
		t.Fatalf("response status = %v, want %d", responseStatus(response), http.StatusForbidden)
	}
}

func TestOwnerAwareServerAuthorizesBeforeOwnerLookup(t *testing.T) {
	t.Parallel()

	local := newAuthHTTPServer(t)
	var lookups atomic.Int32
	lookup := ownerLookupFunc(func(_ context.Context, req ycluster.OwnerLookupRequest) (*ycluster.OwnerResolution, error) {
		lookups.Add(1)
		return &ycluster.OwnerResolution{
			DocumentKey: req.DocumentKey,
			Placement:   ycluster.Placement{ShardID: 1, NodeID: "node-a", Version: 1},
			Local:       true,
		}, nil
	})
	handler, err := NewOwnerAwareServer(OwnerAwareServerConfig{
		Local:       local,
		OwnerLookup: lookup,
	})
	if err != nil {
		t.Fatalf("NewOwnerAwareServer() unexpected error: %v", err)
	}
	srv := newHTTPTestServerWithHandler(t, handler)

	_, response, err := dialWSForAuth(t, srv.URL+"/ws?doc=secure-room&client=904", bearerHeader("token-b"))
	if err == nil {
		t.Fatal("websocket.Dial() error = nil, want forbidden")
	}
	if response == nil || response.StatusCode != http.StatusForbidden {
		t.Fatalf("response status = %v, want %d", responseStatus(response), http.StatusForbidden)
	}
	if lookups.Load() != 0 {
		t.Fatalf("owner lookups = %d, want 0 before authz success", lookups.Load())
	}
}

func TestHTTPServerRateLimiterRejectsExcessRequests(t *testing.T) {
	t.Parallel()

	handler := newRateLimitedAuthHTTPServer(t)
	srv := newHTTPTestServerWithHandler(t, handler)

	first := dialWSWithHeader(t, srv.URL+"/ws?doc=limited-room&client=906&conn=first", bearerHeader("token-a"))
	_ = first.Close(websocket.StatusNormalClosure, "done")

	_, response, err := dialWSForAuth(t, srv.URL+"/ws?doc=limited-room&client=907&conn=second", bearerHeader("token-a"))
	if err == nil {
		t.Fatal("websocket.Dial() error = nil, want rate limit rejection")
	}
	if response == nil || response.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("response status = %v, want %d", responseStatus(response), http.StatusTooManyRequests)
	}
}

func TestOwnerAwareServerRateLimitsBeforeOwnerLookup(t *testing.T) {
	t.Parallel()

	local := newRateLimitedAuthHTTPServer(t)
	var lookups atomic.Int32
	lookup := ownerLookupFunc(func(_ context.Context, req ycluster.OwnerLookupRequest) (*ycluster.OwnerResolution, error) {
		lookups.Add(1)
		return &ycluster.OwnerResolution{
			DocumentKey: req.DocumentKey,
			Placement:   ycluster.Placement{ShardID: 1, NodeID: "node-a", Version: 1},
			Local:       true,
		}, nil
	})
	handler, err := NewOwnerAwareServer(OwnerAwareServerConfig{
		Local:       local,
		OwnerLookup: lookup,
	})
	if err != nil {
		t.Fatalf("NewOwnerAwareServer() unexpected error: %v", err)
	}
	srv := newHTTPTestServerWithHandler(t, handler)

	first := dialWSWithHeader(t, srv.URL+"/ws?doc=limited-room&client=908&conn=first", bearerHeader("token-a"))
	_ = first.Close(websocket.StatusNormalClosure, "done")

	_, response, err := dialWSForAuth(t, srv.URL+"/ws?doc=limited-room&client=909&conn=second", bearerHeader("token-a"))
	if err == nil {
		t.Fatal("websocket.Dial() error = nil, want rate limit rejection")
	}
	if response == nil || response.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("response status = %v, want %d", responseStatus(response), http.StatusTooManyRequests)
	}
	if lookups.Load() != 1 {
		t.Fatalf("owner lookups = %d, want only first request to reach lookup", lookups.Load())
	}
}

func TestFixedWindowRateLimiterResetsAfterWindow(t *testing.T) {
	t.Parallel()

	now := time.Unix(100, 0)
	limiter, err := NewFixedWindowRateLimiter(FixedWindowRateLimiterConfig{
		Limit:  1,
		Window: time.Second,
		Now: func() time.Time {
			return now
		},
	})
	if err != nil {
		t.Fatalf("NewFixedWindowRateLimiter() unexpected error: %v", err)
	}
	req := Request{DocumentKey: storage.DocumentKey{Namespace: "tenant-a", DocumentID: "doc-a"}}
	principal := &Principal{Subject: "alice", Tenant: "tenant-a"}
	if err := limiter.AllowHTTP(context.Background(), nil, principal, req); err != nil {
		t.Fatalf("AllowHTTP(first) unexpected error: %v", err)
	}
	if err := limiter.AllowHTTP(context.Background(), nil, principal, req); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("AllowHTTP(second) error = %v, want %v", err, ErrRateLimited)
	}
	now = now.Add(time.Second)
	if err := limiter.AllowHTTP(context.Background(), nil, principal, req); err != nil {
		t.Fatalf("AllowHTTP(after window) unexpected error: %v", err)
	}
}

func TestAuthenticatorAndAuthorizerFunctionsReceivePrincipal(t *testing.T) {
	t.Parallel()

	var sawPrincipal atomic.Bool
	handler, err := NewServer(ServerConfig{
		Provider: yprotocol.NewProvider(yprotocol.ProviderConfig{Store: memory.New()}),
		ResolveRequest: func(r *http.Request) (Request, error) {
			req, err := resolveTestRequest(r)
			req.DocumentKey.Namespace = "tenant-a"
			return req, err
		},
		Authenticator: AuthenticatorFunc(func(context.Context, *http.Request) (*Principal, error) {
			return &Principal{Subject: "alice", Tenant: "tenant-a", Scopes: []string{"doc:write"}}, nil
		}),
		Authorizer: AuthorizerFunc(func(_ context.Context, principal *Principal, req Request) error {
			if principal != nil &&
				principal.Subject == "alice" &&
				req.Principal != nil &&
				req.Principal.Tenant == req.DocumentKey.Namespace {
				sawPrincipal.Store(true)
			}
			return nil
		}),
	})
	if err != nil {
		t.Fatalf("NewServer() unexpected error: %v", err)
	}
	srv := newHTTPTestServerWithHandler(t, handler)

	conn := dialWS(t, srv.URL+"/ws?doc=secure-room&client=905")
	writeBinary(t, conn, yprotocol.EncodeProtocolSyncStep1([]byte{0x00}))
	_ = readBinary(t, conn)
	if !sawPrincipal.Load() {
		t.Fatal("authorizer did not receive authenticated principal on request")
	}
}

func newAuthHTTPServer(t *testing.T) *Server {
	t.Helper()

	handler, err := NewServer(ServerConfig{
		Provider: yprotocol.NewProvider(yprotocol.ProviderConfig{Store: memory.New()}),
		ResolveRequest: func(r *http.Request) (Request, error) {
			req, err := resolveTestRequest(r)
			req.DocumentKey.Namespace = "tenant-a"
			return req, err
		},
		Authenticator: BearerTokenAuthenticator{
			Tokens: map[string]Principal{
				"token-a": {Subject: "alice", Tenant: "tenant-a"},
				"token-b": {Subject: "bob", Tenant: "tenant-b"},
			},
		},
		Authorizer: TenantAuthorizer{},
	})
	if err != nil {
		t.Fatalf("NewServer() unexpected error: %v", err)
	}
	return handler
}

func newRateLimitedAuthHTTPServer(t *testing.T) *Server {
	t.Helper()

	limiter, err := NewFixedWindowRateLimiter(FixedWindowRateLimiterConfig{
		Limit:  1,
		Window: time.Hour,
	})
	if err != nil {
		t.Fatalf("NewFixedWindowRateLimiter() unexpected error: %v", err)
	}
	handler := newAuthHTTPServer(t)
	handler.rateLimiter = limiter
	return handler
}

func bearerHeader(token string) http.Header {
	header := make(http.Header)
	header.Set("Authorization", "Bearer "+token)
	return header
}

func dialWSWithHeader(t *testing.T, rawURL string, header http.Header) *websocket.Conn {
	t.Helper()

	conn, response, err := dialWSForAuth(t, rawURL, header)
	if err != nil {
		t.Fatalf("websocket.Dial(%q) unexpected error: %v status=%v", rawURL, err, responseStatus(response))
	}
	t.Cleanup(func() {
		_ = conn.CloseNow()
	})
	return conn
}

func dialWSForAuth(t *testing.T, rawURL string, header http.Header) (*websocket.Conn, *http.Response, error) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), testIOTimeout)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(rawURL, "http")
	return websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: header})
}

func responseStatus(response *http.Response) int {
	if response == nil {
		return 0
	}
	return response.StatusCode
}
