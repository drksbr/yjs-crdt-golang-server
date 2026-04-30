package yhttp

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// RateLimitKeyFunc calcula a chave de rate limit para uma request.
type RateLimitKeyFunc func(r *http.Request, principal *Principal, req Request) string

// FixedWindowRateLimiterConfig configura um rate limiter em memória por janela
// fixa. Ele é intencionalmente simples e adequado como referência local; em
// produção distribuída, use uma implementação compartilhada via RateLimiter.
type FixedWindowRateLimiterConfig struct {
	Limit   int
	Window  time.Duration
	KeyFunc RateLimitKeyFunc
	Now     func() time.Time
}

// FixedWindowRateLimiter aplica limite por chave em janelas fixas.
type FixedWindowRateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	keyFunc RateLimitKeyFunc
	now     func() time.Time
	buckets map[string]fixedWindowBucket
}

type fixedWindowBucket struct {
	start time.Time
	count int
}

// NewFixedWindowRateLimiter cria um limiter em memória.
func NewFixedWindowRateLimiter(cfg FixedWindowRateLimiterConfig) (*FixedWindowRateLimiter, error) {
	if cfg.Limit <= 0 {
		return nil, errors.New("yhttp: limite de rate limit deve ser positivo")
	}
	if cfg.Window <= 0 {
		return nil, errors.New("yhttp: janela de rate limit deve ser positiva")
	}
	keyFunc := cfg.KeyFunc
	if keyFunc == nil {
		keyFunc = RateLimitByPrincipalOrRemoteAddr
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &FixedWindowRateLimiter{
		limit:   cfg.Limit,
		window:  cfg.Window,
		keyFunc: keyFunc,
		now:     now,
		buckets: make(map[string]fixedWindowBucket),
	}, nil
}

// AllowHTTP registra a request na janela atual ou retorna ErrRateLimited.
func (l *FixedWindowRateLimiter) AllowHTTP(ctx context.Context, r *http.Request, principal *Principal, req Request) error {
	if l == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	key := strings.TrimSpace(l.keyFunc(r, principal, req))
	if key == "" {
		key = "anonymous"
	}
	now := l.now()

	l.mu.Lock()
	defer l.mu.Unlock()

	bucket := l.buckets[key]
	if bucket.start.IsZero() || now.Sub(bucket.start) >= l.window || now.Before(bucket.start) {
		bucket = fixedWindowBucket{start: now}
	}
	if bucket.count >= l.limit {
		l.buckets[key] = bucket
		return ErrRateLimited
	}
	bucket.count++
	l.buckets[key] = bucket
	return nil
}

// RateLimitByPrincipalOrRemoteAddr usa subject autenticado quando disponível,
// senão o IP remoto.
func RateLimitByPrincipalOrRemoteAddr(r *http.Request, principal *Principal, _ Request) string {
	if principal != nil && strings.TrimSpace(principal.Subject) != "" {
		return "principal:" + strings.TrimSpace(principal.Subject)
	}
	return "remote:" + remoteAddrHost(r)
}

// RateLimitByDocument limita por namespace/documento.
func RateLimitByDocument(_ *http.Request, _ *Principal, req Request) string {
	return "document:" + strings.TrimSpace(req.DocumentKey.Namespace) + "/" + strings.TrimSpace(req.DocumentKey.DocumentID)
}

// RateLimitByTenant limita por tenant autenticado e cai para namespace do documento.
func RateLimitByTenant(_ *http.Request, principal *Principal, req Request) string {
	if principal != nil && strings.TrimSpace(principal.Tenant) != "" {
		return "tenant:" + strings.TrimSpace(principal.Tenant)
	}
	return "tenant:" + strings.TrimSpace(req.DocumentKey.Namespace)
}

func remoteAddrHost(r *http.Request) string {
	if r == nil {
		return "unknown"
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && strings.TrimSpace(host) != "" {
		return host
	}
	if strings.TrimSpace(r.RemoteAddr) != "" {
		return strings.TrimSpace(r.RemoteAddr)
	}
	return "unknown"
}
