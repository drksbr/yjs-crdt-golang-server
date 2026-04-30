package yhttp

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/ycluster"
)

func TestRemoteOwnerHMACAuthenticatorAcceptsValidSignature(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0).UTC()
	dialReq := testRemoteOwnerHMACDialRequest(71)
	headers := testRemoteOwnerHMACHeaders(t, now, "shared-secret", "node-edge", dialReq, "nonce-valid")

	if got := headers.Get(RemoteOwnerNodeTimestampHeader); got != strconv.FormatInt(now.Unix(), 10) {
		t.Fatalf("%s = %q, want %q", RemoteOwnerNodeTimestampHeader, got, strconv.FormatInt(now.Unix(), 10))
	}
	if got := headers.Get(RemoteOwnerNodeNonceHeader); got != "nonce-valid" {
		t.Fatalf("%s = %q, want %q", RemoteOwnerNodeNonceHeader, got, "nonce-valid")
	}
	if headers.Get(RemoteOwnerNodeSignatureHeader) == "" {
		t.Fatalf("%s = empty, want signed header", RemoteOwnerNodeSignatureHeader)
	}
	if got := headers.Get(RemoteOwnerNodeAuthorizationHeader); got != "" {
		t.Fatalf("%s = %q, want empty for HMAC flow", RemoteOwnerNodeAuthorizationHeader, got)
	}

	authReq := testRemoteOwnerHMACAuthRequest(t, "node-edge", dialReq, headers)
	authenticator := RemoteOwnerHMACAuthenticator(RemoteOwnerHMACAuthenticatorConfig{
		Secret:     "shared-secret",
		TimeWindow: time.Minute,
		NonceStore: NewInMemoryRemoteOwnerNonceStore(InMemoryRemoteOwnerNonceStoreConfig{
			Now: func() time.Time { return now },
		}),
		Now: func() time.Time { return now },
	})

	if err := authenticator(context.Background(), authReq); err != nil {
		t.Fatalf("RemoteOwnerHMACAuthenticator(valid) unexpected error: %v", err)
	}
}

func TestRemoteOwnerHMACAuthenticatorAcceptsConfiguredKeyID(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_050, 0).UTC()
	dialReq := testRemoteOwnerHMACDialRequest(74)
	headers := testRemoteOwnerHMACHeadersWithKeyID(t, now, "rotated-secret", "key-2", "node-edge", dialReq, "nonce-keyed")
	if got := headers.Get(RemoteOwnerNodeKeyIDHeader); got != "key-2" {
		t.Fatalf("%s = %q, want key-2", RemoteOwnerNodeKeyIDHeader, got)
	}

	authReq := testRemoteOwnerHMACAuthRequest(t, "node-edge", dialReq, headers)
	authenticator := RemoteOwnerHMACAuthenticator(RemoteOwnerHMACAuthenticatorConfig{
		Secrets: map[string]string{
			"key-1": "old-secret",
			"key-2": "rotated-secret",
		},
		RequireKeyID: true,
		TimeWindow:   time.Minute,
		NonceStore: NewInMemoryRemoteOwnerNonceStore(InMemoryRemoteOwnerNonceStoreConfig{
			Now: func() time.Time { return now },
		}),
		Now: func() time.Time { return now },
	})

	if err := authenticator(context.Background(), authReq); err != nil {
		t.Fatalf("RemoteOwnerHMACAuthenticator(keyed) unexpected error: %v", err)
	}
}

func TestRemoteOwnerHMACAuthenticatorRejectsRepeatedNonce(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_100, 0).UTC()
	dialReq := testRemoteOwnerHMACDialRequest(72)
	headers := testRemoteOwnerHMACHeaders(t, now, "shared-secret", "node-edge", dialReq, "nonce-replay")
	authReq := testRemoteOwnerHMACAuthRequest(t, "node-edge", dialReq, headers)
	store := NewInMemoryRemoteOwnerNonceStore(InMemoryRemoteOwnerNonceStoreConfig{
		Now: func() time.Time { return now },
	})
	authenticator := RemoteOwnerHMACAuthenticator(RemoteOwnerHMACAuthenticatorConfig{
		Secret:     "shared-secret",
		TimeWindow: time.Minute,
		NonceStore: store,
		Now:        func() time.Time { return now },
	})

	if err := authenticator(context.Background(), authReq); err != nil {
		t.Fatalf("RemoteOwnerHMACAuthenticator(first) unexpected error: %v", err)
	}
	if err := authenticator(context.Background(), authReq); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("RemoteOwnerHMACAuthenticator(replay) error = %v, want %v", err, ErrUnauthorized)
	}
}

