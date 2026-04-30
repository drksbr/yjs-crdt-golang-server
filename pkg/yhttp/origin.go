package yhttp

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// OriginPolicy validates browser Origin headers and can emit CORS headers.
type OriginPolicy interface {
	CheckHTTPOrigin(ctx context.Context, r *http.Request, req Request) error
}

// OriginPolicyFunc adapts a function to OriginPolicy.
type OriginPolicyFunc func(ctx context.Context, r *http.Request, req Request) error

// CheckHTTPOrigin validates the origin by calling the wrapped function.
func (f OriginPolicyFunc) CheckHTTPOrigin(ctx context.Context, r *http.Request, req Request) error {
	return f(ctx, r, req)
}

// CORSHeaderPolicy can decorate accepted HTTP/WebSocket responses with CORS
// headers for the configured origin policy.
type CORSHeaderPolicy interface {
	WriteCORSHeaders(w http.ResponseWriter, r *http.Request)
}

// CORSPreflightPolicy handles OPTIONS preflight requests before request
// resolution.
type CORSPreflightPolicy interface {
	HandleCORSPreflight(ctx context.Context, w http.ResponseWriter, r *http.Request) bool
}

type websocketOriginPolicy interface {
	websocketOriginPatterns() ([]string, bool)
}

// StaticOriginPolicy is a small production-oriented reference implementation
// for exact-origin allowlists and CORS preflight handling.
type StaticOriginPolicy struct {
	AllowedOrigins     []string
	AllowedMethods     []string
	AllowedHeaders     []string
	ExposedHeaders     []string
	AllowCredentials   bool
	AllowMissingOrigin bool
	MaxAge             time.Duration
}

// CheckHTTPOrigin allows requests whose Origin matches the allowlist.
func (p StaticOriginPolicy) CheckHTTPOrigin(_ context.Context, r *http.Request, _ Request) error {
	origin := requestOrigin(r)
	if origin == "" {
		if p.AllowMissingOrigin {
			return nil
		}
		return ErrOriginDenied
	}
	if p.originAllowed(origin) {
		return nil
	}
	return ErrOriginDenied
}

// WriteCORSHeaders writes CORS headers when the request origin is accepted.
func (p StaticOriginPolicy) WriteCORSHeaders(w http.ResponseWriter, r *http.Request) {
	if w == nil {
		return
	}
	origin := requestOrigin(r)
	if origin == "" || !p.originAllowed(origin) {
		return
	}
	writeVaryOrigin(w)
	w.Header().Set("Access-Control-Allow-Origin", p.allowedOriginHeader(origin))
	if p.AllowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}
	if methods := joinHeaderValues(p.AllowedMethods); methods != "" {
		w.Header().Set("Access-Control-Allow-Methods", methods)
	}
	if headers := joinHeaderValues(p.AllowedHeaders); headers != "" {
		w.Header().Set("Access-Control-Allow-Headers", headers)
	}
	if exposed := joinHeaderValues(p.ExposedHeaders); exposed != "" {
		w.Header().Set("Access-Control-Expose-Headers", exposed)
	}
	if p.MaxAge > 0 {
		w.Header().Set("Access-Control-Max-Age", strconv.FormatInt(int64(p.MaxAge/time.Second), 10))
	}
}

// HandleCORSPreflight handles accepted or denied OPTIONS preflight requests.
func (p StaticOriginPolicy) HandleCORSPreflight(ctx context.Context, w http.ResponseWriter, r *http.Request) bool {
	if r == nil || r.Method != http.MethodOptions || requestOrigin(r) == "" {
		return false
	}
	if err := p.CheckHTTPOrigin(ctx, r, Request{}); err != nil {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return true
	}
	if !p.preflightMethodAllowed(r.Header.Get("Access-Control-Request-Method")) ||
		!p.preflightHeadersAllowed(r.Header.Get("Access-Control-Request-Headers")) {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return true
	}
	p.WriteCORSHeaders(w, r)
	w.WriteHeader(http.StatusNoContent)
	return true
}

func (p StaticOriginPolicy) originAllowed(origin string) bool {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return false
	}
	for _, allowed := range p.AllowedOrigins {
		allowed = strings.TrimSpace(allowed)
		if allowed == "" {
			continue
		}
		if allowed == "*" && !p.AllowCredentials {
			return true
		}
		if allowed == origin {
			return true
		}
	}
	return false
}

func (p StaticOriginPolicy) allowedOriginHeader(origin string) string {
	for _, allowed := range p.AllowedOrigins {
		if strings.TrimSpace(allowed) == "*" && !p.AllowCredentials {
			return "*"
		}
	}
	return origin
}

func (p StaticOriginPolicy) websocketOriginPatterns() ([]string, bool) {
	patterns := make([]string, 0, len(p.AllowedOrigins))
	for _, allowed := range p.AllowedOrigins {
		allowed = strings.TrimSpace(allowed)
		if allowed == "" {
			continue
		}
		if allowed == "*" && !p.AllowCredentials {
			return nil, true
		}
		patterns = append(patterns, allowed)
	}
	return patterns, false
}

func (p StaticOriginPolicy) preflightMethodAllowed(method string) bool {
	method = strings.TrimSpace(method)
	if method == "" || len(p.AllowedMethods) == 0 {
		return true
	}
	for _, allowed := range p.AllowedMethods {
		if strings.EqualFold(strings.TrimSpace(allowed), method) {
			return true
		}
	}
	return false
}

func (p StaticOriginPolicy) preflightHeadersAllowed(headers string) bool {
	if strings.TrimSpace(headers) == "" || len(p.AllowedHeaders) == 0 {
		return true
	}
	allowed := make(map[string]struct{}, len(p.AllowedHeaders))
	for _, header := range p.AllowedHeaders {
		header = strings.ToLower(strings.TrimSpace(header))
		if header != "" {
			allowed[header] = struct{}{}
		}
	}
	for _, header := range strings.Split(headers, ",") {
		header = strings.ToLower(strings.TrimSpace(header))
		if header == "" {
			continue
		}
		if _, ok := allowed[header]; !ok {
			return false
		}
	}
	return true
}

func requestOrigin(r *http.Request) string {
	if r == nil {
		return ""
	}
	return strings.TrimSpace(r.Header.Get("Origin"))
}

func writeVaryOrigin(w http.ResponseWriter) {
	vary := w.Header().Values("Vary")
	for _, value := range vary {
		for _, part := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(part), "Origin") {
				return
			}
		}
	}
	w.Header().Add("Vary", "Origin")
}

func joinHeaderValues(values []string) string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return strings.Join(out, ", ")
}
