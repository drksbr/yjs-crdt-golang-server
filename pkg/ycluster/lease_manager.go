package ycluster

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// LeaseManagerConfig define o wiring mínimo para coordenar a lease local de um
// shard sem scripts externos no caminho quente de renew/reacquire.
type LeaseManagerConfig struct {
	Store   LeaseStore
	ShardID ShardID
	Holder  NodeID
	TTL     time.Duration
	Token   string
	Metrics Metrics
}

// LeaseManagerRunConfig configura o loop bloqueante de renovação automática de
// lease.
type LeaseManagerRunConfig struct {
	// RenewWithin define a janela antes da expiração em que a lease deve ser
	// renovada.
	RenewWithin time.Duration

	// Interval define a frequência de checagem do loop.
	Interval time.Duration

	// OnLeaseChange é chamado após acquire, renew ou reacquire bem-sucedido.
	// A callback roda no mesmo goroutine do loop e deve retornar rapidamente.
	OnLeaseChange func(*Lease)
}

// Validate confirma se a configuração contém os campos obrigatórios.
func (c LeaseManagerConfig) Validate() error {
	if c.Store == nil {
		return ErrNilLeaseStore
	}
	if err := c.Holder.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidLeaseRequest, err)
	}
	if c.TTL <= 0 {
		return fmt.Errorf("%w: ttl obrigatorio", ErrInvalidLeaseRequest)
	}
	return nil
}

// LeaseManager coordena a lease local de um único shard, mantendo um cache do
// último lease/token conhecido para renew e reacquire.
type LeaseManager struct {
	store   LeaseStore
	shardID ShardID
	holder  NodeID
	ttl     time.Duration
	token   string
	metrics Metrics
	now     func() time.Time

	mu      sync.Mutex
	current *Lease
}

// NewLeaseManager constrói um coordenador local de renew/reacquire para um
// único shard.
func NewLeaseManager(cfg LeaseManagerConfig) (*LeaseManager, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &LeaseManager{
		store:   cfg.Store,
		shardID: cfg.ShardID,
		holder:  cfg.Holder,
		ttl:     cfg.TTL,
		token:   cfg.Token,
		metrics: normalizeMetrics(cfg.Metrics),
		now: func() time.Time {
			return time.Now().UTC()
		},
	}, nil
}

