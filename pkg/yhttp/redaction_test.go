package yhttp

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage/memory"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yprotocol"
)

func TestHashingRequestRedactorRedactsSensitiveFields(t *testing.T) {
	t.Parallel()

	req := Request{
		DocumentKey:  storage.DocumentKey{Namespace: "tenant-a", DocumentID: "secret-doc"},
		ConnectionID: "conn-secret",
		Principal: &Principal{
			Subject: "alice@example.com",
			Tenant:  "tenant-a",
			Scopes:  []string{"doc:write"},
		},
	}
	redacted := HashingRequestRedactor{Salt: "test-salt"}.RedactRequest(req)

	for _, value := range []string{
		redacted.DocumentKey.Namespace,
		redacted.DocumentKey.DocumentID,
		redacted.ConnectionID,
		redacted.Principal.Subject,
		redacted.Principal.Tenant,
	} {
		if !strings.HasPrefix(value, "redacted-") {
			t.Fatalf("redacted value = %q, want redacted-*", value)
		}
	}
	if redacted.Principal.Scopes[0] != "redacted" {
		t.Fatalf("redacted scopes = %v, want [redacted]", redacted.Principal.Scopes)
	}
	if req.Principal.Subject != "alice@example.com" {
		t.Fatal("RedactRequest mutated original principal")
	}
}

func TestServerRedactsMetricsAndErrorHandlerRequests(t *testing.T) {
	t.Parallel()

	metrics := &capturingMetrics{}
	errCh := make(chan capturedErrorRequest, 1)
	handler, err := NewServer(ServerConfig{
		Provider: yprotocol.NewProvider(yprotocol.ProviderConfig{Store: memory.New()}),
		ResolveRequest: func(r *http.Request) (Request, error) {
			return Request{
				DocumentKey:  storage.DocumentKey{Namespace: "tenant-a", DocumentID: "secret-doc"},
				ConnectionID: "conn-secret",
				ClientID:     1,
			}, nil
		},
		OriginPolicy: OriginPolicyFunc(func(context.Context, *http.Request, Request) error {
			return ErrOriginDenied
		}),
		Redactor: HashingRequestRedactor{Salt: "test-salt"},
		Metrics:  metrics,
		OnError: func(r *http.Request, req Request, _ error) {
			errCh <- capturedErrorRequest{httpRequest: r, request: req}
		},
	})
	if err != nil {
		t.Fatalf("NewServer() unexpected error: %v", err)
	}
	srv := newHTTPTestServerWithHandler(t, handler)

	_, response, err := dialWSForAuth(t, srv.URL+"/ws?doc=secret-doc&client=1", http.Header{
		"Origin":                   []string{"https://blocked.example.com"},
		"Authorization":            []string{"Bearer secret"},
		"Cookie":                   []string{"session=secret"},
		"X-Yjs-Node-Authorization": []string{"Bearer node-secret"},
	})
	if err == nil {
		t.Fatal("websocket.Dial() error = nil, want origin rejection")
	}
	if response == nil || response.StatusCode != http.StatusForbidden {
		t.Fatalf("response status = %v, want %d", responseStatus(response), http.StatusForbidden)
	}

	metricsReq, ok := metrics.errorRequest()
	if !ok {
		t.Fatal("metrics did not capture error request")
	}
	assertRedactedRequest(t, metricsReq)
	select {
	case captured := <-errCh:
		assertRedactedRequest(t, captured.request)
		assertRedactedHTTPRequest(t, captured.httpRequest)
	case <-time.After(testIOTimeout):
		t.Fatal("OnError did not receive request")
	}
}

