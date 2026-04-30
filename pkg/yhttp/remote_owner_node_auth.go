package yhttp

import (
	"context"
	"crypto/hmac"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/ycluster"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/ynodeproto"
)

// RemoteOwnerNodeAuthorizationHeader é o header dedicado para autenticação
// edge -> owner entre nós.
const RemoteOwnerNodeAuthorizationHeader = "X-Yjs-Node-Authorization"

const (
	// RemoteOwnerNodeSignatureHeader carrega a assinatura HMAC do handshake
	// inter-node.
	RemoteOwnerNodeSignatureHeader = "X-Yjs-Node-Signature"
	// RemoteOwnerNodeTimestampHeader carrega o timestamp Unix protegido pelo
	// HMAC inter-node.
	RemoteOwnerNodeTimestampHeader = "X-Yjs-Node-Timestamp"
	// RemoteOwnerNodeNonceHeader carrega o nonce protegido pelo HMAC
	// inter-node.
	RemoteOwnerNodeNonceHeader = "X-Yjs-Node-Nonce"
	// RemoteOwnerNodeKeyIDHeader identifica o segredo HMAC usado para permitir
	// rotação segura de chaves inter-node.
	RemoteOwnerNodeKeyIDHeader = "X-Yjs-Node-Key-Id"

	defaultRemoteOwnerHMACTimeWindow = 2 * time.Minute
	remoteOwnerHMACSignatureVersion  = "v1"
)

// RemoteOwnerAuthHeadersFunc gera headers de autenticação para o dialer
// inter-node.
type RemoteOwnerAuthHeadersFunc func(ctx context.Context, req RemoteOwnerDialRequest) (http.Header, error)

// RemoteOwnerBearerAuthHeaders gera `X-Yjs-Node-Authorization: Bearer <token>`.
func RemoteOwnerBearerAuthHeaders(token string) RemoteOwnerAuthHeadersFunc {
	token = strings.TrimSpace(token)
	return func(context.Context, RemoteOwnerDialRequest) (http.Header, error) {
		if token == "" {
			return nil, ErrUnauthorized
		}
		header := make(http.Header, 1)
		header.Set(RemoteOwnerNodeAuthorizationHeader, "Bearer "+token)
		return header, nil
	}
}

// RemoteOwnerBearerAuthenticator valida o token bearer inter-node dedicado.
func RemoteOwnerBearerAuthenticator(tokens ...string) RemoteOwnerAuthenticator {
	allowed := make(map[string]struct{}, len(tokens))
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token != "" {
			allowed[token] = struct{}{}
		}
	}
	return func(_ context.Context, req RemoteOwnerAuthRequest) error {
		value := strings.TrimSpace(req.Header.Get(RemoteOwnerNodeAuthorizationHeader))
		token, ok := strings.CutPrefix(value, "Bearer ")
		if !ok {
			return ErrUnauthorized
		}
		if _, ok := allowed[strings.TrimSpace(token)]; !ok {
			return ErrUnauthorized
		}
		return nil
	}
}

// RemoteOwnerNonceStore registra nonces já consumidos para bloquear replay de
// handshakes inter-node ainda válidos.
type RemoteOwnerNonceStore interface {
	CheckAndStore(ctx context.Context, key string, expiresAt time.Time) error
}

// InMemoryRemoteOwnerNonceStoreConfig define o clock usado pela store de nonce
// em memória.
type InMemoryRemoteOwnerNonceStoreConfig struct {
	Now func() time.Time
}

// InMemoryRemoteOwnerNonceStore mantém nonces em memória com TTL e proteção
// por mutex para replay protection local ao processo.
type InMemoryRemoteOwnerNonceStore struct {
	mu      sync.Mutex
	now     func() time.Time
	entries map[string]time.Time
}

// NewInMemoryRemoteOwnerNonceStore constrói uma store local de nonce para
// autenticação HMAC inter-node.
func NewInMemoryRemoteOwnerNonceStore(cfg InMemoryRemoteOwnerNonceStoreConfig) *InMemoryRemoteOwnerNonceStore {
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &InMemoryRemoteOwnerNonceStore{
		now:     now,
		entries: make(map[string]time.Time),
	}
}

