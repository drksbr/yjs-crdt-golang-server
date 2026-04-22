package yidset

import "slices"

// Set agrupa ranges por client id.
// Cada lista interna fica ordenada por clock e sem ranges adjacentes/duplicados.
type Set struct {
	clients map[uint32][]Range
}

// IdSet preserva o nome usado pelo Yjs.
type IdSet = Set

// New cria um conjunto vazio.
func New() *IdSet {
	return &IdSet{clients: make(map[uint32][]Range)}
}

// Clone cria uma cópia profunda do conjunto.
func (s *IdSet) Clone() *IdSet {
	if s == nil {
		return New()
	}

	cloned := New()
	for client, ranges := range s.clients {
		cloned.clients[client] = slices.Clone(ranges)
	}
	return cloned
}

// IsEmpty informa se não há ranges registrados.
func (s *IdSet) IsEmpty() bool {
	if s == nil {
		return true
	}
	return len(s.clients) == 0
}

// Add insere um range para client, normalizando overlaps e adjacências.
func (s *IdSet) Add(client, clock, length uint32) error {
	if s == nil {
		return nil
	}
	if s.clients == nil {
		s.clients = make(map[uint32][]Range)
	}

	next, err := NewRange(clock, length)
	if err != nil {
		return err
	}

	ranges := s.clients[client]
	if len(ranges) == 0 {
		s.clients[client] = []Range{next}
		return nil
	}

	merged := make([]Range, 0, len(ranges)+1)
	inserted := false

	for _, current := range ranges {
		if current.End() < uint64(next.Clock) {
			merged = append(merged, current)
			continue
		}

		if next.End() < uint64(current.Clock) {
			if !inserted {
				merged = append(merged, next)
				inserted = true
			}
			merged = append(merged, current)
			continue
		}

		next = mergeRanges(next, current)
	}

	if !inserted {
		merged = append(merged, next)
	}

	s.clients[client] = merged
	return nil
}

// InsertIntoIdSet incorpora todos os ranges de src em dest.
func InsertIntoIdSet(dest, src *IdSet) error {
	if dest == nil || src == nil {
		return nil
	}
	return dest.Merge(src)
}

// Merge incorpora os ranges de other neste conjunto.
func (s *IdSet) Merge(other *IdSet) error {
	if s == nil || other == nil {
		return nil
	}

	for client, ranges := range other.clients {
		for _, r := range ranges {
			if err := s.Add(client, r.Clock, r.Length); err != nil {
				return err
			}
		}
	}
	return nil
}

// MergeIdSets cria um novo conjunto contendo a união de todos os sets informados.
func MergeIdSets(sets ...*IdSet) *IdSet {
	merged := New()
	for _, set := range sets {
		_ = InsertIntoIdSet(merged, set)
	}
	return merged
}

// Has informa se client contém o clock consultado.
func (s *IdSet) Has(client, clock uint32) bool {
	if s == nil {
		return false
	}

	ranges := s.clients[client]
	left, right := 0, len(ranges)-1

	for left <= right {
		mid := (left + right) / 2
		current := ranges[mid]
		if current.Contains(clock) {
			return true
		}
		if clock < current.Clock {
			right = mid - 1
		} else {
			left = mid + 1
		}
	}

	return false
}

// Clients retorna os client ids em ordem crescente para iteração determinística.
func (s *IdSet) Clients() []uint32 {
	if s == nil {
		return nil
	}

	clients := make([]uint32, 0, len(s.clients))
	for client := range s.clients {
		clients = append(clients, client)
	}
	slices.Sort(clients)
	return clients
}

// Ranges retorna uma cópia dos ranges associados a client.
func (s *IdSet) Ranges(client uint32) []Range {
	if s == nil {
		return nil
	}
	return slices.Clone(s.clients[client])
}

// ForEach percorre os ranges em ordem determinística por client e clock.
func (s *IdSet) ForEach(fn func(client uint32, r Range)) {
	if s == nil {
		return
	}

	for _, client := range s.Clients() {
		for _, r := range s.clients[client] {
			fn(client, r)
		}
	}
}