func TestServerRedactsAuthenticatedPrincipalOnAuthorizationError(t *testing.T) {
	t.Parallel()

	errCh := make(chan Request, 1)
	handler, err := NewServer(ServerConfig{
		Provider: yprotocol.NewProvider(yprotocol.ProviderConfig{Store: memory.New()}),
		ResolveRequest: func(r *http.Request) (Request, error) {
			return Request{
				DocumentKey:  storage.DocumentKey{Namespace: "tenant-a", DocumentID: "secret-doc"},
				ConnectionID: "conn-secret",
				ClientID:     1,
			}, nil
		},
		Authenticator: AuthenticatorFunc(func(context.Context, *http.Request) (*Principal, error) {
			return &Principal{
				Subject: "alice@example.com",
				Tenant:  "tenant-a",
				Scopes:  []string{"doc:write"},
			}, nil
		}),
		Authorizer: AuthorizerFunc(func(context.Context, *Principal, Request) error {
			return ErrForbidden
		}),
		Redactor: HashingRequestRedactor{Salt: "test-salt"},
		OnError: func(_ *http.Request, req Request, _ error) {
			errCh <- req
		},
	})
	if err != nil {
		t.Fatalf("NewServer() unexpected error: %v", err)
	}
	srv := newHTTPTestServerWithHandler(t, handler)

	_, response, err := dialWSForAuth(t, srv.URL+"/ws?doc=secret-doc&client=1", nil)
	if err == nil {
		t.Fatal("websocket.Dial() error = nil, want authorization rejection")
	}
	if response == nil || response.StatusCode != http.StatusForbidden {
		t.Fatalf("response status = %v, want %d", responseStatus(response), http.StatusForbidden)
	}

	select {
	case req := <-errCh:
		assertRedactedRequest(t, req)
		if req.Principal == nil {
			t.Fatal("redacted principal is nil")
		}
		if req.Principal.Subject == "alice@example.com" || req.Principal.Tenant == "tenant-a" {
			t.Fatalf("principal was not redacted: %#v", req.Principal)
		}
		if len(req.Principal.Scopes) != 1 || req.Principal.Scopes[0] != "redacted" {
			t.Fatalf("principal scopes = %v, want [redacted]", req.Principal.Scopes)
		}
	case <-time.After(testIOTimeout):
		t.Fatal("OnError did not receive authorization request")
	}
}

type capturedErrorRequest struct {
	httpRequest *http.Request
	request     Request
}

type capturingMetrics struct {
	mu  sync.Mutex
	req Request
	ok  bool
}

func (m *capturingMetrics) ConnectionOpened(Request)              {}
func (m *capturingMetrics) ConnectionClosed(Request)              {}
func (m *capturingMetrics) FrameRead(Request, int)                {}
func (m *capturingMetrics) FrameWritten(Request, string, int)     {}
func (m *capturingMetrics) Handle(Request, time.Duration, error)  {}
func (m *capturingMetrics) Persist(Request, time.Duration, error) {}

func (m *capturingMetrics) Error(req Request, _ string, _ error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.req = req
	m.ok = true
}

func (m *capturingMetrics) errorRequest() (Request, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.req, m.ok
}

func assertRedactedRequest(t *testing.T, req Request) {
	t.Helper()

	if req.DocumentKey.Namespace == "tenant-a" || req.DocumentKey.DocumentID == "secret-doc" || req.ConnectionID == "conn-secret" {
		t.Fatalf("request was not redacted: %#v", req)
	}
	if !strings.HasPrefix(req.DocumentKey.DocumentID, "redacted-") {
		t.Fatalf("redacted document id = %q, want redacted-*", req.DocumentKey.DocumentID)
	}
}

func assertRedactedHTTPRequest(t *testing.T, r *http.Request) {
	t.Helper()

	if r == nil {
		t.Fatal("redacted HTTP request is nil")
	}
	for _, header := range sensitiveHTTPHeaders() {
		if got := r.Header.Get(header); got != "" {
			t.Fatalf("%s header = %q, want empty", header, got)
		}
	}
	if r.URL != nil && strings.Contains(r.URL.RawQuery, "secret-doc") {
		t.Fatalf("RawQuery = %q, want redacted", r.URL.RawQuery)
	}
}
