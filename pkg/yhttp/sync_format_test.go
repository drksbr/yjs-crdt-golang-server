package yhttp

import (
	"errors"
	"net/http"
	"net/url"
	"testing"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/yjsbridge"
)

func TestSyncOutputFormatFromHTTPRequest(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name      string
		rawQuery  string
		header    string
		protocols []string
		want      yjsbridge.UpdateFormat
		wantErrIs error
	}{
		{
			name: "default v1",
			want: yjsbridge.UpdateFormatV1,
		},
		{
			name:     "query v2",
			rawQuery: "sync=v2",
			want:     yjsbridge.UpdateFormatV2,
		},
		{
			name:   "header v2",
			header: "update-v2",
			want:   yjsbridge.UpdateFormatV2,
		},
		{
			name:      "subprotocol v2",
			protocols: []string{"chat, yjs-sync-v2"},
			want:      yjsbridge.UpdateFormatV2,
		},
		{
			name:      "query wins over header",
			rawQuery:  "sync=v1",
			header:    "v2",
			protocols: []string{"yjs-sync-v2"},
			want:      yjsbridge.UpdateFormatV1,
		},
		{
			name:      "header wins over subprotocol",
			header:    "v1",
			protocols: []string{"yjs-sync-v2"},
			want:      yjsbridge.UpdateFormatV1,
		},
		{
			name:      "unknown subprotocol ignored",
			protocols: []string{"chat, custom"},
			want:      yjsbridge.UpdateFormatV1,
		},
		{
			name:      "invalid query fails",
			rawQuery:  "sync=v3",
			want:      yjsbridge.UpdateFormatUnknown,
			wantErrIs: yjsbridge.ErrUnknownUpdateFormat,
		},
		{
			name:      "invalid header fails",
			header:    "v3",
			want:      yjsbridge.UpdateFormatUnknown,
			wantErrIs: yjsbridge.ErrUnknownUpdateFormat,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := &http.Request{
				URL:    &url.URL{RawQuery: tt.rawQuery},
				Header: make(http.Header),
			}
			if tt.header != "" {
				req.Header.Set(SyncOutputFormatHeader, tt.header)
			}
			for _, protocol := range tt.protocols {
				req.Header.Add("Sec-WebSocket-Protocol", protocol)
			}

			got, err := SyncOutputFormatFromHTTPRequest(req)
			if tt.wantErrIs != nil {
				if !errors.Is(err, tt.wantErrIs) {
					t.Fatalf("SyncOutputFormatFromHTTPRequest() error = %v, want %v", err, tt.wantErrIs)
				}
			} else if err != nil {
				t.Fatalf("SyncOutputFormatFromHTTPRequest() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("SyncOutputFormatFromHTTPRequest() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestSyncOutputFormatFromHTTPRequestNilRequestDefaultsV1(t *testing.T) {
	t.Parallel()

	got, err := SyncOutputFormatFromHTTPRequest(nil)
	if err != nil {
		t.Fatalf("SyncOutputFormatFromHTTPRequest(nil) unexpected error: %v", err)
	}
	if got != yjsbridge.UpdateFormatV1 {
		t.Fatalf("SyncOutputFormatFromHTTPRequest(nil) = %s, want %s", got, yjsbridge.UpdateFormatV1)
	}
}
