package yhttp

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/ycluster"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yprotocol"
)

// AuthorityRevalidationResult resume uma revalidacao forçada de autoridade.
type AuthorityRevalidationResult struct {
	DocumentKey   storage.DocumentKey
	Checked       int
	AuthorityLost int
}

type authorityRevalidationSession struct {
	req             Request
	connection      *yprotocol.Connection
	cancelSession   context.CancelFunc
	signals         chan<- remoteOwnerCloseSignal
	onAuthorityLoss func(remoteOwnerCloseSignal)
}

type authorityRevalidationRegistry struct {
	mu    sync.RWMutex
	rooms map[storage.DocumentKey]map[string]*authorityRevalidationSession
}

func newAuthorityRevalidationRegistry() authorityRevalidationRegistry {
	return authorityRevalidationRegistry{
		rooms: make(map[storage.DocumentKey]map[string]*authorityRevalidationSession),
	}
}

func (r *authorityRevalidationRegistry) add(key storage.DocumentKey, connectionID string, session *authorityRevalidationSession) {
	if r == nil || session == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	room := r.rooms[key]
	if room == nil {
		room = make(map[string]*authorityRevalidationSession)
		r.rooms[key] = room
	}
	room[connectionID] = session
}

func (r *authorityRevalidationRegistry) remove(key storage.DocumentKey, connectionID string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	room := r.rooms[key]
	if len(room) == 0 {
		return
	}
	delete(room, connectionID)
	if len(room) == 0 {
		delete(r.rooms, key)
	}
}

func (r *authorityRevalidationRegistry) snapshot(key storage.DocumentKey) []*authorityRevalidationSession {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	room := r.rooms[key]
	if len(room) == 0 {
		return nil
	}
	out := make([]*authorityRevalidationSession, 0, len(room))
	for _, session := range room {
		out = append(out, session)
	}
	return out
}

// RevalidateDocumentAuthority força revalidação imediata de todas as conexões
// locais abertas para o documento. Quando a autoridade mudou, as conexões são
// sinalizadas pelo mesmo caminho usado pela revalidação periódica, permitindo
// close retryable ou handoff/rebind transparente quando configurado.
func (s *Server) RevalidateDocumentAuthority(ctx context.Context, key storage.DocumentKey) (AuthorityRevalidationResult, error) {
	if s == nil {
		return AuthorityRevalidationResult{}, ErrNilLocalServer
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := key.Validate(); err != nil {
		return AuthorityRevalidationResult{}, err
	}

	result := AuthorityRevalidationResult{DocumentKey: key}
	sessions := s.authorityRevalidations.snapshot(key)
	var errs []error
	for _, session := range sessions {
		if err := ctx.Err(); err != nil {
			errs = append(errs, err)
			break
		}
		result.Checked++
		err := s.revalidateAuthoritySession(ctx, session)
		switch {
		case err == nil, errors.Is(err, yprotocol.ErrConnectionClosed):
			continue
		case isAuthorityLostRetryableError(err):
			result.AuthorityLost++
		default:
			errs = append(errs, err)
		}
	}
	return result, errors.Join(errs...)
}

func (s *Server) revalidateAuthoritySession(ctx context.Context, session *authorityRevalidationSession) error {
	if session == nil || session.connection == nil {
		return yprotocol.ErrConnectionClosed
	}
	if ctx == nil {
		ctx = context.Background()
	}

	start := time.Now()
	err := session.connection.RevalidateAuthority(ctx)
	duration := time.Since(start)
	switch {
	case err == nil, errors.Is(err, yprotocol.ErrConnectionClosed):
		if err == nil {
			observeAuthorityRevalidation(s.metrics, session.req, authorityRevalidationRoleLocal, duration, nil)
		}
		return err
	case isAuthorityLostRetryableError(err):
		observeAuthorityRevalidation(s.metrics, session.req, authorityRevalidationRoleLocal, duration, err)
		s.metrics.Error(session.req, "revalidate_authority", err)
		s.report(nil, session.req, err)
		signal := remoteOwnerCloseSignalFromError(err, "revalidate_authority")
		signalRemoteOwnerClose(session.signals, signal)
		if session.onAuthorityLoss != nil {
			session.onAuthorityLoss(signal)
		}
		if session.cancelSession != nil {
			session.cancelSession()
		}
		return err
	default:
		observeAuthorityRevalidation(s.metrics, session.req, authorityRevalidationRoleLocal, duration, err)
		s.metrics.Error(session.req, "revalidate_authority", err)
		s.report(nil, session.req, err)
		return err
	}
}

// RebalanceAuthorityRevalidationConfig configura o callback que conecta o
// resultado do `ycluster.RebalanceController` à revalidação imediata da borda.
type RebalanceAuthorityRevalidationConfig struct {
	Server  *Server
	Timeout time.Duration
	OnError func(storage.DocumentKey, error)
}

// NewRebalanceAuthorityRevalidationCallback cria um callback compatível com
// `RebalanceControllerConfig.OnResult`.
func NewRebalanceAuthorityRevalidationCallback(
	cfg RebalanceAuthorityRevalidationConfig,
) (func(ycluster.RebalanceControllerRunResult, error), error) {
	if cfg.Server == nil {
		return nil, ErrNilLocalServer
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = cfg.Server.writeTimeout
	}
	if timeout <= 0 {
		timeout = defaultWriteTimeout
	}

	return func(result ycluster.RebalanceControllerRunResult, _ error) {
		seen := make(map[storage.DocumentKey]struct{}, len(result.Results))
		for _, execution := range result.Results {
			if execution.Err != nil || execution.Result == nil || !execution.Result.Changed {
				continue
			}
			key := execution.Result.DocumentKey
			if err := key.Validate(); err != nil {
				key = execution.Planned.DocumentKey
			}
			if err := key.Validate(); err != nil {
				if cfg.OnError != nil {
					cfg.OnError(key, fmt.Errorf("yhttp: chave de rebalance invalida: %w", err))
				}
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}

			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			_, err := cfg.Server.RevalidateDocumentAuthority(ctx, key)
			cancel()
			if err != nil && cfg.OnError != nil {
				cfg.OnError(key, err)
			}
		}
	}, nil
}
