package yhttp

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage/memory"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/ycluster"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yprotocol"
)

func TestValidateProductionServerConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		mutate       func(*ServerConfig)
		wantIs       error
		wantContains string
	}{
		{
			name: "complete",
		},
		{
			name: "missing provider",
			mutate: func(cfg *ServerConfig) {
				cfg.Provider = nil
			},
			wantIs: ErrNilProvider,
		},
		{
			name: "missing resolve request",
			mutate: func(cfg *ServerConfig) {
				cfg.ResolveRequest = nil
			},
			wantIs: ErrNilResolveRequest,
		},
		{
			name: "missing authenticator",
			mutate: func(cfg *ServerConfig) {
				cfg.Authenticator = nil
			},
			wantIs:       ErrForbidden,
			wantContains: "ServerConfig.Authenticator",
		},
		{
			name: "missing authorizer",
			mutate: func(cfg *ServerConfig) {
				cfg.Authorizer = nil
			},
			wantIs:       ErrForbidden,
			wantContains: "ServerConfig.Authorizer",
		},
		{
			name: "missing rate limiter",
			mutate: func(cfg *ServerConfig) {
				cfg.RateLimiter = nil
			},
			wantIs:       ErrForbidden,
			wantContains: "ServerConfig.RateLimiter",
		},
		{
			name: "missing origin policy",
			mutate: func(cfg *ServerConfig) {
				cfg.OriginPolicy = nil
			},
			wantIs:       ErrForbidden,
			wantContains: "ServerConfig.OriginPolicy",
		},
		{
			name: "missing quota limiter",
			mutate: func(cfg *ServerConfig) {
				cfg.QuotaLimiter = nil
			},
			wantIs:       ErrForbidden,
			wantContains: "ServerConfig.QuotaLimiter",
		},
		{
			name: "missing redactor",
			mutate: func(cfg *ServerConfig) {
				cfg.Redactor = nil
			},
			wantIs:       ErrForbidden,
			wantContains: "ServerConfig.Redactor",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := newProductionTestServerConfig()
			if tt.mutate != nil {
				tt.mutate(&cfg)
			}

			err := ValidateProductionServerConfig(cfg)
			assertProductionValidationResult(t, err, tt.wantIs, tt.wantContains)
		})
	}
}

func TestValidateProductionOwnerAwareConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		build        func(t *testing.T) OwnerAwareServerConfig
		wantIs       error
		wantContains string
	}{
		{
			name: "complete",
			build: func(t *testing.T) OwnerAwareServerConfig {
				t.Helper()
				return newProductionTestOwnerAwareConfig(t)
			},
		},
		{
			name: "missing local server",
			build: func(t *testing.T) OwnerAwareServerConfig {
				t.Helper()
				cfg := newProductionTestOwnerAwareConfig(t)
				cfg.Local = nil
				return cfg
			},
			wantIs: ErrNilLocalServer,
		},
		{
			name: "missing owner lookup",
			build: func(t *testing.T) OwnerAwareServerConfig {
				t.Helper()
				cfg := newProductionTestOwnerAwareConfig(t)
				cfg.OwnerLookup = nil
				return cfg
			},
			wantIs: ErrNilOwnerLookup,
		},
		{
			name: "local server missing rate limiter",
			build: func(t *testing.T) OwnerAwareServerConfig {
				t.Helper()
				cfg := newProductionTestOwnerAwareConfig(t)
				cfg.Local = newProductionTestServerWithMutation(t, func(serverCfg *ServerConfig) {
					serverCfg.RateLimiter = nil
				})
				return cfg
			},
			wantIs:       ErrForbidden,
			wantContains: "OwnerAwareServerConfig.Local.RateLimiter",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateProductionOwnerAwareConfig(tt.build(t))
			assertProductionValidationResult(t, err, tt.wantIs, tt.wantContains)
		})
	}
}

