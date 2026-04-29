package ycluster

import (
	"context"
	"fmt"
	"sort"
	"time"
)

// NodeHealthState representa a condicao operacional observada de um no.
type NodeHealthState string

const (
	// NodeHealthUnknown representa estado ausente ou ainda nao observado.
	NodeHealthUnknown NodeHealthState = "unknown"
	// NodeHealthReady indica que o no pode receber ownership de documentos.
	NodeHealthReady NodeHealthState = "ready"
	// NodeHealthDraining indica que o no ainda existe, mas nao deve receber
	// novos documentos a menos que a politica permita explicitamente.
	NodeHealthDraining NodeHealthState = "draining"
	// NodeHealthUnhealthy indica que o no nao deve receber ownership.
	NodeHealthUnhealthy NodeHealthState = "unhealthy"
)

// NodeHealth descreve o estado mais recente de um no do cluster.
type NodeHealth struct {
	NodeID   NodeID
	State    NodeHealthState
	LastSeen time.Time
	Labels   map[string]string
}

// Clone retorna uma copia independente do registro de health.
func (h NodeHealth) Clone() NodeHealth {
	cloned := h
	if h.Labels != nil {
		cloned.Labels = make(map[string]string, len(h.Labels))
		for key, value := range h.Labels {
			cloned.Labels[key] = value
		}
	}
	return cloned
}

// Validate confirma se o registro pode alimentar politicas de rebalance.
func (h NodeHealth) Validate() error {
	if err := h.NodeID.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidNodeHealth, err)
	}
	switch normalizeNodeHealthState(h.State) {
	case NodeHealthUnknown, NodeHealthReady, NodeHealthDraining, NodeHealthUnhealthy:
		return nil
	default:
		return fmt.Errorf("%w: state %q", ErrInvalidNodeHealth, h.State)
	}
}

// NodeHealthSource lista o health observado dos nos participantes do cluster.
type NodeHealthSource interface {
	ListNodeHealth(ctx context.Context) ([]NodeHealth, error)
}

// NodeHealthSourceFunc adapta uma funcao simples como fonte de health.
type NodeHealthSourceFunc func(ctx context.Context) ([]NodeHealth, error)

// ListNodeHealth chama a funcao adaptada.
func (f NodeHealthSourceFunc) ListNodeHealth(ctx context.Context) ([]NodeHealth, error) {
	if f == nil {
		return nil, fmt.Errorf("%w: node health source nil", ErrInvalidRebalancePlan)
	}
	return f(ctx)
}

// StaticNodeHealthSource materializa uma fonte imutavel para testes, exemplos e
// ambientes em que o membership ja e injetado por configuracao externa.
type StaticNodeHealthSource struct {
	nodes []NodeHealth
}

// NewStaticNodeHealthSource cria uma fonte imutavel de registros de health.
func NewStaticNodeHealthSource(nodes []NodeHealth) *StaticNodeHealthSource {
	return &StaticNodeHealthSource{nodes: cloneNodeHealthSlice(nodes)}
}

// ListNodeHealth retorna os registros configurados.
func (s *StaticNodeHealthSource) ListNodeHealth(ctx context.Context) ([]NodeHealth, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if s == nil {
		return nil, fmt.Errorf("%w: static node health source nil", ErrInvalidRebalancePlan)
	}
	return cloneNodeHealthSlice(s.nodes), nil
}

// RebalanceTargetSource resolve dinamicamente quais nos podem receber ownership
// em um ciclo de rebalance.
type RebalanceTargetSource interface {
	RebalanceTargets(ctx context.Context) ([]NodeID, error)
}

// RebalanceTargetSourceFunc adapta uma funcao simples como fonte de targets.
type RebalanceTargetSourceFunc func(ctx context.Context) ([]NodeID, error)

// RebalanceTargets chama a funcao adaptada e normaliza os targets retornados.
func (f RebalanceTargetSourceFunc) RebalanceTargets(ctx context.Context) ([]NodeID, error) {
	if f == nil {
		return nil, fmt.Errorf("%w: target source nil", ErrInvalidRebalancePlan)
	}
	targets, err := f(ctx)
	return normalizeRebalanceTargetsFromContext(ctx, targets, err)
}

// StaticRebalanceTargetSource materializa uma lista fixa como fonte de targets.
type StaticRebalanceTargetSource []NodeID