// CheckAndStore rejeita nonces ainda ativos e registra um novo TTL para o
// nonce informado.
func (s *InMemoryRemoteOwnerNonceStore) CheckAndStore(ctx context.Context, key string, expiresAt time.Time) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("%w: nonce inter-node obrigatorio", ErrUnauthorized)
	}
	if expiresAt.IsZero() {
		return fmt.Errorf("%w: expiracao do nonce obrigatoria", ErrUnauthorized)
	}

	now := time.Now
	if s != nil && s.now != nil {
		now = s.now
	}

	if s == nil {
		return fmt.Errorf("%w: nonce store inter-node obrigatoria", ErrUnauthorized)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return err
	}
	if s.entries == nil {
		s.entries = make(map[string]time.Time)
	}

	current := now().UTC()
	for storedKey, storedExpiry := range s.entries {
		if storedExpiry.Before(current) {
			delete(s.entries, storedKey)
		}
	}
	if storedExpiry, ok := s.entries[key]; ok && !storedExpiry.Before(current) {
		return fmt.Errorf("%w: nonce inter-node repetido", ErrUnauthorized)
	}

	s.entries[key] = expiresAt.UTC()
	return nil
}

// RemoteOwnerHMACAuthHeadersConfig define como o edge assina o handshake
// inter-node antes de abrir o stream com o owner remoto.
type RemoteOwnerHMACAuthHeadersConfig struct {
	Secret   string
	KeyID    string
	NodeID   ycluster.NodeID
	Now      func() time.Time
	NewNonce func(ctx context.Context) (string, error)
}

// RemoteOwnerHMACAuthenticatorConfig define como o owner valida o HMAC, a
// janela temporal e o nonce do handshake inter-node.
type RemoteOwnerHMACAuthenticatorConfig struct {
	Secret       string
	Secrets      map[string]string
	RequireKeyID bool
	TimeWindow   time.Duration
	NonceStore   RemoteOwnerNonceStore
	Now          func() time.Time
}

// RemoteOwnerHMACAuthHeaders gera headers HMAC dedicados para autenticação
// inter-node com timestamp e nonce.
func RemoteOwnerHMACAuthHeaders(cfg RemoteOwnerHMACAuthHeadersConfig) RemoteOwnerAuthHeadersFunc {
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	newNonce := cfg.NewNonce
	if newNonce == nil {
		newNonce = defaultRemoteOwnerNonce
	}
	secret := strings.TrimSpace(cfg.Secret)
	keyID := strings.TrimSpace(cfg.KeyID)
	nodeID := cfg.NodeID

	return func(ctx context.Context, req RemoteOwnerDialRequest) (http.Header, error) {
		if ctx == nil {
			ctx = context.Background()
		}
		if secret == "" {
			return nil, fmt.Errorf("%w: segredo HMAC inter-node obrigatorio", ErrUnauthorized)
		}
		if err := nodeID.Validate(); err != nil {
			return nil, err
		}

		authReq, err := remoteOwnerAuthRequestFromDial(nodeID, req)
		if err != nil {
			return nil, err
		}

		timestamp := strconv.FormatInt(now().UTC().Unix(), 10)
		nonce, err := newNonce(ctx)
		if err != nil {
			return nil, err
		}
		nonce = strings.TrimSpace(nonce)
		if nonce == "" {
			return nil, fmt.Errorf("%w: nonce HMAC inter-node obrigatorio", ErrUnauthorized)
		}

		signature, err := remoteOwnerHMACSignature(secret, authReq, timestamp, nonce, keyID)
		if err != nil {
			return nil, err
		}

		header := make(http.Header, 3)
		header.Set(RemoteOwnerNodeTimestampHeader, timestamp)
		header.Set(RemoteOwnerNodeNonceHeader, nonce)
		header.Set(RemoteOwnerNodeSignatureHeader, signature)
		if keyID != "" {
			header.Set(RemoteOwnerNodeKeyIDHeader, keyID)
		}
		return header, nil
	}
}