func TestValidateProductionRemoteOwnerEndpointConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		build        func(t *testing.T) RemoteOwnerEndpointConfig
		wantIs       error
		wantContains string
	}{
		{
			name: "complete",
			build: func(t *testing.T) RemoteOwnerEndpointConfig {
				t.Helper()
				return newProductionTestRemoteOwnerEndpointConfig(t)
			},
		},
		{
			name: "missing local server",
			build: func(t *testing.T) RemoteOwnerEndpointConfig {
				t.Helper()
				cfg := newProductionTestRemoteOwnerEndpointConfig(t)
				cfg.Local = nil
				return cfg
			},
			wantIs: ErrNilRemoteOwnerEndpoint,
		},
		{
			name: "missing remote owner authenticator",
			build: func(t *testing.T) RemoteOwnerEndpointConfig {
				t.Helper()
				cfg := newProductionTestRemoteOwnerEndpointConfig(t)
				cfg.Authenticate = nil
				return cfg
			},
			wantIs:       ErrForbidden,
			wantContains: "RemoteOwnerEndpointConfig.Authenticate",
		},
		{
			name: "invalid local node id",
			build: func(t *testing.T) RemoteOwnerEndpointConfig {
				t.Helper()
				cfg := newProductionTestRemoteOwnerEndpointConfig(t)
				cfg.LocalNodeID = ""
				return cfg
			},
			wantIs: ycluster.ErrInvalidNodeID,
		},
		{
			name: "local server missing redactor",
			build: func(t *testing.T) RemoteOwnerEndpointConfig {
				t.Helper()
				cfg := newProductionTestRemoteOwnerEndpointConfig(t)
				cfg.Local = newProductionTestServerWithMutation(t, func(serverCfg *ServerConfig) {
					serverCfg.Redactor = nil
				})
				return cfg
			},
			wantIs:       ErrForbidden,
			wantContains: "RemoteOwnerEndpointConfig.Local.Redactor",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateProductionRemoteOwnerEndpointConfig(tt.build(t))
			assertProductionValidationResult(t, err, tt.wantIs, tt.wantContains)
		})
	}
}

func newProductionTestServerConfig() ServerConfig {
	return ServerConfig{
		Provider:       yprotocol.NewProvider(yprotocol.ProviderConfig{Store: memory.New()}),
		ResolveRequest: resolveTestRequest,
		Authenticator: AuthenticatorFunc(func(context.Context, *http.Request) (*Principal, error) {
			return &Principal{Subject: "alice", Tenant: "tests"}, nil
		}),
		Authorizer: AuthorizerFunc(func(context.Context, *Principal, Request) error {
			return nil
		}),
		RateLimiter: RateLimiterFunc(func(context.Context, *http.Request, *Principal, Request) error {
			return nil
		}),
		QuotaLimiter: QuotaLimiterFunc(func(context.Context, *http.Request, Request) (QuotaLease, error) {
			return QuotaLeaseFunc(func(context.Context, QuotaDirection, int) error { return nil }), nil
		}),
		OriginPolicy: StaticOriginPolicy{
			AllowedOrigins: []string{"https://app.example.com"},
		},
		Redactor: HashingRequestRedactor{Salt: "test-salt"},
	}
}

func newProductionTestOwnerAwareConfig(t *testing.T) OwnerAwareServerConfig {
	t.Helper()

	return OwnerAwareServerConfig{
		Local:       newProductionTestServerWithMutation(t, nil),
		OwnerLookup: ownerLookupFunc(func(context.Context, ycluster.OwnerLookupRequest) (*ycluster.OwnerResolution, error) { return nil, nil }),
	}
}

func newProductionTestRemoteOwnerEndpointConfig(t *testing.T) RemoteOwnerEndpointConfig {
	t.Helper()

	return RemoteOwnerEndpointConfig{
		Local:        newProductionTestServerWithMutation(t, nil),
		LocalNodeID:  "node-owner",
		Authenticate: RemoteOwnerBearerAuthenticator("node-token"),
	}
}

func newProductionTestServerWithMutation(t *testing.T, mutate func(*ServerConfig)) *Server {
	t.Helper()

	cfg := newProductionTestServerConfig()
	if mutate != nil {
		mutate(&cfg)
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer() unexpected error: %v", err)
	}
	return server
}

func assertProductionValidationResult(t *testing.T, err error, wantIs error, wantContains string) {
	t.Helper()

	if wantIs == nil {
		if err != nil {
			t.Fatalf("validation unexpected error: %v", err)
		}
		return
	}
	if err == nil {
		t.Fatalf("validation error = nil, want %v", wantIs)
	}
	if !errors.Is(err, wantIs) {
		t.Fatalf("errors.Is(%v, %v) = false", err, wantIs)
	}
	if wantContains != "" && !strings.Contains(err.Error(), wantContains) {
		t.Fatalf("error %q does not contain %q", err.Error(), wantContains)
	}
}