// RebalanceTargets retorna os targets fixos normalizados.
func (s StaticRebalanceTargetSource) RebalanceTargets(ctx context.Context) ([]NodeID, error) {
	return normalizeRebalanceTargetsFromContext(ctx, []NodeID(s), nil)
}

// HealthyRebalanceTargetSourceConfig configura a selecao de targets a partir de
// registros de membership/health.
type HealthyRebalanceTargetSourceConfig struct {
	Source NodeHealthSource

	IncludeDraining bool
	MaxStaleness    time.Duration
	RequiredLabels  map[string]string
	Now             func() time.Time
}

// Validate confirma se a fonte dinamica pode ser criada.
func (c HealthyRebalanceTargetSourceConfig) Validate() error {
	if c.Source == nil {
		return fmt.Errorf("%w: node health source obrigatorio", ErrInvalidRebalancePlan)
	}
	if c.MaxStaleness < 0 {
		return fmt.Errorf("%w: maxStaleness negativo", ErrInvalidRebalancePlan)
	}
	return nil
}

// HealthyRebalanceTargetSource filtra membership por health, staleness e labels.
type HealthyRebalanceTargetSource struct {
	source          NodeHealthSource
	includeDraining bool
	maxStaleness    time.Duration
	requiredLabels  map[string]string
	now             func() time.Time
}

var _ RebalanceTargetSource = (*HealthyRebalanceTargetSource)(nil)

// NewHealthyRebalanceTargetSource cria uma fonte dinamica de targets saudaveis.
func NewHealthyRebalanceTargetSource(cfg HealthyRebalanceTargetSourceConfig) (*HealthyRebalanceTargetSource, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &HealthyRebalanceTargetSource{
		source:          cfg.Source,
		includeDraining: cfg.IncludeDraining,
		maxStaleness:    cfg.MaxStaleness,
		requiredLabels:  cloneStringMap(cfg.RequiredLabels),
		now:             now,
	}, nil
}

// RebalanceTargets retorna os nos aptos a receber documentos no ciclo atual.
func (s *HealthyRebalanceTargetSource) RebalanceTargets(ctx context.Context) ([]NodeID, error) {
	if s == nil {
		return nil, fmt.Errorf("%w: healthy target source nil", ErrInvalidRebalancePlan)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	nodes, err := s.source.ListNodeHealth(ctx)
	if err != nil {
		return nil, err
	}

	now := s.now()
	targets := make([]NodeID, 0, len(nodes))
	seen := make(map[NodeID]struct{}, len(nodes))
	for _, node := range nodes {
		if err := node.Validate(); err != nil {
			return nil, err
		}
		if !s.acceptsState(node.State) || s.isStale(node, now) || !labelsMatch(node.Labels, s.requiredLabels) {
			continue
		}
		if _, ok := seen[node.NodeID]; ok {
			continue
		}
		seen[node.NodeID] = struct{}{}
		targets = append(targets, node.NodeID)
	}
	sort.Slice(targets, func(i, j int) bool {
		return targets[i] < targets[j]
	})
	if len(targets) == 0 {
		return nil, fmt.Errorf("%w: nenhum target saudavel", ErrInvalidRebalancePlan)
	}
	return targets, nil
}

func (s *HealthyRebalanceTargetSource) acceptsState(state NodeHealthState) bool {
	switch normalizeNodeHealthState(state) {
	case NodeHealthReady:
		return true
	case NodeHealthDraining:
		return s.includeDraining
	default:
		return false
	}
}

func (s *HealthyRebalanceTargetSource) isStale(node NodeHealth, now time.Time) bool {
	if s.maxStaleness <= 0 {
		return false
	}
	if node.LastSeen.IsZero() {
		return true
	}
	return now.Sub(node.LastSeen) > s.maxStaleness
}

func normalizeNodeHealthState(state NodeHealthState) NodeHealthState {
	if state == "" {
		return NodeHealthUnknown
	}
	return state
}

func labelsMatch(actual, required map[string]string) bool {
	for key, expected := range required {
		if actual[key] != expected {
			return false
		}
	}
	return true
}

func cloneNodeHealthSlice(nodes []NodeHealth) []NodeHealth {
	if len(nodes) == 0 {
		return nil
	}
	out := make([]NodeHealth, len(nodes))
	for i, node := range nodes {
		out[i] = node.Clone()
	}
	return out
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func normalizeRebalanceTargetsFromContext(ctx context.Context, targets []NodeID, err error) ([]NodeID, error) {
	if err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	return normalizeRebalanceTargets(targets)
}