// RemoteOwnerHMACAuthenticator valida headers HMAC dedicados e protege o
// owner contra replay dentro da janela configurada.
func RemoteOwnerHMACAuthenticator(cfg RemoteOwnerHMACAuthenticatorConfig) RemoteOwnerAuthenticator {
	now := cfg.Now
	if now == nil {
		now = time.Now
	}

	timeWindow := cfg.TimeWindow
	if timeWindow <= 0 {
		timeWindow = defaultRemoteOwnerHMACTimeWindow
	}

	nonceStore := cfg.NonceStore
	if nonceStore == nil {
		nonceStore = NewInMemoryRemoteOwnerNonceStore(InMemoryRemoteOwnerNonceStoreConfig{Now: now})
	}

	secret := strings.TrimSpace(cfg.Secret)
	secrets := cloneHMACSecrets(cfg.Secrets)
	requireKeyID := cfg.RequireKeyID

	return func(ctx context.Context, req RemoteOwnerAuthRequest) error {
		if ctx == nil {
			ctx = context.Background()
		}
		if err := req.NodeID.Validate(); err != nil {
			return fmt.Errorf("%w: %v", ErrUnauthorized, err)
		}

		timestamp, nonce, actualSignature, keyID, err := remoteOwnerAuthHeaderValues(req.Header)
		if err != nil {
			return err
		}
		resolvedSecret, err := resolveRemoteOwnerHMACSecret(secret, secrets, keyID, requireKeyID)
		if err != nil {
			return err
		}

		observedAt, err := parseRemoteOwnerTimestamp(timestamp)
		if err != nil {
			return fmt.Errorf("%w: timestamp inter-node invalido", ErrUnauthorized)
		}

		current := now().UTC()
		if observedAt.Before(current.Add(-timeWindow)) || observedAt.After(current.Add(timeWindow)) {
			return fmt.Errorf("%w: timestamp inter-node fora da janela", ErrUnauthorized)
		}

		expectedMAC, err := remoteOwnerHMACDigest(resolvedSecret, req, timestamp, nonce, keyID)
		if err != nil {
			return err
		}
		actualMAC, err := remoteOwnerDecodeSignature(actualSignature)
		if err != nil {
			return fmt.Errorf("%w: assinatura inter-node invalida", ErrUnauthorized)
		}
		if !hmac.Equal(actualMAC, expectedMAC) {
			return fmt.Errorf("%w: assinatura inter-node invalida", ErrUnauthorized)
		}

		expiresAt := observedAt.Add(timeWindow)
		if err := nonceStore.CheckAndStore(ctx, remoteOwnerNonceStoreKey(req.NodeID, keyID, nonce), expiresAt); err != nil {
			return err
		}
		return nil
	}
}

type remoteOwnerHMACPayload struct {
	Version      string `json:"version"`
	NodeID       string `json:"nodeId"`
	Namespace    string `json:"namespace"`
	DocumentID   string `json:"documentId"`
	ConnectionID string `json:"connectionId"`
	ClientID     uint32 `json:"clientId"`
	Epoch        uint64 `json:"epoch"`
	Flags        uint16 `json:"flags"`
	Timestamp    string `json:"timestamp"`
	Nonce        string `json:"nonce"`
	KeyID        string `json:"keyId,omitempty"`
}

func remoteOwnerAuthRequestFromDial(nodeID ycluster.NodeID, req RemoteOwnerDialRequest) (RemoteOwnerAuthRequest, error) {
	epoch, err := remoteOwnerEpoch(req.Resolution)
	if err != nil {
		return RemoteOwnerAuthRequest{}, err
	}

	return RemoteOwnerAuthRequest{
		NodeID:       nodeID,
		DocumentKey:  req.Request.DocumentKey,
		ConnectionID: req.Request.ConnectionID,
		ClientID:     req.Request.ClientID,
		Epoch:        epoch,
		Flags:        remoteOwnerRequestFlags(req.Request),
	}, nil
}

func remoteOwnerRequestFlags(req Request) ynodeproto.Flags {
	flags := ynodeproto.FlagNone
	if req.PersistOnClose {
		flags |= ynodeproto.FlagPersistOnClose
	}
	return flags
}

