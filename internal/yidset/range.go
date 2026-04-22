package yidset

// Range representa um intervalo semântico [clock, clock+length) para um cliente.
// A forma interna sempre é normalizada: length > 0 e sem extrapolar a faixa
// de clocks uint32 compatível com o decoder atual.
type Range struct {
	Clock  uint32
	Length uint32
}

// NewRange valida e constrói um range.
//
// Decisão de compatibilidade:
// o Yjs tolera ranges vazios em alguns estados intermediários e os descarta ao
// normalizar. Nesta API, ranges vazios são rejeitados na borda pública para
// manter invariantes simples.
func NewRange(clock, length uint32) (Range, error) {
	if length == 0 {
		return Range{}, ErrInvalidLength
	}
	if _, ok := rangeEnd(clock, length); !ok {
		return Range{}, ErrRangeOverflow
	}
	return Range{Clock: clock, Length: length}, nil
}

// End retorna o limite exclusivo do range.
func (r Range) End() uint64 {
	return uint64(r.Clock) + uint64(r.Length)
}

// Contains informa se clock pertence ao intervalo.
func (r Range) Contains(clock uint32) bool {
	return uint64(clock) >= uint64(r.Clock) && uint64(clock) < r.End()
}

// CanMerge informa se os ranges são adjacentes ou sobrepostos.
func (r Range) CanMerge(other Range) bool {
	return r.End() >= uint64(other.Clock) && other.End() >= uint64(r.Clock)
}

func mergeRanges(left, right Range) Range {
	start := left.Clock
	if right.Clock < start {
		start = right.Clock
	}

	end := left.End()
	if right.End() > end {
		end = right.End()
	}

	return Range{
		Clock:  start,
		Length: uint32(end - uint64(start)),
	}
}

func rangeEnd(clock, length uint32) (uint64, bool) {
	end := uint64(clock) + uint64(length)
	if end > 1<<32 {
		return 0, false
	}
	return end, true
}
