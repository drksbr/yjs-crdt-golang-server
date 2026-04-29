package ycluster

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
)

// DocumentOwnershipRuntimeConfig configura um runtime ref-counted de ownership
// por documento.
type DocumentOwnershipRuntimeConfig struct {
	Coordinator    *StorageOwnershipCoordinator
	Lease          LeaseManagerRunConfig
	ReleaseTimeout time.Duration
}

// Validate confirma se a configuração do runtime está completa.
func (c DocumentOwnershipRuntimeConfig) Validate() error {
	if c.Coordinator == nil {
		return ErrNilOwnershipCoordinator
	}
	if c.ReleaseTimeout < 0 {
		return fmt.Errorf("%w: releaseTimeout invalido", ErrInvalidLeaseRequest)
	}
	return nil
}

// DocumentOwnershipRuntime mantém uma única execução de ownership por documento
// e compartilha essa execução entre múltiplos callers locais.
type DocumentOwnershipRuntime struct {
	coordinator    *StorageOwnershipCoordinator
	lease          LeaseManagerRunConfig
	releaseTimeout time.Duration

	mu      sync.Mutex
	closed  bool
	entries map[storage.DocumentKey]*documentOwnershipRuntimeEntry
}

type documentOwnershipRuntimeEntry struct {
	key       storage.DocumentKey
	cancel    context.CancelFunc
	ready     chan struct{}
	done      chan struct{}
	refs      int
	ownership *DocumentOwnership
	err       error
}

// NewDocumentOwnershipRuntime constrói um runtime local de ownership por
// documento em cima de `StorageOwnershipCoordinator`.
func NewDocumentOwnershipRuntime(cfg DocumentOwnershipRuntimeConfig) (*DocumentOwnershipRuntime, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &DocumentOwnershipRuntime{
		coordinator:    cfg.Coordinator,
		lease:          cfg.Lease,
		releaseTimeout: cfg.ReleaseTimeout,
		entries:        make(map[storage.DocumentKey]*documentOwnershipRuntimeEntry),
	}, nil
}

// DocumentOwnershipHandle representa uma referência local a uma execução ativa
// de ownership de documento.
type DocumentOwnershipHandle struct {
	runtime *DocumentOwnershipRuntime
	entry   *documentOwnershipRuntimeEntry
	once    sync.Once
	err     error
}

// Ownership retorna a visão atual conhecida do ownership. A lease pode avançar
// conforme o loop de renovação roda.
func (h *DocumentOwnershipHandle) Ownership() *DocumentOwnership {
	if h == nil || h.runtime == nil || h.entry == nil {
		return nil
	}
	return h.runtime.entryOwnership(h.entry)
}

// Done é fechado quando a execução de ownership associada termina.
func (h *DocumentOwnershipHandle) Done() <-chan struct{} {
	if h == nil || h.entry == nil {
		done := make(chan struct{})
		close(done)
		return done
	}
	return h.entry.done
}

// Err retorna o erro final da execução, quando ela já terminou.
func (h *DocumentOwnershipHandle) Err() error {
	if h == nil || h.runtime == nil || h.entry == nil {
		return nil
	}
	return h.runtime.entryErr(h.entry)
}

// Release libera esta referência local. Quando é a última referência do
// documento, o runtime cancela o loop, aguarda o release controlado da lease e
// retorna o erro final.
func (h *DocumentOwnershipHandle) Release(ctx context.Context) error {
	if h == nil || h.runtime == nil || h.entry == nil {
		return nil
	}
	h.once.Do(func() {
		h.err = h.runtime.releaseEntry(ctx, h.entry, true)
	})
	return h.err
}

// AcquireDocumentOwnership garante ownership ativo para o documento e retorna
// uma referência compartilhada. Callers devem invocar Release ao encerrar o uso.
func (r *DocumentOwnershipRuntime) AcquireDocumentOwnership(ctx context.Context, req ClaimDocumentRequest) (*DocumentOwnershipHandle, error) {
	if r == nil || r.coordinator == nil {
		return nil, ErrNilOwnershipCoordinator
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := req.DocumentKey.Validate(); err != nil {
		return nil, err
	}

	runCtx, cancel := context.WithCancel(context.Background())
	entry, created, err := r.acquireEntry(req, cancel)
	if err != nil {
		cancel()
		return nil, err
	}
	if created {
		go r.runEntry(runCtx, entry, req)
	} else {
		cancel()
	}

	handle, err := r.waitEntryReady(ctx, entry)
	if err != nil {
		r.releaseEntry(context.Background(), entry, false)
		return nil, err
	}
	return handle, nil
}

// Close cancela todas as execuções ativas e impede novos acquires.
func (r *DocumentOwnershipRuntime) Close(ctx context.Context) error {
	if r == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil
	}
	r.closed = true
	entries := make([]*documentOwnershipRuntimeEntry, 0, len(r.entries))
	for _, entry := range r.entries {
		entry.refs = 0
		if entry.cancel != nil {
			entry.cancel()
		}
		entries = append(entries, entry)
	}
	r.mu.Unlock()

	var joined error
	for _, entry := range entries {
		if err := waitEntryDone(ctx, entry); err != nil {
			joined = errors.Join(joined, err)
		}
	}
	return joined
}

