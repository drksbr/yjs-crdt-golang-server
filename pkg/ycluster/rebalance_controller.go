package ycluster

import (
	"context"
	"fmt"
	"time"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
)

// RebalanceDocumentSource lista os documentos que devem entrar no ciclo de
// planejamento de rebalance.
type RebalanceDocumentSource interface {
	RebalanceDocuments(ctx context.Context) ([]storage.DocumentKey, error)
}

// RebalanceDocumentSourceFunc adapta uma função simples como source.
type RebalanceDocumentSourceFunc func(ctx context.Context) ([]storage.DocumentKey, error)

// RebalanceDocuments chama a função adaptada.
func (f RebalanceDocumentSourceFunc) RebalanceDocuments(ctx context.Context) ([]storage.DocumentKey, error) {
	if f == nil {
		return nil, fmt.Errorf("%w: document source nil", ErrInvalidRebalancePlan)
	}
	return f(ctx)
}

// RebalanceControllerConfig configura o control loop de rebalance.
type RebalanceControllerConfig struct {
	Coordinator *StorageOwnershipCoordinator
	Source      RebalanceDocumentSource

	TargetHolders         []NodeID
	TargetSource          RebalanceTargetSource
	Interval              time.Duration
	MaxMoves              int
	PromoteIfOwnerMissing bool
	TTL                   time.Duration
	TokenForDocument      func(PlannedRebalance) string

	OnPlan   func(*RebalancePlan)
	OnResult func(RebalanceControllerRunResult, error)
}

// Validate confirma se o controller possui dependências mínimas.
func (c RebalanceControllerConfig) Validate() error {
	if c.Coordinator == nil {
		return ErrNilOwnershipCoordinator
	}
	if c.Source == nil {
		return fmt.Errorf("%w: document source obrigatorio", ErrInvalidRebalancePlan)
	}
	if c.Interval <= 0 {
		return fmt.Errorf("%w: interval obrigatorio", ErrInvalidRebalancePlan)
	}
	if c.MaxMoves < 0 {
		return fmt.Errorf("%w: maxMoves negativo", ErrInvalidRebalancePlan)
	}
	if c.TargetSource == nil {
		if _, err := normalizeRebalanceTargets(c.TargetHolders); err != nil {
			return err
		}
	}
	return nil
}

// RebalanceController executa planejamento e rebalance de forma periódica.
type RebalanceController struct {
	coordinator           *StorageOwnershipCoordinator
	source                RebalanceDocumentSource
	targetSource          RebalanceTargetSource
	interval              time.Duration
	maxMoves              int
	promoteIfOwnerMissing bool
	ttl                   time.Duration
	tokenForDocument      func(PlannedRebalance) string
	onPlan                func(*RebalancePlan)
	onResult              func(RebalanceControllerRunResult, error)
}

// RebalanceControllerRunResult resume um ciclo do controller.
type RebalanceControllerRunResult struct {
	Documents int
	Targets   []NodeID
	Plan      *RebalancePlan
	Results   []RebalancePlanExecutionResult
}

// NewRebalanceController cria um controller de rebalance.
func NewRebalanceController(cfg RebalanceControllerConfig) (*RebalanceController, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	targetSource := cfg.TargetSource
	if targetSource == nil {
		targets, err := normalizeRebalanceTargets(cfg.TargetHolders)
		if err != nil {
			return nil, err
		}
		targetSource = StaticRebalanceTargetSource(targets)
	}
	return &RebalanceController{
		coordinator:           cfg.Coordinator,
		source:                cfg.Source,
		targetSource:          targetSource,
		interval:              cfg.Interval,
		maxMoves:              cfg.MaxMoves,
		promoteIfOwnerMissing: cfg.PromoteIfOwnerMissing,
		ttl:                   cfg.TTL,
		tokenForDocument:      cfg.TokenForDocument,
		onPlan:                cfg.OnPlan,
		onResult:              cfg.OnResult,
	}, nil
}

// RunOnce executa um ciclo completo: listar documentos, planejar e executar.
func (c *RebalanceController) RunOnce(ctx context.Context) (RebalanceControllerRunResult, error) {
	if c == nil {
		return RebalanceControllerRunResult{}, ErrInvalidRebalancePlan
	}
	if ctx == nil {
		ctx = context.Background()
	}

	documents, err := c.source.RebalanceDocuments(ctx)
	if err != nil {
		result := RebalanceControllerRunResult{}
		c.observeResult(result, err)
		return result, err
	}
	targets, err := c.targetSource.RebalanceTargets(ctx)
	if err != nil {
		result := RebalanceControllerRunResult{Documents: len(documents)}
		c.observeResult(result, err)
		return result, err
	}

	plan, err := c.coordinator.PlanDocumentRebalance(ctx, RebalancePlanRequest{
		Documents:             documents,
		TargetHolders:         targets,
		MaxMoves:              c.maxMoves,
		PromoteIfOwnerMissing: c.promoteIfOwnerMissing,
	})
	if c.onPlan != nil && plan != nil {
		c.onPlan(plan)
	}
	if err != nil {
		result := RebalanceControllerRunResult{Documents: len(documents), Targets: targets, Plan: plan}
		c.observeResult(result, err)
		return result, err
	}

	results, err := c.coordinator.ExecuteRebalancePlan(ctx, plan, RebalancePlanExecutionOptions{
		TTL:              c.ttl,
		TokenForDocument: c.tokenForDocument,
	})
	result := RebalanceControllerRunResult{
		Documents: len(documents),
		Targets:   targets,
		Plan:      plan,
		Results:   results,
	}
	c.observeResult(result, err)
	return result, err
}

// Run executa ciclos periódicos até o contexto ser cancelado ou algum ciclo
// retornar erro.
func (c *RebalanceController) Run(ctx context.Context) error {
	if c == nil {
		return ErrInvalidRebalancePlan
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, err := c.RunOnce(ctx); err != nil {
		return err
	}

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if _, err := c.RunOnce(ctx); err != nil {
				return err
			}
		}
	}
}

func (c *RebalanceController) observeResult(result RebalanceControllerRunResult, err error) {
	if c != nil && c.onResult != nil {
		c.onResult(result, err)
	}
}
