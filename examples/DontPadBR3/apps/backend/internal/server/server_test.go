package server

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/common"
	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/config"
	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/security"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yhttp"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yjsbridge"
)

func TestResolveWSRequestDefaultsToV2(t *testing.T) {
	t.Parallel()

	srv := testServer()
	req := testWSRequest(t, srv, "pad-v2-default", "")

	resolved, err := srv.resolveWSRequest(req)
	if err != nil {
		t.Fatalf("resolveWSRequest() unexpected error: %v", err)
	}
	if resolved.SyncOutputFormat != yjsbridge.UpdateFormatV2 {
		t.Fatalf("SyncOutputFormat = %s, want %s", resolved.SyncOutputFormat, yjsbridge.UpdateFormatV2)
	}
}

func TestResolveWSRequestHonorsExplicitSyncFormat(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name     string
		query    string
		header   string
		protocol string
		want     yjsbridge.UpdateFormat
	}{
		{name: "query v1", query: "sync=v1", want: yjsbridge.UpdateFormatV1},
		{name: "query v2", query: "sync=v2", want: yjsbridge.UpdateFormatV2},
		{name: "header v1", header: "v1", want: yjsbridge.UpdateFormatV1},
		{name: "subprotocol v1", protocol: yhttp.SyncOutputFormatSubprotocolV1, want: yjsbridge.UpdateFormatV1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := testServer()
			req := testWSRequest(t, srv, "pad-explicit-sync", tt.query)
			if tt.header != "" {
				req.Header.Set(yhttp.SyncOutputFormatHeader, tt.header)
			}
			if tt.protocol != "" {
				req.Header.Set("Sec-WebSocket-Protocol", tt.protocol)
			}

			resolved, err := srv.resolveWSRequest(req)
			if err != nil {
				t.Fatalf("resolveWSRequest() unexpected error: %v", err)
			}
			if resolved.SyncOutputFormat != tt.want {
				t.Fatalf("SyncOutputFormat = %s, want %s", resolved.SyncOutputFormat, tt.want)
			}
		})
	}
}

func testServer() *Server {
	sec := security.New(security.Deps{
		Namespace: "tests",
		Address:   common.DefaultAddress,
		Secret:    []byte("test-secret"),
	})
	return &Server{
		cfg: config.Config{
			Namespace: "tests",
		},
		security: sec,
	}
}

func testWSRequest(t *testing.T, srv *Server, documentID, extraQuery string) *http.Request {
	t.Helper()

	token, err := srv.security.SignToken(common.SignedToken{
		DocumentID: documentID,
		Scope:      "ws",
		ExpiresAt:  time.Now().Add(time.Minute).Unix(),
	})
	if err != nil {
		t.Fatalf("SignToken() unexpected error: %v", err)
	}

	values := url.Values{}
	values.Set("doc", documentID)
	values.Set("client", "101")
	values.Set("token", token)
	if extraQuery != "" {
		extraValues, err := url.ParseQuery(extraQuery)
		if err != nil {
			t.Fatalf("url.ParseQuery(%q) unexpected error: %v", extraQuery, err)
		}
		for key, vals := range extraValues {
			for _, value := range vals {
				values.Add(key, value)
			}
		}
	}

	req, err := http.NewRequest(http.MethodGet, "http://example.test/ws?"+values.Encode(), nil)
	if err != nil {
		t.Fatalf("http.NewRequest() unexpected error: %v", err)
	}
	return req
}
