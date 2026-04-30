package yhttp

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
)

// BearerTokenAuthenticator autentica requests por token Bearer estático.
//
// A chave do mapa é o token sem o prefixo "Bearer ". Os principals são
// copiados antes de serem anexados à request.
type BearerTokenAuthenticator struct {
	Tokens map[string]Principal
}

// AuthenticateHTTP valida o header Authorization: Bearer <token>.
func (a BearerTokenAuthenticator) AuthenticateHTTP(_ context.Context, r *http.Request) (*Principal, error) {
	if r == nil {
		return nil, ErrUnauthorized
	}
	value := strings.TrimSpace(r.Header.Get("Authorization"))
	if value == "" {
		return nil, ErrUnauthorized
	}
	token, ok := strings.CutPrefix(value, "Bearer ")
	if !ok || strings.TrimSpace(token) == "" {
		return nil, ErrUnauthorized
	}
	principal, ok := a.Tokens[strings.TrimSpace(token)]
	if !ok {
		return nil, ErrUnauthorized
	}
	return clonePrincipal(&principal), nil
}

// TenantAuthorizer aplica `Principal.Tenant` como fronteira para
// `DocumentKey.Namespace`.
type TenantAuthorizer struct {
	AllowEmptyTenant bool
}

// AuthorizeHTTP permite a request quando o tenant do principal corresponde ao
// namespace do documento.
func (a TenantAuthorizer) AuthorizeHTTP(_ context.Context, principal *Principal, req Request) error {
	if principal == nil {
		return ErrUnauthorized
	}
	if strings.TrimSpace(principal.Tenant) == "" && a.AllowEmptyTenant {
		return nil
	}
	if !documentNamespaceMatchesTenant(req.DocumentKey, principal.Tenant) {
		return ErrForbidden
	}
	return nil
}

func documentNamespaceMatchesTenant(key storage.DocumentKey, tenant string) bool {
	return strings.TrimSpace(key.Namespace) == strings.TrimSpace(tenant)
}

func clonePrincipal(src *Principal) *Principal {
	if src == nil {
		return nil
	}
	cloned := *src
	if len(src.Scopes) > 0 {
		cloned.Scopes = append([]string(nil), src.Scopes...)
	}
	return &cloned
}

func statusFromAuthError(err error) int {
	switch {
	case errors.Is(err, ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, ErrRateLimited):
		return http.StatusTooManyRequests
	case errors.Is(err, ErrQuotaExceeded):
		return http.StatusTooManyRequests
	case errors.Is(err, ErrQuotaUnavailable):
		return http.StatusServiceUnavailable
	case errors.Is(err, ErrOriginDenied):
		return http.StatusForbidden
	default:
		return http.StatusForbidden
	}
}
