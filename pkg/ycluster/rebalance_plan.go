package ycluster

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
)

const (
	// RebalanceReasonMissingOwner indica promoção planejada para documento sem
	// owner ativo.
	RebalanceReasonMissingOwner = "owner_missing"
	// RebalanceReasonOwnerOutsideTargets indica owner atual fora do conjunto de
	// holders aceitos pelo plano.
	RebalanceReasonOwnerOutsideTargets = "owner_outside_targets"
	// RebalanceReasonLoadShedding indica movimento para reduzir desequilíbrio
	// entre holders alvo.
	RebalanceReasonLoadShedding = "load_shedding"
)

// RebalancePlanRequest configura o planejamento determinístico de rebalance
// sobre uma lista conhecida de documentos.
type RebalancePlanRequest struct {
	Documents             []storage.DocumentKey
	TargetHolders         []NodeID
	MaxMoves              int
	PromoteIfOwnerMissing bool
}

// PlannedRebalance descreve um movimento planejado para um documento.
type PlannedRebalance struct {
	DocumentKey storage.DocumentKey
	From        NodeID
	To          NodeID
	Reason      string
}

// SkippedRebalanceDocument descreve um documento ignorado pelo planejamento.
type SkippedRebalanceDocument struct {
	DocumentKey storage.DocumentKey
	Reason      string
	Err         error
}

// RebalancePlan contém os movimentos e skips calculados por uma política.
type RebalancePlan struct {
	Moves   []PlannedRebalance
	Skipped []SkippedRebalanceDocument
}

// RebalancePlanExecutionOptions controla a execução sequencial de um plano.
type RebalancePlanExecutionOptions struct {
	TTL time.Duration

	// TokenForDocument permite gerar fencing token específico por movimento.
	// Quando nil, RebalanceDocument deixa o store gerar um token.
	TokenForDocument func(PlannedRebalance) string
}

// RebalancePlanExecutionResult materializa o resultado de um movimento.
type RebalancePlanExecutionResult struct {
	Planned PlannedRebalance
	Result  *RebalanceDocumentResult
	Err     error
}