func remoteOwnerAuthHeaderValues(header http.Header) (timestamp string, nonce string, signature string, keyID string, err error) {
	if len(header) == 0 {
		return "", "", "", "", ErrUnauthorized
	}

	timestamp = strings.TrimSpace(header.Get(RemoteOwnerNodeTimestampHeader))
	nonce = strings.TrimSpace(header.Get(RemoteOwnerNodeNonceHeader))
	signature = strings.TrimSpace(header.Get(RemoteOwnerNodeSignatureHeader))
	keyID = strings.TrimSpace(header.Get(RemoteOwnerNodeKeyIDHeader))
	if timestamp == "" || nonce == "" || signature == "" {
		return "", "", "", "", ErrUnauthorized
	}
	return timestamp, nonce, signature, keyID, nil
}

func remoteOwnerHMACSignature(secret string, req RemoteOwnerAuthRequest, timestamp string, nonce string, keyID string) (string, error) {
	sum, err := remoteOwnerHMACDigest(secret, req, timestamp, nonce, keyID)
	if err != nil {
		return "", err
	}
	return remoteOwnerEncodeSignature(sum), nil
}

func remoteOwnerHMACDigest(secret string, req RemoteOwnerAuthRequest, timestamp string, nonce string, keyID string) ([]byte, error) {
	payload, err := remoteOwnerHMACPayloadBytes(req, timestamp, nonce, keyID)
	if err != nil {
		return nil, err
	}

	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := mac.Write(payload); err != nil {
		return nil, err
	}
	return mac.Sum(nil), nil
}

func remoteOwnerHMACPayloadBytes(req RemoteOwnerAuthRequest, timestamp string, nonce string, keyID string) ([]byte, error) {
	payload := remoteOwnerHMACPayload{
		Version:      remoteOwnerHMACSignatureVersion,
		NodeID:       req.NodeID.String(),
		Namespace:    req.DocumentKey.Namespace,
		DocumentID:   req.DocumentKey.DocumentID,
		ConnectionID: req.ConnectionID,
		ClientID:     req.ClientID,
		Epoch:        req.Epoch,
		Flags:        uint16(req.Flags),
		Timestamp:    timestamp,
		Nonce:        nonce,
		KeyID:        strings.TrimSpace(keyID),
	}
	return json.Marshal(payload)
}

func remoteOwnerEncodeSignature(sum []byte) string {
	return remoteOwnerHMACSignatureVersion + "=" + base64.RawURLEncoding.EncodeToString(sum)
}

func remoteOwnerDecodeSignature(value string) ([]byte, error) {
	version, encoded, ok := strings.Cut(strings.TrimSpace(value), "=")
	if !ok || version != remoteOwnerHMACSignatureVersion || encoded == "" {
		return nil, ErrUnauthorized
	}
	return base64.RawURLEncoding.DecodeString(encoded)
}

func parseRemoteOwnerTimestamp(value string) (time.Time, error) {
	seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(seconds, 0).UTC(), nil
}

func cloneHMACSecrets(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(src))
	for keyID, secret := range src {
		keyID = strings.TrimSpace(keyID)
		secret = strings.TrimSpace(secret)
		if keyID != "" && secret != "" {
			cloned[keyID] = secret
		}
	}
	return cloned
}

func resolveRemoteOwnerHMACSecret(fallback string, secrets map[string]string, keyID string, requireKeyID bool) (string, error) {
	keyID = strings.TrimSpace(keyID)
	if keyID != "" {
		secret := strings.TrimSpace(secrets[keyID])
		if secret == "" {
			return "", fmt.Errorf("%w: key id HMAC inter-node desconhecido", ErrUnauthorized)
		}
		return secret, nil
	}
	if requireKeyID {
		return "", fmt.Errorf("%w: key id HMAC inter-node obrigatorio", ErrUnauthorized)
	}
	fallback = strings.TrimSpace(fallback)
	if fallback == "" {
		return "", fmt.Errorf("%w: segredo HMAC inter-node obrigatorio", ErrUnauthorized)
	}
	return fallback, nil
}

func remoteOwnerNonceStoreKey(nodeID ycluster.NodeID, keyID string, nonce string) string {
	keyID = strings.TrimSpace(keyID)
	if keyID == "" {
		keyID = "default"
	}
	return nodeID.String() + ":" + keyID + ":" + nonce
}

func defaultRemoteOwnerNonce(context.Context) (string, error) {
	var buf [16]byte
	if _, err := cryptorand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}
