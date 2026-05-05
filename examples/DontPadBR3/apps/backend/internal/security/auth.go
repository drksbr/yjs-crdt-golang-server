package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/common"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	db             *pgxpool.Pool
	schemaSQL      string
	namespace      string
	address        string
	masterPassword string
	secret         []byte
	rateLimiter    *pinRateLimiter
}

type Deps struct {
	DB             *pgxpool.Pool
	SchemaSQL      string
	Namespace      string
	Address        string
	MasterPassword string
	Secret         []byte
}

func New(deps Deps) *Service {
	return &Service{
		db:             deps.DB,
		schemaSQL:      deps.SchemaSQL,
		namespace:      deps.Namespace,
		address:        deps.Address,
		masterPassword: deps.MasterPassword,
		secret:         deps.Secret,
		rateLimiter: &pinRateLimiter{
			entries: make(map[string]pinRateLimitEntry),
		},
	}
}

type pinRateLimiter struct {
	mu      sync.Mutex
	entries map[string]pinRateLimitEntry
}

type pinRateLimitEntry struct {
	Count   int
	ResetAt time.Time
}

func (s *Service) SignToken(payload common.SignedToken) (string, error) {
	raw := fmt.Sprintf("%s|%s|%d", payload.DocumentID, payload.Scope, payload.ExpiresAt)
	mac := hmac.New(sha256.New, s.secret)
	if _, err := mac.Write([]byte(raw)); err != nil {
		return "", err
	}
	sig := hex.EncodeToString(mac.Sum(nil))
	full := raw + "|" + sig
	return base64.RawURLEncoding.EncodeToString([]byte(full)), nil
}

func (s *Service) VerifySignedToken(encoded, expectedScope string) (common.SignedToken, error) {
	rawBytes, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return common.SignedToken{}, err
	}
	parts := strings.Split(string(rawBytes), "|")
	if len(parts) != 4 {
		return common.SignedToken{}, errors.New("token format invalido")
	}
	expiresAt, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return common.SignedToken{}, err
	}
	raw := strings.Join(parts[:3], "|")
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(raw))
	expected := hex.EncodeToString(mac.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(parts[3]), []byte(expected)) != 1 {
		return common.SignedToken{}, errors.New("assinatura invalida")
	}
	if parts[1] != expectedScope {
		return common.SignedToken{}, errors.New("scope invalido")
	}
	if time.Now().Unix() > expiresAt {
		return common.SignedToken{}, errors.New("token expirado")
	}
	return common.SignedToken{
		DocumentID: parts[0],
		Scope:      parts[1],
		ExpiresAt:  expiresAt,
	}, nil
}

func (s *Service) SetDocumentAuthCookie(c *gin.Context, documentID string) {
	token, err := s.SignToken(common.SignedToken{
		DocumentID: documentID,
		Scope:      "doc",
		ExpiresAt:  time.Now().Add(common.DefaultCookieTTL).Unix(),
	})
	if err != nil {
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     common.CookieNamePrefix + documentID,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   false,
		MaxAge:   int(common.DefaultCookieTTL.Seconds()),
	})
}

func (s *Service) ClearDocumentAuthCookie(c *gin.Context, documentID string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     common.CookieNamePrefix + documentID,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   false,
		MaxAge:   -1,
	})
}

func (s *Service) HasDocumentAccessCookie(r *http.Request, documentID string) bool {
	cookie, err := r.Cookie(common.CookieNamePrefix + documentID)
	if err != nil {
		return false
	}
	token, err := s.VerifySignedToken(cookie.Value, "doc")
	if err != nil {
		return false
	}
	return token.DocumentID == documentID
}

func (s *Service) WebsocketBaseURL(r *http.Request) string {
	scheme := "ws"
	host := r.Host
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-Host")); forwarded != "" {
		host = forwarded
	}
	if proto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); proto == "https" || proto == "wss" {
		scheme = "wss"
	}
	if strings.HasPrefix(strings.ToLower(r.Host), "localhost:3000") || strings.HasPrefix(strings.ToLower(r.Host), "127.0.0.1:3000") {
		host = common.NormalizeDisplayAddress(s.address)
		scheme = "ws"
	}
	return fmt.Sprintf("%s://%s/ws", scheme, host)
}

func (rl *pinRateLimiter) allow(key string, maxAttempts int, window time.Duration) (bool, time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	entry, ok := rl.entries[key]
	if !ok || now.After(entry.ResetAt) {
		rl.entries[key] = pinRateLimitEntry{
			Count:   1,
			ResetAt: now.Add(window),
		}
		return true, 0
	}
	if entry.Count >= maxAttempts {
		return false, entry.ResetAt.Sub(now)
	}
	entry.Count++
	rl.entries[key] = entry
	return true, 0
}

func (rl *pinRateLimiter) reset(key string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.entries, key)
}

func IsVisibilityModeValid(mode common.VisibilityMode) bool {
	return mode == common.VisibilityPublic || mode == common.VisibilityPublicRead || mode == common.VisibilityPrivate
}

func hashPIN(pin string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(pin), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func verifyPIN(pin, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pin)) == nil
}
