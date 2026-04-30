package yhttp

import (
	"fmt"
	"reflect"
)

// ValidateProductionServerConfig valida se um ServerConfig atende os
// requisitos minimos de wiring e seguranca para exposicao em producao.
func ValidateProductionServerConfig(cfg ServerConfig) error {
	if cfg.Provider == nil {
		return ErrNilProvider
	}
	if cfg.ResolveRequest == nil {
		return ErrNilResolveRequest
	}
	return validateProductionServerHooks("ServerConfig", cfg.Authenticator, cfg.Authorizer, cfg.RateLimiter, cfg.QuotaLimiter, cfg.OriginPolicy, cfg.Redactor)
}

// ValidateProductionOwnerAwareConfig valida se um OwnerAwareServerConfig atende
// os requisitos minimos de wiring e seguranca para exposicao em producao.
func ValidateProductionOwnerAwareConfig(cfg OwnerAwareServerConfig) error {
	if cfg.Local == nil {
		return ErrNilLocalServer
	}
	if cfg.OwnerLookup == nil {
		return ErrNilOwnerLookup
	}
	return validateProductionServerInstance("OwnerAwareServerConfig.Local", cfg.Local)
}

// ValidateProductionRemoteOwnerEndpointConfig valida se um
// RemoteOwnerEndpointConfig atende os requisitos minimos de wiring e seguranca
// para uso owner-side em producao.
func ValidateProductionRemoteOwnerEndpointConfig(cfg RemoteOwnerEndpointConfig) error {
	if cfg.Local == nil {
		return ErrNilRemoteOwnerEndpoint
	}
	if err := cfg.LocalNodeID.Validate(); err != nil {
		return err
	}
	if cfg.Authenticate == nil {
		return productionRequirementError("RemoteOwnerEndpointConfig.Authenticate")
	}
	return validateProductionServerInstance("RemoteOwnerEndpointConfig.Local", cfg.Local)
}

func validateProductionServerInstance(fieldPrefix string, srv *Server) error {
	if srv.provider == nil {
		return wrapProductionFieldError(ErrNilProvider, fieldPrefix+".Provider")
	}
	if srv.resolveRequest == nil {
		return wrapProductionFieldError(ErrNilResolveRequest, fieldPrefix+".ResolveRequest")
	}
	return validateProductionServerHooks(
		fieldPrefix,
		srv.authenticator,
		srv.authorizer,
		srv.rateLimiter,
		srv.quotaLimiter,
		srv.originPolicy,
		srv.redactor,
	)
}

func validateProductionServerHooks(
	fieldPrefix string,
	authenticator Authenticator,
	authorizer Authorizer,
	rateLimiter RateLimiter,
	quotaLimiter QuotaLimiter,
	originPolicy OriginPolicy,
	redactor RequestRedactor,
) error {
	if isNilHook(authenticator) {
		return productionRequirementError(fieldPrefix + ".Authenticator")
	}
	if isNilHook(authorizer) {
		return productionRequirementError(fieldPrefix + ".Authorizer")
	}
	if isNilHook(rateLimiter) {
		return productionRequirementError(fieldPrefix + ".RateLimiter")
	}
	if isNilHook(quotaLimiter) {
		return productionRequirementError(fieldPrefix + ".QuotaLimiter")
	}
	if isNilHook(originPolicy) {
		return productionRequirementError(fieldPrefix + ".OriginPolicy")
	}
	if isNilHook(redactor) {
		return productionRequirementError(fieldPrefix + ".Redactor")
	}
	return nil
}

func productionRequirementError(field string) error {
	return fmt.Errorf("%w: %s obrigatorio em producao", ErrForbidden, field)
}

func wrapProductionFieldError(err error, field string) error {
	return fmt.Errorf("%w: %s", err, field)
}

func isNilHook(hook any) bool {
	if hook == nil {
		return true
	}

	value := reflect.ValueOf(hook)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