func (r *DocumentOwnershipRuntime) acquireEntry(req ClaimDocumentRequest, cancel context.CancelFunc) (*documentOwnershipRuntimeEntry, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil, false, ErrOwnershipRuntimeClosed
	}
	if entry := r.entries[req.DocumentKey]; entry != nil {
		entry.refs++
		return entry, false, nil
	}

	entry := &documentOwnershipRuntimeEntry{
		cancel: cancel,
		key:    req.DocumentKey,
		ready:  make(chan struct{}),
		done:   make(chan struct{}),
		refs:   1,
	}
	r.entries[req.DocumentKey] = entry
	return entry, true, nil
}

func (r *DocumentOwnershipRuntime) runEntry(ctx context.Context, entry *documentOwnershipRuntimeEntry, req ClaimDocumentRequest) {
	leaseCfg := r.lease
	onLeaseChange := leaseCfg.OnLeaseChange
	leaseCfg.OnLeaseChange = func(lease *Lease) {
		r.updateEntryLease(entry, lease)
		if onLeaseChange != nil {
			onLeaseChange(lease.Clone())
		}
	}

	err := r.coordinator.RunDocumentOwnership(ctx, DocumentOwnershipRunConfig{
		Claim:          req,
		Lease:          leaseCfg,
		ReleaseOnStop:  true,
		ReleaseTimeout: r.releaseTimeout,
		OnClaimed: func(ownership *DocumentOwnership) {
			r.markEntryReady(entry, ownership, nil)
		},
	})
	r.finishEntry(entry, err)
}

func (r *DocumentOwnershipRuntime) waitEntryReady(ctx context.Context, entry *documentOwnershipRuntimeEntry) (*DocumentOwnershipHandle, error) {
	select {
	case <-entry.ready:
		if err := r.entryErrBeforeReady(entry); err != nil {
			return nil, err
		}
		return &DocumentOwnershipHandle{runtime: r, entry: entry}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (r *DocumentOwnershipRuntime) markEntryReady(entry *documentOwnershipRuntimeEntry, ownership *DocumentOwnership, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	select {
	case <-entry.ready:
		return
	default:
		entry.ownership = ownership.Clone()
		entry.err = err
		close(entry.ready)
	}
}

func (r *DocumentOwnershipRuntime) updateEntryLease(entry *documentOwnershipRuntimeEntry, lease *Lease) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if entry.ownership == nil {
		return
	}
	updated := entry.ownership.Clone()
	updated.Lease = lease.Clone()
	entry.ownership = updated
}

func (r *DocumentOwnershipRuntime) finishEntry(entry *documentOwnershipRuntimeEntry, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	select {
	case <-entry.ready:
	default:
		entry.err = err
		close(entry.ready)
	}
	entry.err = err
	if current := r.entries[entry.key]; current == entry {
		delete(r.entries, entry.key)
	}
	close(entry.done)
}

func (r *DocumentOwnershipRuntime) releaseEntry(ctx context.Context, entry *documentOwnershipRuntimeEntry, wait bool) error {
	if ctx == nil {
		ctx = context.Background()
	}

	r.mu.Lock()
	if entry.refs > 0 {
		entry.refs--
	}
	last := entry.refs == 0
	if last && entry.cancel != nil {
		entry.cancel()
	}
	r.mu.Unlock()

	if !last || !wait {
		return nil
	}
	if err := waitEntryDone(ctx, entry); err != nil {
		return err
	}
	err := r.entryErr(entry)
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func (r *DocumentOwnershipRuntime) entryOwnership(entry *documentOwnershipRuntimeEntry) *DocumentOwnership {
	r.mu.Lock()
	defer r.mu.Unlock()
	return entry.ownership.Clone()
}

func (r *DocumentOwnershipRuntime) entryErr(entry *documentOwnershipRuntimeEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return entry.err
}

func (r *DocumentOwnershipRuntime) entryErrBeforeReady(entry *documentOwnershipRuntimeEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if entry.ownership == nil {
		return entry.err
	}
	return nil
}

func waitEntryDone(ctx context.Context, entry *documentOwnershipRuntimeEntry) error {
	select {
	case <-entry.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
