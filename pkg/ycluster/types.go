package ycluster

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
)

// NodeID identifica de forma estavel um no participante do cluster.
type NodeID string

// String materializa o identificador textual do no.
func (id NodeID) String() string {
	return string(id)
}

// Validate confirma se o identificador do no pode ser usado em contratos
// publicos do control plane.
func (id NodeID) Validate() error {
	if strings.TrimSpace(string(id)) == "" {
		return ErrInvalidNodeID
	}
	return nil
}

// ShardID identifica um shard logico dentro do espaco do cluster.
type ShardID uint32

// String retorna a representacao decimal do shard.
func (id ShardID) String() string {
	return strconv.FormatUint(uint64(id), 10)
}

// Placement representa o owner atual de um shard, opcionalmente acompanhado
// do lease ativo que sustenta esse ownership.
type Placement struct {
	ShardID ShardID
	NodeID  NodeID
	Lease   *Lease
	Version uint64
}

// Validate confirma se o placement esta completo e consistente.
func (p Placement) Validate() error {
	if err := p.NodeID.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidPlacement, err)
	}
	if p.Lease == nil {
		return nil
	}
	if err := p.Lease.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidPlacement, err)
	}
	if p.Lease.ShardID != p.ShardID {
		return fmt.Errorf("%w: lease shard %s difere do placement %s", ErrInvalidPlacement, p.Lease.ShardID, p.ShardID)
	}
	if p.Lease.Holder != p.NodeID {
		return fmt.Errorf("%w: lease holder %q difere do owner %q", ErrInvalidPlacement, p.Lease.Holder, p.NodeID)
	}
	return nil
}

// Lease representa a posse temporaria de um shard por um no.
//
// `Token` funciona como fencing token opaco para renew/release.
type Lease struct {
	ShardID    ShardID
	Holder     NodeID
	Epoch      uint64
	Token      string
	AcquiredAt time.Time
	ExpiresAt  time.Time
}

// Clone retorna uma cópia independente da lease.
func (l *Lease) Clone() *Lease {
	if l == nil {
		return nil
	}

	cloned := *l
	return &cloned
}

// Validate confirma se a lease esta completa e consistente.
func (l Lease) Validate() error {
	if err := l.Holder.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidLease, err)
	}
	if l.Epoch == 0 {
		return fmt.Errorf("%w: epoch obrigatorio", ErrInvalidLease)
	}
	if strings.TrimSpace(l.Token) == "" {
		return fmt.Errorf("%w: token obrigatorio", ErrInvalidLease)
	}
	if l.ExpiresAt.IsZero() {
		return fmt.Errorf("%w: expiresAt obrigatorio", ErrInvalidLease)
	}
	if !l.AcquiredAt.IsZero() && !l.ExpiresAt.After(l.AcquiredAt) {
		return fmt.Errorf("%w: expiresAt deve ser posterior a acquiredAt", ErrInvalidLease)
	}
	return nil
}

// ActiveAt informa se a lease ainda e valida no instante informado.
func (l Lease) ActiveAt(now time.Time) bool {
	if l.ExpiresAt.IsZero() {
		return false
	}
	return now.Before(l.ExpiresAt)
}

// ExpiredAt informa se a lease ja expirou no instante informado.
func (l Lease) ExpiredAt(now time.Time) bool {
	if l.ExpiresAt.IsZero() {
		return false
	}
	return !now.Before(l.ExpiresAt)
}

// LeaseRequest descreve a intencao de acquire ou renew de uma lease.
//
// `Token` pode ficar vazio em acquire inicial e deve ser reaproveitado em renew
// quando o backend exigir fencing explicito.
type LeaseRequest struct {
	ShardID ShardID
	Holder  NodeID
	TTL     time.Duration
	Token   string
}

// Validate confirma se a request contem os campos minimos para aquisicao ou
// renovacao de lease.
func (r LeaseRequest) Validate() error {
	if err := r.Holder.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidLeaseRequest, err)
	}
	if r.TTL <= 0 {
		return fmt.Errorf("%w: ttl obrigatorio", ErrInvalidLeaseRequest)
	}
	return nil
}

// OwnerLookupRequest descreve uma consulta de owner para um documento.
type OwnerLookupRequest struct {
	DocumentKey storage.DocumentKey
}

// Validate confirma se a request contem uma chave de documento valida.
func (r OwnerLookupRequest) Validate() error {
	if err := r.DocumentKey.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidOwnerLookupRequest, err)
	}
	return nil
}

// OwnerResolution representa o resultado materializado de uma consulta de
// ownership para um documento.
type OwnerResolution struct {
	DocumentKey storage.DocumentKey
	Placement   Placement
	Local       bool
}
