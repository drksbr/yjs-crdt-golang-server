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
