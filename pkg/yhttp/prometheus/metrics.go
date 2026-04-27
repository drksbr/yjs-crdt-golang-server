package yhttpprometheus

import (
	"fmt"
	"strings"
	"time"

	prometheuslib "github.com/prometheus/client_golang/prometheus"

	"yjs-go-bridge/pkg/yhttp"
)

const (
	defaultNamespace = "yjsbridge"
	defaultSubsystem = "yhttp"
)

// Config define o namespace e o registro Prometheus usados pelo adapter.
type Config struct {
	Namespace                   string
	Subsystem                   string
	Registerer                  prometheuslib.Registerer
	HandleDurationBuckets       []float64
	PersistDurationBucket       []float64
	OwnerLookupDurationBuckets  []float64
	RemoteOwnerHandshakeBuckets []float64
}

// Metrics implementa `yhttp.Metrics` com coletores Prometheus.
type Metrics struct {
	connectionsOpened         prometheuslib.Counter
	connectionsActive         prometheuslib.Gauge
	framesRead                prometheuslib.Counter
	framesWritten             *prometheuslib.CounterVec
	bytesRead                 prometheuslib.Counter
	bytesWritten              *prometheuslib.CounterVec
	handleDuration            *prometheuslib.HistogramVec
	persistDuration           *prometheuslib.HistogramVec
	errors                    *prometheuslib.CounterVec
	ownerLookupDuration       *prometheuslib.HistogramVec
	routeDecisions            *prometheuslib.CounterVec
	remoteOwnerConnections    *prometheuslib.CounterVec
	remoteOwnerConnectionsAct *prometheuslib.GaugeVec
	remoteOwnerHandshake      *prometheuslib.HistogramVec
	remoteOwnerMessages       *prometheuslib.CounterVec
	remoteOwnerCloses         *prometheuslib.CounterVec
}

var _ yhttp.Metrics = (*Metrics)(nil)
var _ yhttp.OwnerLookupMetrics = (*Metrics)(nil)
var _ yhttp.RouteDecisionMetrics = (*Metrics)(nil)
var _ yhttp.RemoteOwnerMetrics = (*Metrics)(nil)

