package ycluster

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestHealthyRebalanceTargetSourceFiltersMembership(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
	source := NewStaticNodeHealthSource([]NodeHealth{
		{NodeID: "node-b", State: NodeHealthReady, LastSeen: now.Add(-10 * time.Second), Labels: map[string]string{"region": "eu"}},
		{NodeID: "node-a", State: NodeHealthReady, LastSeen: now.Add(-5 * time.Second), Labels: map[string]string{"region": "eu"}},
		{NodeID: "node-c", State: NodeHealthDraining, LastSeen: now.Add(-5 * time.Second), Labels: map[string]string{"region": "eu"}},
		{NodeID: "node-d", State: NodeHealthReady, LastSeen: now.Add(-time.Minute), Labels: map[string]string{"region": "eu"}},
		{NodeID: "node-e", State: NodeHealthUnhealthy, LastSeen: now.Add(-5 * time.Second), Labels: map[string]string{"region": "eu"}},
		{NodeID: "node-f", State: NodeHealthReady, LastSeen: now.Add(-5 * time.Second), Labels: map[string]string{"region": "us"}},
		{NodeID: "node-a", State: NodeHealthReady, LastSeen: now.Add(-4 * time.Second), Labels: map[string]string{"region": "eu"}},
	})

	tests := []struct {
		name            string
		includeDraining bool
		want            []NodeID
	}{
		{name: "ready only", want: []NodeID{"node-a", "node-b"}},
		{name: "with draining", includeDraining: true, want: []NodeID{"node-a", "node-b", "node-c"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			targets, err := NewHealthyRebalanceTargetSource(HealthyRebalanceTargetSourceConfig{
				Source:          source,
				IncludeDraining: tc.includeDraining,
				MaxStaleness:    30 * time.Second,
				RequiredLabels:  map[string]string{"region": "eu"},
				Now:             func() time.Time { return now },
			})
			if err != nil {
				t.Fatalf("NewHealthyRebalanceTargetSource() unexpected error: %v", err)
			}

			got, err := targets.RebalanceTargets(context.Background())
			if err != nil {
				t.Fatalf("RebalanceTargets() unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("RebalanceTargets() = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestHealthyRebalanceTargetSourceValidation(t *testing.T) {
	t.Parallel()

	_, err := NewHealthyRebalanceTargetSource(HealthyRebalanceTargetSourceConfig{
		Source:       NewStaticNodeHealthSource(nil),
		MaxStaleness: -time.Second,
	})
	if !errors.Is(err, ErrInvalidRebalancePlan) {
		t.Fatalf("NewHealthyRebalanceTargetSource(negative staleness) error = %v, want %v", err, ErrInvalidRebalancePlan)
	}

	targets, err := NewHealthyRebalanceTargetSource(HealthyRebalanceTargetSourceConfig{
		Source: NewStaticNodeHealthSource([]NodeHealth{{NodeID: "node-a", State: "degraded"}}),
	})
	if err != nil {
		t.Fatalf("NewHealthyRebalanceTargetSource(invalid state source) unexpected error: %v", err)
	}
	_, err = targets.RebalanceTargets(context.Background())
	if !errors.Is(err, ErrInvalidNodeHealth) {
		t.Fatalf("RebalanceTargets(invalid state) error = %v, want %v", err, ErrInvalidNodeHealth)
	}

	targets, err = NewHealthyRebalanceTargetSource(HealthyRebalanceTargetSourceConfig{
		Source: NewStaticNodeHealthSource([]NodeHealth{{NodeID: "node-a", State: NodeHealthUnhealthy}}),
	})
	if err != nil {
		t.Fatalf("NewHealthyRebalanceTargetSource(unhealthy) unexpected error: %v", err)
	}
	_, err = targets.RebalanceTargets(context.Background())
	if !errors.Is(err, ErrInvalidRebalancePlan) {
		t.Fatalf("RebalanceTargets(no healthy nodes) error = %v, want %v", err, ErrInvalidRebalancePlan)
	}
}
