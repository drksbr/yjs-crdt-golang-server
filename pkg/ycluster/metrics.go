package ycluster

import (
	"errors"
	"time"
)

const (
	ownerLookupResultLocal        = "local"
	ownerLookupResultRemote       = "remote"
	ownerLookupResultOwnerMissing = "owner_not_found"
	ownerLookupResultLeaseExpired = "lease_expired"
	ownerLookupResultInvalid      = "invalid"
	ownerLookupResultError        = "error"

	leaseOperationAcquire = "acquire"
	leaseOperationHandoff = "handoff"
	leaseOperationRenew   = "renew"
	leaseOperationRelease = "release"

	leaseManagerActionAcquire   = "acquire"
	leaseManagerActionRenew     = "renew"
	leaseManagerActionReacquire = "reacquire"
	leaseManagerActionNoop      = "noop"
	leaseManagerActionRelease   = "release"

	metricsResultOK            = "ok"
	metricsResultHeld          = "held"
	metricsResultExpired       = "expired"
	metricsResultOwnerNotFound = "owner_not_found"
	metricsResultTokenMismatch = "token_mismatch"
	metricsResultInvalid       = "invalid"
	metricsResultError         = "error"
)

// Metrics descreve hooks opcionais de observabilidade do control plane
// distribuído em `pkg/ycluster`.
type Metrics interface {
	OwnerLookup(duration time.Duration, result string)
	LeaseOperation(shardID ShardID, operation string, duration time.Duration, result string)
	LeaseManagerAction(shardID ShardID, action string, duration time.Duration, result string)
}

type noopMetrics struct{}

func (noopMetrics) OwnerLookup(time.Duration, string) {}

func (noopMetrics) LeaseOperation(ShardID, string, time.Duration, string) {}

func (noopMetrics) LeaseManagerAction(ShardID, string, time.Duration, string) {}

func normalizeMetrics(metrics Metrics) Metrics {
	if metrics == nil {
		return noopMetrics{}
	}
	return metrics
}

func observeOwnerLookup(metrics Metrics, duration time.Duration, result string) {
	if metrics == nil {
		return
	}
	metrics.OwnerLookup(duration, result)
}

func observeLeaseOperation(metrics Metrics, shardID ShardID, operation string, duration time.Duration, result string) {
	if metrics == nil {
		return
	}
	metrics.LeaseOperation(shardID, operation, duration, result)
}

func observeLeaseManagerAction(metrics Metrics, shardID ShardID, action string, duration time.Duration, result string) {
	if metrics == nil {
		return
	}
	metrics.LeaseManagerAction(shardID, action, duration, result)
}

func ownerLookupResultLabel(resolution *OwnerResolution, err error) string {
	if err == nil {
		if resolution != nil && resolution.Local {
			return ownerLookupResultLocal
		}
		return ownerLookupResultRemote
	}
	switch {
	case errors.Is(err, ErrLeaseExpired):
		return ownerLookupResultLeaseExpired
	case errors.Is(err, ErrPlacementNotFound), errors.Is(err, ErrOwnerNotFound):
		return ownerLookupResultOwnerMissing
	case errors.Is(err, ErrInvalidPlacement), errors.Is(err, ErrInvalidLease):
		return ownerLookupResultInvalid
	default:
		return ownerLookupResultError
	}
}

func leaseResultLabel(err error) string {
	if err == nil {
		return metricsResultOK
	}
	switch {
	case errors.Is(err, ErrLeaseHeld):
		return metricsResultHeld
	case errors.Is(err, ErrLeaseExpired):
		return metricsResultExpired
	case errors.Is(err, ErrOwnerNotFound):
		return metricsResultOwnerNotFound
	case errors.Is(err, ErrLeaseTokenMismatch):
		return metricsResultTokenMismatch
	case errors.Is(err, ErrInvalidLeaseRequest), errors.Is(err, ErrInvalidLease):
		return metricsResultInvalid
	default:
		return metricsResultError
	}
}
