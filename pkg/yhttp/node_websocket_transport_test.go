package yhttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/ycluster"
)

func TestWebSocketRemoteOwnerDialerAddsNodeAuthHeadersWithoutForwardingClientAuthorization(t *testing.T) {
	t.Parallel()

	headersCh := make(chan http.Header, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headersCh <- cloneHeader(r.Header)
		socket, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		_ = socket.CloseNow()
	}))
	t.Cleanup(srv.Close)

	dialer, err := NewWebSocketRemoteOwnerDialer(WebSocketRemoteOwnerDialerConfig{
		ResolveURL: func(context.Context, RemoteOwnerDialRequest) (string, error) {
			return "ws" + strings.TrimPrefix(srv.URL, "http"), nil
		},
		AuthHeaders: RemoteOwnerBearerAuthHeaders("node-token"),
	})
	if err != nil {
		t.Fatalf("NewWebSocketRemoteOwnerDialer() unexpected error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), testIOTimeout)
	defer cancel()
	stream, err := dialer.DialRemoteOwner(ctx, RemoteOwnerDialRequest{
		Request: Request{
			DocumentKey:  storage.DocumentKey{Namespace: "tests", DocumentID: "node-auth"},
			ConnectionID: "conn-node-auth",
			ClientID:     1001,
		},
		Resolution: ycluster.OwnerResolution{
			Placement: ycluster.Placement{NodeID: "node-owner"},
		},
		Header: http.Header{
			"Authorization": []string{"Bearer user-token"},
			"X-Trace-Id":    []string{"trace-1"},
		},
	})
	if err != nil {
		t.Fatalf("DialRemoteOwner() unexpected error: %v", err)
	}
	defer func() {
		_ = stream.Close()
	}()

	select {
	case headers := <-headersCh:
		if got := headers.Get(RemoteOwnerNodeAuthorizationHeader); got != "Bearer node-token" {
			t.Fatalf("%s = %q, want bearer node-token", RemoteOwnerNodeAuthorizationHeader, got)
		}
		if got := headers.Get("Authorization"); got != "" {
			t.Fatalf("Authorization was forwarded to owner node: %q", got)
		}
		if got := headers.Get("X-Trace-Id"); got != "trace-1" {
			t.Fatalf("X-Trace-Id = %q, want trace-1", got)
		}
	case <-time.After(testIOTimeout):
		t.Fatal("owner websocket did not receive dial request")
	}
}

func TestRemoteOwnerBearerAuthenticator(t *testing.T) {
	t.Parallel()

	auth := RemoteOwnerBearerAuthenticator("node-token")
	if err := auth(context.Background(), RemoteOwnerAuthRequest{
		Header: http.Header{RemoteOwnerNodeAuthorizationHeader: []string{"Bearer node-token"}},
	}); err != nil {
		t.Fatalf("RemoteOwnerBearerAuthenticator(valid) unexpected error: %v", err)
	}
	if err := auth(context.Background(), RemoteOwnerAuthRequest{
		Header: http.Header{RemoteOwnerNodeAuthorizationHeader: []string{"Bearer wrong"}},
	}); err != ErrUnauthorized {
		t.Fatalf("RemoteOwnerBearerAuthenticator(wrong) error = %v, want %v", err, ErrUnauthorized)
	}
}
