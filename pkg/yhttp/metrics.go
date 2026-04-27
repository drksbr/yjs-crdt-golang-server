package yhttp

import "time"

// Metrics descreve hooks opcionais de observabilidade da camada HTTP/WebSocket.
//
// A interface fica livre de dependências externas para permitir adapters
// específicos, como Prometheus, sem acoplar o núcleo de transporte a uma
// biblioteca de métricas.
type Metrics interface {
	ConnectionOpened(req Request)
	ConnectionClosed(req Request)
	FrameRead(req Request, bytes int)
	FrameWritten(req Request, kind string, bytes int)
	Handle(req Request, duration time.Duration, err error)
	Persist(req Request, duration time.Duration, err error)
	Error(req Request, stage string, err error)
}

// OwnerLookupMetrics adiciona observabilidade opcional para resolucao de owner.
type OwnerLookupMetrics interface {
	OwnerLookup(req Request, duration time.Duration, result string)
}

// RouteDecisionMetrics adiciona observabilidade opcional para a decisao final
// de roteamento feita pela borda owner-aware.
type RouteDecisionMetrics interface {
	RouteDecision(req Request, decision string)
}

// RemoteOwnerMetrics adiciona observabilidade opcional para o relay edge<->
// owner e para o endpoint owner-side de streams tipados inter-node.
type RemoteOwnerMetrics interface {
	RemoteOwnerConnectionOpened(req Request, role string)
	RemoteOwnerConnectionClosed(req Request, role string)
	RemoteOwnerHandshake(req Request, role string, duration time.Duration, err error)
	RemoteOwnerMessage(req Request, role string, direction string, kind string)
	RemoteOwnerClose(req Request, role string, reason string)
}

// AuthorityRevalidationMetrics adiciona observabilidade opcional para os
// ciclos periodicos de revalidacao de autoridade/lease.
type AuthorityRevalidationMetrics interface {
	AuthorityRevalidation(req Request, role string, duration time.Duration, err error)
}

// OwnershipTransitionMetrics adiciona observabilidade opcional para handoffs e
// rebinds entre modos de ownership local/remoto.
type OwnershipTransitionMetrics interface {
	OwnershipTransition(req Request, from string, to string, duration time.Duration, err error)
}

type noopMetrics struct{}

func (noopMetrics) ConnectionOpened(Request)              {}
func (noopMetrics) ConnectionClosed(Request)              {}
func (noopMetrics) FrameRead(Request, int)                {}
func (noopMetrics) FrameWritten(Request, string, int)     {}
func (noopMetrics) Handle(Request, time.Duration, error)  {}
func (noopMetrics) Persist(Request, time.Duration, error) {}
func (noopMetrics) Error(Request, string, error)          {}

func normalizeMetrics(metrics Metrics) Metrics {
	if metrics == nil {
		return noopMetrics{}
	}
	return metrics
}

func observeOwnerLookup(metrics Metrics, req Request, duration time.Duration, result string) {
	observer, ok := metrics.(OwnerLookupMetrics)
	if !ok {
		return
	}
	observer.OwnerLookup(req, duration, result)
}

func observeRouteDecision(metrics Metrics, req Request, decision string) {
	observer, ok := metrics.(RouteDecisionMetrics)
	if !ok {
		return
	}
	observer.RouteDecision(req, decision)
}

func observeRemoteOwnerConnectionOpened(metrics Metrics, req Request, role string) {
	observer, ok := metrics.(RemoteOwnerMetrics)
	if !ok {
		return
	}
	observer.RemoteOwnerConnectionOpened(req, role)
}

func observeRemoteOwnerConnectionClosed(metrics Metrics, req Request, role string) {
	observer, ok := metrics.(RemoteOwnerMetrics)
	if !ok {
		return
	}
	observer.RemoteOwnerConnectionClosed(req, role)
}

func observeRemoteOwnerHandshake(metrics Metrics, req Request, role string, duration time.Duration, err error) {
	observer, ok := metrics.(RemoteOwnerMetrics)
	if !ok {
		return
	}
	observer.RemoteOwnerHandshake(req, role, duration, err)
}

func observeRemoteOwnerMessage(metrics Metrics, req Request, role string, direction string, kind string) {
	observer, ok := metrics.(RemoteOwnerMetrics)
	if !ok {
		return
	}
	observer.RemoteOwnerMessage(req, role, direction, kind)
}

func observeRemoteOwnerClose(metrics Metrics, req Request, role string, reason string) {
	observer, ok := metrics.(RemoteOwnerMetrics)
	if !ok {
		return
	}
	observer.RemoteOwnerClose(req, role, reason)
}

func observeAuthorityRevalidation(metrics Metrics, req Request, role string, duration time.Duration, err error) {
	observer, ok := metrics.(AuthorityRevalidationMetrics)
	if !ok {
		return
	}
	observer.AuthorityRevalidation(req, role, duration, err)
}

func observeOwnershipTransition(metrics Metrics, req Request, from string, to string, duration time.Duration, err error) {
	observer, ok := metrics.(OwnershipTransitionMetrics)
	if !ok {
		return
	}
	observer.OwnershipTransition(req, from, to, duration, err)
}