// PlanDocumentRebalance calcula movimentos document-level para distribuir os
// documentos informados entre os target holders.
func (c *StorageOwnershipCoordinator) PlanDocumentRebalance(ctx context.Context, req RebalancePlanRequest) (*RebalancePlan, error) {
	if c == nil {
		return nil, ErrNilOwnershipCoordinator
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if req.MaxMoves < 0 {
		return nil, fmt.Errorf("%w: maxMoves negativo", ErrInvalidRebalancePlan)
	}

	targets, err := normalizeRebalanceTargets(req.TargetHolders)
	if err != nil {
		return nil, err
	}
	targetSet := make(map[NodeID]struct{}, len(targets))
	loads := make(map[NodeID]int, len(targets))
	docsByOwner := make(map[NodeID][]storage.DocumentKey, len(targets))
	for _, target := range targets {
		targetSet[target] = struct{}{}
		loads[target] = 0
	}

	plan := &RebalancePlan{}
	moved := make(map[storage.DocumentKey]struct{})
	for _, key := range req.Documents {
		if err := key.Validate(); err != nil {
			plan.Skipped = append(plan.Skipped, SkippedRebalanceDocument{
				DocumentKey: key,
				Reason:      "invalid_document",
				Err:         err,
			})
			continue
		}

		resolution, lookupErr := c.LookupOwner(ctx, OwnerLookupRequest{DocumentKey: key})
		if lookupErr != nil {
			if canPromoteAfterOwnerLookup(lookupErr) && req.PromoteIfOwnerMissing && !rebalanceMoveLimitReached(req.MaxMoves, len(plan.Moves)) {
				target := leastLoadedTarget(targets, loads)
				loads[target]++
				plan.Moves = append(plan.Moves, PlannedRebalance{
					DocumentKey: key,
					To:          target,
					Reason:      RebalanceReasonMissingOwner,
				})
				moved[key] = struct{}{}
				continue
			}
			plan.Skipped = append(plan.Skipped, SkippedRebalanceDocument{
				DocumentKey: key,
				Reason:      "owner_lookup_failed",
				Err:         normalizeRebalanceOwnerLookupError(lookupErr),
			})
			continue
		}
		if resolution == nil {
			plan.Skipped = append(plan.Skipped, SkippedRebalanceDocument{
				DocumentKey: key,
				Reason:      "owner_lookup_failed",
				Err:         ErrOwnerNotFound,
			})
			continue
		}

		owner := resolution.Placement.NodeID
		if _, ok := targetSet[owner]; !ok {
			if rebalanceMoveLimitReached(req.MaxMoves, len(plan.Moves)) {
				plan.Skipped = append(plan.Skipped, SkippedRebalanceDocument{
					DocumentKey: key,
					Reason:      "max_moves_reached",
				})
				continue
			}
			target := leastLoadedTarget(targets, loads)
			loads[target]++
			plan.Moves = append(plan.Moves, PlannedRebalance{
				DocumentKey: key,
				From:        owner,
				To:          target,
				Reason:      RebalanceReasonOwnerOutsideTargets,
			})
			moved[key] = struct{}{}
			continue
		}

		loads[owner]++
		docsByOwner[owner] = append(docsByOwner[owner], key)
	}

	for !rebalanceMoveLimitReached(req.MaxMoves, len(plan.Moves)) {
		from := mostLoadedTarget(targets, loads)
		to := leastLoadedTarget(targets, loads)
		if from == "" || to == "" || from == to || loads[from]-loads[to] <= 1 {
			break
		}

		key, ok := nextMovableDocument(docsByOwner[from], moved)
		if !ok {
			break
		}
		moved[key] = struct{}{}
		loads[from]--
		loads[to]++
		plan.Moves = append(plan.Moves, PlannedRebalance{
			DocumentKey: key,
			From:        from,
			To:          to,
			Reason:      RebalanceReasonLoadShedding,
		})
	}

	return plan, nil
}

// ExecuteRebalancePlan executa sequencialmente os movimentos calculados por um
// plano usando RebalanceDocument.
func (c *StorageOwnershipCoordinator) ExecuteRebalancePlan(
	ctx context.Context,
	plan *RebalancePlan,
	opts RebalancePlanExecutionOptions,
) ([]RebalancePlanExecutionResult, error) {
	if c == nil {
		return nil, ErrNilOwnershipCoordinator
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if plan == nil {
		return nil, fmt.Errorf("%w: plano nil", ErrInvalidRebalancePlan)
	}

	results := make([]RebalancePlanExecutionResult, 0, len(plan.Moves))
	var errs []error
	for _, move := range plan.Moves {
		token := ""
		if opts.TokenForDocument != nil {
			token = opts.TokenForDocument(move)
		}
		result, err := c.RebalanceDocument(ctx, RebalanceDocumentRequest{
			DocumentKey:           move.DocumentKey,
			TargetHolder:          move.To,
			TTL:                   opts.TTL,
			Token:                 token,
			PromoteIfOwnerMissing: move.From == "",
		})
		results = append(results, RebalancePlanExecutionResult{
			Planned: move,
			Result:  result,
			Err:     err,
		})
		if err != nil {
			errs = append(errs, fmt.Errorf("rebalance %s/%s to %q: %w", move.DocumentKey.Namespace, move.DocumentKey.DocumentID, move.To, err))
		}
	}
	return results, errors.Join(errs...)
}

func normalizeRebalanceTargets(targets []NodeID) ([]NodeID, error) {
	if len(targets) == 0 {
		return nil, fmt.Errorf("%w: targetHolders obrigatorio", ErrInvalidRebalancePlan)
	}
	seen := make(map[NodeID]struct{}, len(targets))
	out := make([]NodeID, 0, len(targets))
	for _, target := range targets {
		if err := target.Validate(); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidRebalancePlan, err)
		}
		if _, ok := seen[target]; ok {
			continue
		}
		seen[target] = struct{}{}
		out = append(out, target)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i] < out[j]
	})
	return out, nil
}

func rebalanceMoveLimitReached(maxMoves, current int) bool {
	return maxMoves > 0 && current >= maxMoves
}

func leastLoadedTarget(targets []NodeID, loads map[NodeID]int) NodeID {
	var selected NodeID
	for i, target := range targets {
		if i == 0 || loads[target] < loads[selected] {
			selected = target
		}
	}
	return selected
}

func mostLoadedTarget(targets []NodeID, loads map[NodeID]int) NodeID {
	var selected NodeID
	for i, target := range targets {
		if i == 0 || loads[target] > loads[selected] {
			selected = target
		}
	}
	return selected
}

func nextMovableDocument(keys []storage.DocumentKey, moved map[storage.DocumentKey]struct{}) (storage.DocumentKey, bool) {
	for _, key := range keys {
		if _, ok := moved[key]; !ok {
			return key, true
		}
	}
	return storage.DocumentKey{}, false
}