// Current retorna a lease atualmente cacheada pelo manager.
func (m *LeaseManager) Current() *Lease {
	if m == nil {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	return m.current.Clone()
}

// Acquire tenta adquirir a lease do shard para o holder configurado e atualiza
// o cache local quando bem-sucedido.
func (m *LeaseManager) Acquire(ctx context.Context) (*Lease, error) {
	lease, _, err := m.acquireAction(ctx, leaseManagerActionAcquire)
	return lease, err
}

// Run executa um loop bloqueante de acquire/renew/reacquire até o contexto ser
// cancelado ou uma perda de ownership ocorrer.
func (m *LeaseManager) Run(ctx context.Context, cfg LeaseManagerRunConfig) error {
	if m == nil || m.store == nil {
		return ErrNilLeaseStore
	}
	if ctx == nil {
		ctx = context.Background()
	}
	cfg, err := m.normalizeRunConfig(cfg)
	if err != nil {
		return err
	}

	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		lease, changed, err := m.Ensure(ctx, cfg.RenewWithin)
		if err != nil {
			return err
		}
		if changed && cfg.OnLeaseChange != nil {
			cfg.OnLeaseChange(lease.Clone())
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (m *LeaseManager) acquireAction(ctx context.Context, action string) (lease *Lease, changed bool, err error) {
	start := time.Now()
	metrics := Metrics(nil)
	shardID := ShardID(0)
	if m != nil {
		metrics = m.metrics
		shardID = m.shardID
	}
	defer func() {
		observeLeaseManagerAction(metrics, shardID, action, time.Since(start), leaseResultLabel(err))
	}()
	if m == nil || m.store == nil {
		return nil, false, ErrNilLeaseStore
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if adopted, ok, err := m.tryAdoptActiveLease(ctx); err != nil {
		return nil, false, err
	} else if ok {
		return adopted.Clone(), false, nil
	}

	lease, err = m.store.AcquireLease(ctx, LeaseRequest{
		ShardID: m.shardID,
		Holder:  m.holder,
		TTL:     m.ttl,
		Token:   m.token,
	})
	if err != nil {
		return nil, false, err
	}
	m.setCurrent(lease)
	return lease.Clone(), true, nil
}

type leaseLoader interface {
	LoadLease(ctx context.Context, shardID ShardID) (*Lease, error)
}

func (m *LeaseManager) tryAdoptActiveLease(ctx context.Context) (*Lease, bool, error) {
	loader, ok := m.store.(leaseLoader)
	if !ok {
		return nil, false, nil
	}
	lease, err := loader.LoadLease(ctx, m.shardID)
	if err != nil {
		if errors.Is(err, ErrOwnerNotFound) || errors.Is(err, ErrLeaseExpired) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if lease == nil || lease.ExpiredAt(m.nowTime()) {
		return nil, false, nil
	}
	if lease.Holder != m.holder {
		return nil, false, fmt.Errorf("%w: shard %s esta leased para %q", ErrLeaseHeld, m.shardID, lease.Holder)
	}
	if m.token != "" && lease.Token != m.token {
		return nil, false, ErrLeaseTokenMismatch
	}
	m.setCurrent(lease)
	return lease.Clone(), true, nil
}

// Ensure garante uma lease ativa local.
//
// Quando não há lease cacheada, ou quando a lease atual já expirou, o manager
// tenta um novo acquire.
//
// Quando a lease atual ainda está ativa mas vence em até `renewWithin`, o
// manager tenta renová-la mantendo o mesmo epoch/token.
//
// O retorno `changed` informa se houve acquire/reacquire/renew nesta chamada.
func (m *LeaseManager) Ensure(ctx context.Context, renewWithin time.Duration) (lease *Lease, changed bool, err error) {
	if m == nil || m.store == nil {
		return nil, false, ErrNilLeaseStore
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if renewWithin < 0 {
		return nil, false, fmt.Errorf("%w: renewWithin invalido", ErrInvalidLeaseRequest)
	}

	current := m.Current()
	now := m.nowTime()
	if current == nil {
		lease, changed, err = m.acquireAction(ctx, leaseManagerActionAcquire)
		if err != nil {
			return nil, false, err
		}
		return lease, changed, nil
	}

	if current.ExpiredAt(now) {
		m.clearCurrent()
		lease, changed, err = m.acquireAction(ctx, leaseManagerActionReacquire)
		if err != nil {
			return nil, false, err
		}
		return lease, changed, nil
	}

	if renewWithin == 0 || current.ExpiresAt.Sub(now) > renewWithin {
		observeLeaseManagerAction(m.metrics, m.shardID, leaseManagerActionNoop, 0, metricsResultOK)
		return current.Clone(), false, nil
	}

	start := time.Now()
	lease, err = m.store.RenewLease(ctx, LeaseRequest{
		ShardID: m.shardID,
		Holder:  m.holder,
		TTL:     m.ttl,
		Token:   current.Token,
	})
	switch {
	case err == nil:
		m.setCurrent(lease)
		observeLeaseManagerAction(m.metrics, m.shardID, leaseManagerActionRenew, time.Since(start), metricsResultOK)
		return lease.Clone(), true, nil
	case errors.Is(err, ErrLeaseExpired), errors.Is(err, ErrOwnerNotFound):
		observeLeaseManagerAction(m.metrics, m.shardID, leaseManagerActionRenew, time.Since(start), leaseResultLabel(err))
		m.clearCurrent()
		lease, changed, err = m.acquireAction(ctx, leaseManagerActionReacquire)
		if err != nil {
			return nil, false, err
		}
		return lease, changed, nil
	case errors.Is(err, ErrLeaseTokenMismatch), errors.Is(err, ErrLeaseHeld):
		observeLeaseManagerAction(m.metrics, m.shardID, leaseManagerActionRenew, time.Since(start), leaseResultLabel(err))
		m.clearCurrent()
		return nil, false, err
	default:
		observeLeaseManagerAction(m.metrics, m.shardID, leaseManagerActionRenew, time.Since(start), leaseResultLabel(err))
		return nil, false, err
	}
}

// Release libera explicitamente a lease atualmente cacheada e limpa o estado
// local quando bem-sucedido.
func (m *LeaseManager) Release(ctx context.Context) (err error) {
	start := time.Now()
	metrics := Metrics(nil)
	shardID := ShardID(0)
	if m != nil {
		metrics = m.metrics
		shardID = m.shardID
	}
	defer func() {
		observeLeaseManagerAction(metrics, shardID, leaseManagerActionRelease, time.Since(start), leaseResultLabel(err))
	}()
	if m == nil || m.store == nil {
		return ErrNilLeaseStore
	}
	if ctx == nil {
		ctx = context.Background()
	}

	current := m.Current()
	if current == nil {
		return ErrOwnerNotFound
	}
	if err := m.store.ReleaseLease(ctx, *current); err != nil {
		return err
	}
	m.clearCurrent()
	return nil
}

func (m *LeaseManager) setCurrent(lease *Lease) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.current = lease.Clone()
}

func (m *LeaseManager) clearCurrent() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.current = nil
}

func (m *LeaseManager) nowTime() time.Time {
	if m == nil || m.now == nil {
		return time.Now().UTC()
	}
	return m.now()
}

func (m *LeaseManager) normalizeRunConfig(cfg LeaseManagerRunConfig) (LeaseManagerRunConfig, error) {
	if cfg.RenewWithin < 0 {
		return LeaseManagerRunConfig{}, fmt.Errorf("%w: renewWithin invalido", ErrInvalidLeaseRequest)
	}
	if cfg.Interval < 0 {
		return LeaseManagerRunConfig{}, fmt.Errorf("%w: interval invalido", ErrInvalidLeaseRequest)
	}
	if cfg.RenewWithin == 0 {
		cfg.RenewWithin = m.ttl / 3
		if cfg.RenewWithin <= 0 {
			cfg.RenewWithin = m.ttl
		}
	}
	if cfg.Interval == 0 {
		cfg.Interval = cfg.RenewWithin / 2
		if cfg.Interval <= 0 {
			cfg.Interval = m.ttl / 4
		}
		if cfg.Interval <= 0 {
			cfg.Interval = time.Second
		}
	}
	return cfg, nil
}
