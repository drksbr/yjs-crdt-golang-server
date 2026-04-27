package yhttp

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"yjs-go-bridge/pkg/storage"
	"yjs-go-bridge/pkg/ycluster"
)

const (
	defaultOwnerRetryAfter = time.Second
	remoteOwnerStatusCode  = http.StatusConflict
)

// RemoteOwnerHandler permite customizar o comportamento quando o owner
// resolvido é remoto.
//
// O handler deve retornar `true` quando já tiver escrito a resposta completa.
// Se retornar `false`, `OwnerAwareServer` cai no payload padrão com metadados do
// owner remoto. `NewRemoteOwnerForwardHandler` constrói uma implementação
// padrão para proxy WebSocket via `RemoteOwnerDialer`.
type RemoteOwnerHandler func(w http.ResponseWriter, r *http.Request, req Request, resolution ycluster.OwnerResolution) bool

// OwnerAwareServerConfig descreve o wiring do handler owner-aware.
type OwnerAwareServerConfig struct {
	Local         *Server
	OwnerLookup   ycluster.OwnerLookup
	OnRemoteOwner RemoteOwnerHandler
	RetryAfter    time.Duration
}

// OwnerAwareServer resolve ownership antes de encaminhar a conexão ao
// `Server` local.
//
// Quando o owner resolvido é local, o comportamento é idêntico ao do `Server`
// encapsulado. Quando o owner é remoto, o handler pode delegar a um hook
// customizado ou responder com metadados retryable até que o proxy inter-node
// seja implementado.
type OwnerAwareServer struct {
	local         *Server
	metrics       Metrics
	ownerLookup   ycluster.OwnerLookup
	onRemoteOwner RemoteOwnerHandler
	retryAfter    time.Duration
}

// NewOwnerAwareServer valida a configuração e constrói um handler owner-aware
// acima de um `Server` local já configurado.
func NewOwnerAwareServer(cfg OwnerAwareServerConfig) (*OwnerAwareServer, error) {
	if cfg.Local == nil {
		return nil, ErrNilLocalServer
	}
	if cfg.OwnerLookup == nil {
		return nil, ErrNilOwnerLookup
	}

	retryAfter := cfg.RetryAfter
	if retryAfter <= 0 {
		retryAfter = defaultOwnerRetryAfter
	}

	return &OwnerAwareServer{
		local:         cfg.Local,
		metrics:       normalizeMetrics(cfg.Local.metrics),
		ownerLookup:   cfg.OwnerLookup,
		onRemoteOwner: cfg.OnRemoteOwner,
		retryAfter:    retryAfter,
	}, nil
}

// ServeHTTP resolve ownership do documento antes de abrir o provider local.
func (s *OwnerAwareServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	req, err := s.local.resolveRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resolution, err := s.ownerLookup.LookupOwner(r.Context(), ycluster.OwnerLookupRequest{
		DocumentKey: req.DocumentKey,
	})
	if err != nil {
		s.metrics.Error(req, "lookup_owner", err)
		s.writeLookupError(w, err)
		return
	}
	if resolution == nil {
		err = ycluster.ErrOwnerNotFound
		s.metrics.Error(req, "lookup_owner", err)
		s.writeLookupError(w, err)
		return
	}
	if resolution.Local {
		s.local.serveResolvedHTTP(w, r, req)
		return
	}
	if strings.TrimSpace(req.ConnectionID) == "" {
		req.ConnectionID = s.local.newConnectionID()
	}

	if s.onRemoteOwner != nil && s.onRemoteOwner(w, r, req, *resolution) {
		return
	}
	s.writeRemoteOwnerResponse(w, req, *resolution)
}

func (s *OwnerAwareServer) writeLookupError(w http.ResponseWriter, err error) {
	status := statusFromOwnerLookupError(err)
	if status == http.StatusServiceUnavailable {
		s.setRetryAfterHeader(w)
	}
	http.Error(w, err.Error(), status)
}

func (s *OwnerAwareServer) writeRemoteOwnerResponse(w http.ResponseWriter, req Request, resolution ycluster.OwnerResolution) {
	response := remoteOwnerResponse{
		Error:       "yhttp: owner remoto; reenvie para o no owner ou tente novamente",
		Code:        "remote_owner",
		Retryable:   true,
		DocumentKey: req.DocumentKey,
		Owner: remoteOwnerMetadata{
			NodeID:  resolution.Placement.NodeID.String(),
			ShardID: uint32(resolution.Placement.ShardID),
			Version: resolution.Placement.Version,
		},
	}
	if lease := resolution.Placement.Lease; lease != nil && !lease.ExpiresAt.IsZero() {
		expiresAt := lease.ExpiresAt.UTC()
		response.Owner.LeaseExpiresAt = &expiresAt
		response.Owner.Epoch = lease.Epoch
	}

	payload, err := json.Marshal(response)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Yjs-Owner-Node", response.Owner.NodeID)
	w.Header().Set("X-Yjs-Owner-Shard", strconv.FormatUint(uint64(response.Owner.ShardID), 10))
	w.Header().Set("X-Yjs-Retryable", strconv.FormatBool(response.Retryable))
	if response.Owner.Version > 0 {
		w.Header().Set("X-Yjs-Owner-Version", strconv.FormatUint(response.Owner.Version, 10))
	}
	if response.Owner.Epoch > 0 {
		w.Header().Set("X-Yjs-Owner-Epoch", strconv.FormatUint(response.Owner.Epoch, 10))
	}
	s.setRetryAfterHeader(w)
	w.WriteHeader(remoteOwnerStatusCode)
	_, _ = w.Write(payload)
}

func (s *OwnerAwareServer) setRetryAfterHeader(w http.ResponseWriter) {
	if s.retryAfter <= 0 {
		return
	}

	seconds := s.retryAfter / time.Second
	if s.retryAfter%time.Second != 0 {
		seconds++
	}
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.FormatInt(int64(seconds), 10))
}

func statusFromOwnerLookupError(err error) int {
	switch {
	case errors.Is(err, ycluster.ErrInvalidOwnerLookupRequest), errors.Is(err, storage.ErrInvalidDocumentKey):
		return http.StatusBadRequest
	case errors.Is(err, ycluster.ErrOwnerNotFound),
		errors.Is(err, ycluster.ErrPlacementNotFound),
		errors.Is(err, ycluster.ErrLeaseExpired),
		errors.Is(err, ycluster.ErrInvalidPlacement),
		errors.Is(err, ycluster.ErrInvalidLease):
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

type remoteOwnerResponse struct {
	Error       string              `json:"error"`
	Code        string              `json:"code"`
	Retryable   bool                `json:"retryable"`
	DocumentKey storage.DocumentKey `json:"documentKey"`
	Owner       remoteOwnerMetadata `json:"owner"`
}

type remoteOwnerMetadata struct {
	NodeID         string     `json:"nodeID"`
	ShardID        uint32     `json:"shardID"`
	Version        uint64     `json:"version,omitempty"`
	Epoch          uint64     `json:"epoch,omitempty"`
	LeaseExpiresAt *time.Time `json:"leaseExpiresAt,omitempty"`
}