func TestRemoteOwnerHMACAuthenticatorRejectsInvalidRequests(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_200, 0).UTC()
	dialReq := testRemoteOwnerHMACDialRequest(73)

	tests := []struct {
		name       string
		signNow    time.Time
		authNow    time.Time
		authSecret string
		mutate     func(*RemoteOwnerAuthRequest)
	}{
		{
			name:       "wrong secret",
			signNow:    now,
			authNow:    now,
			authSecret: "wrong-secret",
		},
		{
			name:       "timestamp outside window",
			signNow:    now.Add(-3 * time.Minute),
			authNow:    now,
			authSecret: "shared-secret",
		},
		{
			name:       "tampered document key",
			signNow:    now,
			authNow:    now,
			authSecret: "shared-secret",
			mutate: func(req *RemoteOwnerAuthRequest) {
				req.DocumentKey.DocumentID = "room-remote-owner-hmac-tampered"
			},
		},
		{
			name:       "tampered epoch",
			signNow:    now,
			authNow:    now,
			authSecret: "shared-secret",
			mutate: func(req *RemoteOwnerAuthRequest) {
				req.Epoch++
			},
		},
		{
			name:       "unknown key id",
			signNow:    now,
			authNow:    now,
			authSecret: "",
			mutate: func(req *RemoteOwnerAuthRequest) {
				req.Header.Set(RemoteOwnerNodeKeyIDHeader, "missing-key")
			},
		},
		{
			name:       "missing required key id",
			signNow:    now,
			authNow:    now,
			authSecret: "",
			mutate: func(req *RemoteOwnerAuthRequest) {
				req.Header.Del(RemoteOwnerNodeKeyIDHeader)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			headers := testRemoteOwnerHMACHeaders(t, tt.signNow, "shared-secret", "node-edge", dialReq, "nonce-"+tt.name)
			authReq := testRemoteOwnerHMACAuthRequest(t, "node-edge", dialReq, headers)
			if tt.mutate != nil {
				tt.mutate(&authReq)
			}

			authenticator := RemoteOwnerHMACAuthenticator(RemoteOwnerHMACAuthenticatorConfig{
				Secret: tt.authSecret,
				Secrets: map[string]string{
					"known-key": "shared-secret",
				},
				RequireKeyID: tt.authSecret == "",
				TimeWindow:   time.Minute,
				NonceStore: NewInMemoryRemoteOwnerNonceStore(InMemoryRemoteOwnerNonceStoreConfig{
					Now: func() time.Time { return tt.authNow },
				}),
				Now: func() time.Time { return tt.authNow },
			})

			if err := authenticator(context.Background(), authReq); !errors.Is(err, ErrUnauthorized) {
				t.Fatalf("RemoteOwnerHMACAuthenticator() error = %v, want %v", err, ErrUnauthorized)
			}
		})
	}
}

func testRemoteOwnerHMACHeaders(
	t *testing.T,
	now time.Time,
	secret string,
	nodeID ycluster.NodeID,
	req RemoteOwnerDialRequest,
	nonce string,
) http.Header {
	t.Helper()
	return testRemoteOwnerHMACHeadersWithKeyID(t, now, secret, "", nodeID, req, nonce)
}

func testRemoteOwnerHMACHeadersWithKeyID(
	t *testing.T,
	now time.Time,
	secret string,
	keyID string,
	nodeID ycluster.NodeID,
	req RemoteOwnerDialRequest,
	nonce string,
) http.Header {
	t.Helper()
	headersFunc := RemoteOwnerHMACAuthHeaders(RemoteOwnerHMACAuthHeadersConfig{
		Secret: secret,
		KeyID:  keyID,
		NodeID: nodeID,
		Now: func() time.Time {
			return now
		},
		NewNonce: func(context.Context) (string, error) {
			return nonce, nil
		},
	})
	headers, err := headersFunc(context.Background(), req)
	if err != nil {
		t.Fatalf("RemoteOwnerHMACAuthHeaders() unexpected error: %v", err)
	}
	return headers
}

func testRemoteOwnerHMACAuthRequest(
	t *testing.T,
	nodeID ycluster.NodeID,
	req RemoteOwnerDialRequest,
	header http.Header,
) RemoteOwnerAuthRequest {
	t.Helper()

	authReq, err := remoteOwnerAuthRequestFromDial(nodeID, req)
	if err != nil {
		t.Fatalf("remoteOwnerAuthRequestFromDial() unexpected error: %v", err)
	}
	authReq.Header = header.Clone()
	return authReq
}

func testRemoteOwnerHMACDialRequest(epoch uint64) RemoteOwnerDialRequest {
	key := testDocumentKey("room-remote-owner-hmac")
	return RemoteOwnerDialRequest{
		Request: Request{
			DocumentKey:    key,
			ConnectionID:   "remote-owner-hmac",
			ClientID:       914,
			PersistOnClose: true,
		},
		Resolution: ycluster.OwnerResolution{
			DocumentKey: key,
			Placement: ycluster.Placement{
				NodeID: "node-owner",
				Lease: &ycluster.Lease{
					Epoch: epoch,
				},
			},
		},
	}
}