// New constrói e registra um conjunto de métricas para `pkg/yhttp`.
func New(cfg Config) (*Metrics, error) {
	namespace := strings.TrimSpace(cfg.Namespace)
	if namespace == "" {
		namespace = defaultNamespace
	}

	subsystem := strings.TrimSpace(cfg.Subsystem)
	if subsystem == "" {
		subsystem = defaultSubsystem
	}

	handleBuckets := cfg.HandleDurationBuckets
	if len(handleBuckets) == 0 {
		handleBuckets = prometheuslib.DefBuckets
	}

	persistBuckets := cfg.PersistDurationBucket
	if len(persistBuckets) == 0 {
		persistBuckets = prometheuslib.DefBuckets
	}
	ownerLookupBuckets := cfg.OwnerLookupDurationBuckets
	if len(ownerLookupBuckets) == 0 {
		ownerLookupBuckets = prometheuslib.DefBuckets
	}
	remoteOwnerHandshakeBuckets := cfg.RemoteOwnerHandshakeBuckets
	if len(remoteOwnerHandshakeBuckets) == 0 {
		remoteOwnerHandshakeBuckets = prometheuslib.DefBuckets
	}

	metrics := &Metrics{
		connectionsOpened: prometheuslib.NewCounter(prometheuslib.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "connections_opened_total",
			Help:      "Total de conexoes WebSocket abertas pelo handler yhttp.",
		}),
		connectionsActive: prometheuslib.NewGauge(prometheuslib.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "connections_active",
			Help:      "Numero atual de conexoes WebSocket ativas no handler yhttp.",
		}),
		framesRead: prometheuslib.NewCounter(prometheuslib.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "frames_read_total",
			Help:      "Total de frames binarios lidos pelo handler yhttp.",
		}),
		framesWritten: prometheuslib.NewCounterVec(prometheuslib.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "frames_written_total",
			Help:      "Total de frames binarios escritos pelo handler yhttp.",
		}, []string{"kind"}),
		bytesRead: prometheuslib.NewCounter(prometheuslib.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "bytes_read_total",
			Help:      "Total de bytes recebidos em frames binarios pelo handler yhttp.",
		}),
		bytesWritten: prometheuslib.NewCounterVec(prometheuslib.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "bytes_written_total",
			Help:      "Total de bytes enviados em frames binarios pelo handler yhttp.",
		}, []string{"kind"}),
		handleDuration: prometheuslib.NewHistogramVec(prometheuslib.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "handle_duration_seconds",
			Help:      "Duracao de processamento de payloads do y-protocol pelo handler yhttp.",
			Buckets:   handleBuckets,
		}, []string{"result"}),
		persistDuration: prometheuslib.NewHistogramVec(prometheuslib.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "persist_duration_seconds",
			Help:      "Duracao da persistencia opcional de snapshot no fechamento da conexao.",
			Buckets:   persistBuckets,
		}, []string{"result"}),
		errors: prometheuslib.NewCounterVec(prometheuslib.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "errors_total",
			Help:      "Total de erros observados pela borda yhttp, rotulados por estagio.",
		}, []string{"stage"}),
		ownerLookupDuration: prometheuslib.NewHistogramVec(prometheuslib.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "owner_lookup_duration_seconds",
			Help:      "Duracao da resolucao de owner na borda owner-aware.",
			Buckets:   ownerLookupBuckets,
		}, []string{"result"}),
		routeDecisions: prometheuslib.NewCounterVec(prometheuslib.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "route_decisions_total",
			Help:      "Total de decisoes de roteamento da borda owner-aware.",
		}, []string{"decision"}),
		remoteOwnerConnections: prometheuslib.NewCounterVec(prometheuslib.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "remote_owner_connections_opened_total",
			Help:      "Total de streams remotos edge/owner abertos para relay inter-node.",
		}, []string{"role"}),
		remoteOwnerConnectionsAct: prometheuslib.NewGaugeVec(prometheuslib.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "remote_owner_connections_active",
			Help:      "Numero atual de streams remotos edge/owner ativos no relay inter-node.",
		}, []string{"role"}),
		remoteOwnerHandshake: prometheuslib.NewHistogramVec(prometheuslib.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "remote_owner_handshake_duration_seconds",
			Help:      "Duracao do handshake inicial entre edge e owner remoto.",
			Buckets:   remoteOwnerHandshakeBuckets,
		}, []string{"role", "result"}),
		remoteOwnerMessages: prometheuslib.NewCounterVec(prometheuslib.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "remote_owner_messages_total",
			Help:      "Total de mensagens tipadas trafegadas no relay inter-node.",
		}, []string{"role", "direction", "kind"}),
		remoteOwnerCloses: prometheuslib.NewCounterVec(prometheuslib.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "remote_owner_closes_total",
			Help:      "Total de encerramentos observados no relay inter-node.",
		}, []string{"role", "reason"}),
	}

	registerer := cfg.Registerer
	if registerer == nil {
		registerer = prometheuslib.DefaultRegisterer
	}

	collectors := []prometheuslib.Collector{
		metrics.connectionsOpened,
		metrics.connectionsActive,
		metrics.framesRead,
		metrics.framesWritten,
		metrics.bytesRead,
		metrics.bytesWritten,
		metrics.handleDuration,
		metrics.persistDuration,
		metrics.errors,
		metrics.ownerLookupDuration,
		metrics.routeDecisions,
		metrics.remoteOwnerConnections,
		metrics.remoteOwnerConnectionsAct,
		metrics.remoteOwnerHandshake,
		metrics.remoteOwnerMessages,
		metrics.remoteOwnerCloses,
	}
	for _, collector := range collectors {
		if err := registerer.Register(collector); err != nil {
			return nil, fmt.Errorf("registrar coletor prometheus: %w", err)
		}
	}

	return metrics, nil
}

// ConnectionOpened registra abertura de conexão.
func (m *Metrics) ConnectionOpened(yhttp.Request) {
	m.connectionsOpened.Inc()
	m.connectionsActive.Inc()
}

