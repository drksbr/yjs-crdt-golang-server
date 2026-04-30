package yhttp

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"
)

// RequestRedactor sanitizes request metadata before it is sent to metrics and
// asynchronous error handlers.
type RequestRedactor interface {
	RedactRequest(req Request) Request
}

// RequestRedactorFunc adapts a function to RequestRedactor.
type RequestRedactorFunc func(req Request) Request

// RedactRequest redacts a request by calling the wrapped function.
func (f RequestRedactorFunc) RedactRequest(req Request) Request {
	return f(req)
}

// HTTPRequestRedactor sanitizes the raw HTTP request passed to error handlers.
type HTTPRequestRedactor interface {
	RedactHTTPRequest(r *http.Request) *http.Request
}

// HashingRequestRedactor replaces high-cardinality/sensitive request fields
// with stable salted hashes suitable for telemetry labels and logs.
type HashingRequestRedactor struct {
	Salt          string
	KeepNamespace bool
	KeepTenant    bool
	KeepScopes    bool
}

// RedactRequest returns a copy of req with sensitive fields hashed or removed.
func (r HashingRequestRedactor) RedactRequest(req Request) Request {
	if !r.KeepNamespace {
		req.DocumentKey.Namespace = redactString(r.Salt, req.DocumentKey.Namespace)
	}
	req.DocumentKey.DocumentID = redactString(r.Salt, req.DocumentKey.DocumentID)
	req.ConnectionID = redactString(r.Salt, req.ConnectionID)
	req.Principal = clonePrincipal(req.Principal)
	if req.Principal != nil {
		req.Principal.Subject = redactString(r.Salt, req.Principal.Subject)
		if !r.KeepTenant {
			req.Principal.Tenant = redactString(r.Salt, req.Principal.Tenant)
		}
		if !r.KeepScopes && len(req.Principal.Scopes) > 0 {
			req.Principal.Scopes = []string{"redacted"}
		}
	}
	return req
}

// RedactHTTPRequest returns a clone with sensitive headers and raw query
// removed before the request is passed to error handlers.
func (r HashingRequestRedactor) RedactHTTPRequest(req *http.Request) *http.Request {
	if req == nil {
		return nil
	}
	cloned := req.Clone(req.Context())
	cloned.Header = req.Header.Clone()
	for _, header := range sensitiveHTTPHeaders() {
		cloned.Header.Del(header)
	}
	if cloned.URL != nil {
		urlCopy := *cloned.URL
		if urlCopy.RawQuery != "" {
			urlCopy.RawQuery = "redacted=1"
		}
		cloned.URL = &urlCopy
	}
	cloned.RequestURI = ""
	return cloned
}

func redactRequest(redactor RequestRedactor, req Request) Request {
	if redactor == nil {
		return req
	}
	return redactor.RedactRequest(req)
}

func redactHTTPRequest(redactor RequestRedactor, r *http.Request) *http.Request {
	httpRedactor, ok := redactor.(HTTPRequestRedactor)
	if !ok {
		return r
	}
	return httpRedactor.RedactHTTPRequest(r)
}

func sensitiveHTTPHeaders() []string {
	return []string{
		"Authorization",
		"Cookie",
		"Proxy-Authorization",
		"Set-Cookie",
		RemoteOwnerNodeAuthorizationHeader,
		"X-Api-Key",
		"X-Auth-Token",
	}
}

func redactString(salt string, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(salt + "\x00" + value))
	return "redacted-" + hex.EncodeToString(sum[:8])
}

type redactingMetrics struct {
	base     Metrics
	redactor RequestRedactor
}

func (m redactingMetrics) redacted(req Request) Request {
	return redactRequest(m.redactor, req)
}

func (m redactingMetrics) ConnectionOpened(req Request) {
	m.base.ConnectionOpened(m.redacted(req))
}

func (m redactingMetrics) ConnectionClosed(req Request) {
	m.base.ConnectionClosed(m.redacted(req))
}

func (m redactingMetrics) FrameRead(req Request, bytes int) {
	m.base.FrameRead(m.redacted(req), bytes)
}

func (m redactingMetrics) FrameWritten(req Request, kind string, bytes int) {
	m.base.FrameWritten(m.redacted(req), kind, bytes)
}

func (m redactingMetrics) Handle(req Request, duration time.Duration, err error) {
	m.base.Handle(m.redacted(req), duration, err)
}

func (m redactingMetrics) Persist(req Request, duration time.Duration, err error) {
	m.base.Persist(m.redacted(req), duration, err)
}

func (m redactingMetrics) Error(req Request, stage string, err error) {
	m.base.Error(m.redacted(req), stage, err)
}

func (m redactingMetrics) OwnerLookup(req Request, duration time.Duration, result string) {
	if observer, ok := m.base.(OwnerLookupMetrics); ok {
		observer.OwnerLookup(m.redacted(req), duration, result)
	}
}

func (m redactingMetrics) RouteDecision(req Request, decision string) {
	if observer, ok := m.base.(RouteDecisionMetrics); ok {
		observer.RouteDecision(m.redacted(req), decision)
	}
}

func (m redactingMetrics) RemoteOwnerConnectionOpened(req Request, role string) {
	if observer, ok := m.base.(RemoteOwnerMetrics); ok {
		observer.RemoteOwnerConnectionOpened(m.redacted(req), role)
	}
}

func (m redactingMetrics) RemoteOwnerConnectionClosed(req Request, role string) {
	if observer, ok := m.base.(RemoteOwnerMetrics); ok {
		observer.RemoteOwnerConnectionClosed(m.redacted(req), role)
	}
}

func (m redactingMetrics) RemoteOwnerHandshake(req Request, role string, duration time.Duration, err error) {
	if observer, ok := m.base.(RemoteOwnerMetrics); ok {
		observer.RemoteOwnerHandshake(m.redacted(req), role, duration, err)
	}
}

func (m redactingMetrics) RemoteOwnerMessage(req Request, role string, direction string, kind string) {
	if observer, ok := m.base.(RemoteOwnerMetrics); ok {
		observer.RemoteOwnerMessage(m.redacted(req), role, direction, kind)
	}
}

func (m redactingMetrics) RemoteOwnerClose(req Request, role string, reason string) {
	if observer, ok := m.base.(RemoteOwnerMetrics); ok {
		observer.RemoteOwnerClose(m.redacted(req), role, reason)
	}
}

func (m redactingMetrics) AuthorityRevalidation(req Request, role string, duration time.Duration, err error) {
	if observer, ok := m.base.(AuthorityRevalidationMetrics); ok {
		observer.AuthorityRevalidation(m.redacted(req), role, duration, err)
	}
}

func (m redactingMetrics) OwnershipTransition(req Request, from string, to string, duration time.Duration, err error) {
	if observer, ok := m.base.(OwnershipTransitionMetrics); ok {
		observer.OwnershipTransition(m.redacted(req), from, to, duration, err)
	}
}
