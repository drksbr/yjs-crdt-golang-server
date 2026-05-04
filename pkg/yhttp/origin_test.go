package yhttp

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage/memory"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/ycluster"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yprotocol"
)

func TestHTTPServerRejectsDisallowedOrigin(t *testing.T) {
	t.Parallel()

	handler := newOriginHTTPServer(t, StaticOriginPolicy{
		AllowedOrigins:     []string{"https://app.example.com"},
		AllowMissingOrigin: true,
	})
	srv := newHTTPTestServerWithHandler(t, handler)

	header := http.Header{"Origin": []string{"https://evil.example.com"}}
	_, response, err := dialWSForAuth(t, srv.URL+"/ws?doc=origin-room&client=910", header)
	if err == nil {
		t.Fatal("websocket.Dial() error = nil, want origin rejection")
	}
	if response == nil || response.StatusCode != http.StatusForbidden {
		t.Fatalf("response status = %v, want %d", responseStatus(response), http.StatusForbidden)
	}
}

func TestHTTPServerHandlesAllowedCORSPreflight(t *testing.T) {
	t.Parallel()

	handler := newOriginHTTPServer(t, StaticOriginPolicy{
		AllowedOrigins: []string{"https://app.example.com"},
		AllowedMethods: []string{http.MethodGet},
		AllowedHeaders: []string{"Authorization"},
		MaxAge:         time.Minute,
	})
	srv := newHTTPTestServerWithHandler(t, handler)

	req, err := http.NewRequest(http.MethodOptions, srv.URL+"/ws", nil)
	if err != nil {
		t.Fatalf("NewRequest() unexpected error: %v", err)
	}
	req.Header.Set("Origin", "https://app.example.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("preflight request unexpected error: %v", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want %d", response.StatusCode, http.StatusNoContent)
	}
	if got := response.Header.Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want allowed origin", got)
	}
	if got := response.Header.Get("Access-Control-Allow-Headers"); got != "Authorization" {
		t.Fatalf("Access-Control-Allow-Headers = %q, want Authorization", got)
	}
}

func TestHTTPServerRejectsDisallowedCORSPreflightHeader(t *testing.T) {
	t.Parallel()

	handler := newOriginHTTPServer(t, StaticOriginPolicy{
		AllowedOrigins: []string{"https://app.example.com"},
		AllowedMethods: []string{http.MethodGet},
		AllowedHeaders: []string{"Authorization"},
	})
	srv := newHTTPTestServerWithHandler(t, handler)

	req, err := http.NewRequest(http.MethodOptions, srv.URL+"/ws", nil)
	if err != nil {
		t.Fatalf("NewRequest() unexpected error: %v", err)
	}
	req.Header.Set("Origin", "https://app.example.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	req.Header.Set("Access-Control-Request-Headers", "Authorization, X-Unsafe")
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("preflight request unexpected error: %v", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("preflight status = %d, want %d", response.StatusCode, http.StatusForbidden)
	}
}

func TestOwnerAwareServerRejectsOriginBeforeOwnerLookup(t *testing.T) {
	t.Parallel()

	local := newOriginHTTPServer(t, StaticOriginPolicy{
		AllowedOrigins:     []string{"https://app.example.com"},
		AllowMissingOrigin: true,
	})
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

	header := http.Header{"Origin": []string{"https://evil.example.com"}}
	_, response, err := dialWSForAuth(t, srv.URL+"/ws?doc=origin-room&client=911", header)
	if err == nil {
		t.Fatal("websocket.Dial() error = nil, want origin rejection")
	}
	if response == nil || response.StatusCode != http.StatusForbidden {
		t.Fatalf("response status = %v, want %d", responseStatus(response), http.StatusForbidden)
	}
	if lookups.Load() != 0 {
		t.Fatalf("owner lookups = %d, want 0 before origin success", lookups.Load())
	}
}

func TestHTTPServerAcceptsAllowedOrigin(t *testing.T) {
	t.Parallel()

	handler := newOriginHTTPServer(t, StaticOriginPolicy{
		AllowedOrigins: []string{"https://app.example.com"},
	})
	srv := newHTTPTestServerWithHandler(t, handler)

	conn := dialWSWithHeader(t, srv.URL+"/ws?doc=origin-room&client=912", http.Header{
		"Origin": []string{"https://app.example.com"},
	})
	writeBinary(t, conn, yprotocol.EncodeProtocolSyncStep1([]byte{0x00}))
	_ = readBinary(t, conn)
	_ = conn.Close(websocket.StatusNormalClosure, "done")
}

func TestHTTPServerWithoutOriginPolicyKeepsDefaultNoOriginBehavior(t *testing.T) {
	t.Parallel()

	handler, err := NewServer(ServerConfig{
		Provider:       yprotocol.NewProvider(yprotocol.ProviderConfig{Store: memory.New()}),
		ResolveRequest: resolveTestRequest,
	})
	if err != nil {
		t.Fatalf("NewServer() unexpected error: %v", err)
	}
	srv := newHTTPTestServerWithHandler(t, handler)

	conn := dialWS(t, srv.URL+"/ws?doc=origin-room&client=913")
	writeBinary(t, conn, yprotocol.EncodeProtocolSyncStep1([]byte{0x00}))
	_ = readBinary(t, conn)
	_ = conn.Close(websocket.StatusNormalClosure, "done")
}

func TestStaticOriginPolicyAugmentsWebSocketAcceptOptions(t *testing.T) {
	t.Parallel()

	options := cloneAcceptOptionsForOriginPolicy(&websocket.AcceptOptions{
		OriginPatterns: []string{"https://existing.example.com"},
	}, StaticOriginPolicy{
		AllowedOrigins: []string{"https://app.example.com"},
	})

	if options == nil {
		t.Fatal("cloneAcceptOptionsForOriginPolicy() = nil, want options")
	}
	if !containsOriginPattern(options.OriginPatterns, "https://existing.example.com") ||
		!containsOriginPattern(options.OriginPatterns, "https://app.example.com") {
		t.Fatalf("OriginPatterns = %v, want existing and policy origin", options.OriginPatterns)
	}
}

func TestStaticOriginWildcardUsesInsecureWebSocketOriginSkip(t *testing.T) {
	t.Parallel()

	options := cloneAcceptOptionsForOriginPolicy(nil, StaticOriginPolicy{
		AllowedOrigins: []string{"*"},
	})
	if options == nil || !options.InsecureSkipVerify || len(options.OriginPatterns) != 0 {
		t.Fatalf("options = %#v, want InsecureSkipVerify without patterns", options)
	}
}

func newOriginHTTPServer(t *testing.T, policy OriginPolicy) *Server {
	t.Helper()

	handler, err := NewServer(ServerConfig{
		Provider:       yprotocol.NewProvider(yprotocol.ProviderConfig{Store: memory.New()}),
		ResolveRequest: resolveTestRequest,
		OriginPolicy:   policy,
	})
	if err != nil {
		t.Fatalf("NewServer() unexpected error: %v", err)
	}
	return handler
}

func TestStaticOriginPolicyRejectsMissingOriginWhenConfigured(t *testing.T) {
	t.Parallel()

	policy := StaticOriginPolicy{AllowedOrigins: []string{"https://app.example.com"}}
	err := policy.CheckHTTPOrigin(context.Background(), &http.Request{Header: http.Header{}}, Request{
		DocumentKey: storage.DocumentKey{Namespace: "tests", DocumentID: "origin"},
	})
	if err != ErrOriginDenied {
		t.Fatalf("CheckHTTPOrigin() error = %v, want %v", err, ErrOriginDenied)
	}
}