// ConnectionClosed registra fechamento de conexão.
func (m *Metrics) ConnectionClosed(yhttp.Request) {
	m.connectionsActive.Dec()
}

// FrameRead registra leitura de um frame binário.
func (m *Metrics) FrameRead(_ yhttp.Request, bytes int) {
	m.framesRead.Inc()
	m.bytesRead.Add(float64(normalizeBytes(bytes)))
}

// FrameWritten registra escrita de um frame binário.
func (m *Metrics) FrameWritten(_ yhttp.Request, kind string, bytes int) {
	label := normalizeLabel(kind, "unknown")
	m.framesWritten.WithLabelValues(label).Inc()
	m.bytesWritten.WithLabelValues(label).Add(float64(normalizeBytes(bytes)))
}

// Handle observa a duração do processamento de um payload do protocolo.
func (m *Metrics) Handle(_ yhttp.Request, duration time.Duration, err error) {
	m.handleDuration.WithLabelValues(resultLabel(err)).Observe(duration.Seconds())
}

// Persist observa a duração da persistência no fechamento.
func (m *Metrics) Persist(_ yhttp.Request, duration time.Duration, err error) {
	m.persistDuration.WithLabelValues(resultLabel(err)).Observe(duration.Seconds())
}

// Error incrementa o contador do estágio informado.
func (m *Metrics) Error(_ yhttp.Request, stage string, err error) {
	if err == nil {
		return
	}
	m.errors.WithLabelValues(normalizeLabel(stage, "unknown")).Inc()
}

// OwnerLookup observa a duracao e o resultado da resolucao de owner.
func (m *Metrics) OwnerLookup(_ yhttp.Request, duration time.Duration, result string) {
	m.ownerLookupDuration.WithLabelValues(normalizeLabel(result, "unknown")).Observe(duration.Seconds())
}

// RouteDecision incrementa o contador da decisao final de roteamento.
func (m *Metrics) RouteDecision(_ yhttp.Request, decision string) {
	m.routeDecisions.WithLabelValues(normalizeLabel(decision, "unknown")).Inc()
}

// RemoteOwnerConnectionOpened contabiliza a abertura de um stream remoto.
func (m *Metrics) RemoteOwnerConnectionOpened(_ yhttp.Request, role string) {
	label := normalizeLabel(role, "unknown")
	m.remoteOwnerConnections.WithLabelValues(label).Inc()
	m.remoteOwnerConnectionsAct.WithLabelValues(label).Inc()
}

// RemoteOwnerConnectionClosed contabiliza o fechamento de um stream remoto.
func (m *Metrics) RemoteOwnerConnectionClosed(_ yhttp.Request, role string) {
	m.remoteOwnerConnectionsAct.WithLabelValues(normalizeLabel(role, "unknown")).Dec()
}

// RemoteOwnerHandshake observa a duracao do handshake inicial do relay.
func (m *Metrics) RemoteOwnerHandshake(_ yhttp.Request, role string, duration time.Duration, err error) {
	m.remoteOwnerHandshake.WithLabelValues(normalizeLabel(role, "unknown"), resultLabel(err)).Observe(duration.Seconds())
}

// RemoteOwnerMessage contabiliza mensagens tipadas trafegadas pelo relay.
func (m *Metrics) RemoteOwnerMessage(_ yhttp.Request, role string, direction string, kind string) {
	m.remoteOwnerMessages.WithLabelValues(
		normalizeLabel(role, "unknown"),
		normalizeLabel(direction, "unknown"),
		normalizeLabel(kind, "unknown"),
	).Inc()
}

// RemoteOwnerClose contabiliza o motivo observado no encerramento do relay.
func (m *Metrics) RemoteOwnerClose(_ yhttp.Request, role string, reason string) {
	m.remoteOwnerCloses.WithLabelValues(
		normalizeLabel(role, "unknown"),
		normalizeLabel(reason, "unknown"),
	).Inc()
}

func normalizeBytes(value int) int {
	if value < 0 {
		return 0
	}
	return value
}

func normalizeLabel(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func resultLabel(err error) string {
	if err != nil {
		return "error"
	}
	return "ok"
}
