package yhttp

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
)

// QuotaDirection identifies the byte budget being consumed.
type QuotaDirection string

const (
	// QuotaDirectionInbound accounts bytes received from a client.
	QuotaDirectionInbound QuotaDirection = "inbound"
	// QuotaDirectionOutbound accounts bytes written back to a client.
	QuotaDirectionOutbound QuotaDirection = "outbound"
)

// QuotaLimiter reserves and enforces per-connection quota for HTTP/WebSocket
// sessions. Implementations may be local or backed by distributed storage.
type QuotaLimiter interface {
	OpenQuota(ctx context.Context, r *http.Request, req Request) (QuotaLease, error)
}

// QuotaLimiterFunc adapts a function to QuotaLimiter.
type QuotaLimiterFunc func(ctx context.Context, r *http.Request, req Request) (QuotaLease, error)

// OpenQuota reserves quota by calling the wrapped function.
func (f QuotaLimiterFunc) OpenQuota(ctx context.Context, r *http.Request, req Request) (QuotaLease, error) {
	return f(ctx, r, req)
}

// QuotaLease tracks quota consumption for one accepted connection.
type QuotaLease interface {
	AllowFrame(ctx context.Context, direction QuotaDirection, bytes int) error
	Close(ctx context.Context) error
}

// QuotaLeaseFunc adapts a frame-checking function to QuotaLease.
type QuotaLeaseFunc func(ctx context.Context, direction QuotaDirection, bytes int) error

// AllowFrame checks whether the frame is allowed.
func (f QuotaLeaseFunc) AllowFrame(ctx context.Context, direction QuotaDirection, bytes int) error {
	return f(ctx, direction, bytes)
}

// Close releases no resources for a function-backed quota lease.
func (f QuotaLeaseFunc) Close(context.Context) error {
	return nil
}

// LocalQuotaLimiterConfig configures the in-memory quota reference
// implementation. Zero values disable each individual limit.
type LocalQuotaLimiterConfig struct {
	MaxConnections                int
	MaxConnectionsPerTenant       int
	MaxConnectionsPerDocument     int
	MaxInboundFrameBytes          int
	MaxOutboundFrameBytes         int
	MaxInboundBytesPerConnection  int64
	MaxOutboundBytesPerConnection int64
}

// LocalQuotaLimiter is an in-memory reference implementation for quotas.
//
// It is safe for concurrent use by multiple HTTP/WebSocket handlers in a
// single process. It is not a distributed quota system.
type LocalQuotaLimiter struct {
	cfg LocalQuotaLimiterConfig

	mu         sync.Mutex
	total      int
	byTenant   map[string]int
	byDocument map[string]int
}

// NewLocalQuotaLimiter builds an in-memory quota limiter.
func NewLocalQuotaLimiter(cfg LocalQuotaLimiterConfig) (*LocalQuotaLimiter, error) {
	if cfg.MaxConnections < 0 ||
		cfg.MaxConnectionsPerTenant < 0 ||
		cfg.MaxConnectionsPerDocument < 0 ||
		cfg.MaxInboundFrameBytes < 0 ||
		cfg.MaxOutboundFrameBytes < 0 ||
		cfg.MaxInboundBytesPerConnection < 0 ||
		cfg.MaxOutboundBytesPerConnection < 0 {
		return nil, errors.New("yhttp: quotas nao podem ser negativas")
	}
	return &LocalQuotaLimiter{
		cfg:        cfg,
		byTenant:   make(map[string]int),
		byDocument: make(map[string]int),
	}, nil
}

// OpenQuota reserves a connection slot for the request.
func (l *LocalQuotaLimiter) OpenQuota(_ context.Context, _ *http.Request, req Request) (QuotaLease, error) {
	if l == nil {
		return nil, nil
	}
	tenantKey := quotaTenantKey(req)
	documentKey := quotaDocumentKey(req)

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.cfg.MaxConnections > 0 && l.total >= l.cfg.MaxConnections {
		return nil, ErrQuotaExceeded
	}
	if l.cfg.MaxConnectionsPerTenant > 0 && l.byTenant[tenantKey] >= l.cfg.MaxConnectionsPerTenant {
		return nil, ErrQuotaExceeded
	}
	if l.cfg.MaxConnectionsPerDocument > 0 && l.byDocument[documentKey] >= l.cfg.MaxConnectionsPerDocument {
		return nil, ErrQuotaExceeded
	}

	l.total++
	l.byTenant[tenantKey]++
	l.byDocument[documentKey]++
	return &localQuotaLease{
		limiter:     l,
		tenantKey:   tenantKey,
		documentKey: documentKey,
	}, nil
}

func (l *LocalQuotaLimiter) release(tenantKey string, documentKey string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.total > 0 {
		l.total--
	}
	decrementQuotaCounter(l.byTenant, tenantKey)
	decrementQuotaCounter(l.byDocument, documentKey)
}

type localQuotaLease struct {
	limiter     *LocalQuotaLimiter
	tenantKey   string
	documentKey string

	mu       sync.Mutex
	closed   bool
	inbound  int64
	outbound int64
}

// AllowFrame consumes a frame byte budget.
func (l *localQuotaLease) AllowFrame(_ context.Context, direction QuotaDirection, bytes int) error {
	if l == nil || l.limiter == nil {
		return nil
	}
	if bytes < 0 {
		return ErrQuotaExceeded
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return ErrQuotaExceeded
	}

	cfg := l.limiter.cfg
	switch direction {
	case QuotaDirectionInbound:
		if cfg.MaxInboundFrameBytes > 0 && bytes > cfg.MaxInboundFrameBytes {
			return ErrQuotaExceeded
		}
		l.inbound += int64(bytes)
		if cfg.MaxInboundBytesPerConnection > 0 && l.inbound > cfg.MaxInboundBytesPerConnection {
			return ErrQuotaExceeded
		}
	case QuotaDirectionOutbound:
		if cfg.MaxOutboundFrameBytes > 0 && bytes > cfg.MaxOutboundFrameBytes {
			return ErrQuotaExceeded
		}
		l.outbound += int64(bytes)
		if cfg.MaxOutboundBytesPerConnection > 0 && l.outbound > cfg.MaxOutboundBytesPerConnection {
			return ErrQuotaExceeded
		}
	default:
		return ErrQuotaExceeded
	}
	return nil
}

// Close releases the reserved connection slot once.
func (l *localQuotaLease) Close(context.Context) error {
	if l == nil || l.limiter == nil {
		return nil
	}
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return nil
	}
	l.closed = true
	l.mu.Unlock()
	l.limiter.release(l.tenantKey, l.documentKey)
	return nil
}

func quotaTenantKey(req Request) string {
	if req.Principal != nil && strings.TrimSpace(req.Principal.Tenant) != "" {
		return "principal:" + strings.TrimSpace(req.Principal.Tenant)
	}
	if strings.TrimSpace(req.DocumentKey.Namespace) != "" {
		return "namespace:" + strings.TrimSpace(req.DocumentKey.Namespace)
	}
	return "tenant:unknown"
}

func quotaDocumentKey(req Request) string {
	namespace := strings.TrimSpace(req.DocumentKey.Namespace)
	documentID := strings.TrimSpace(req.DocumentKey.DocumentID)
	return namespace + "/" + documentID
}

func decrementQuotaCounter(values map[string]int, key string) {
	count := values[key]
	if count <= 1 {
		delete(values, key)
		return
	}
	values[key] = count - 1
}
