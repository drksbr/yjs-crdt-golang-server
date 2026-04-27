package ytypes

// Kind classifica as structs mínimas suportadas neste estágio do port.
type Kind uint8

const (
	KindGC Kind = iota
	KindItem
	KindSkip
)

func (k Kind) String() string {
	switch k {
	case KindGC:
		return "gc"
	case KindItem:
		return "item"
	case KindSkip:
		return "skip"
	default:
		return "unknown"
	}
}

// Struct define a superfície mínima comum para iteração sobre updates.
// O parser futuro pode tratar `Item`, `GC` e `Skip` de forma uniforme sem
// conhecer detalhes do conteúdo de cada caso.
type Struct interface {
	Kind() Kind
	ID() ID
	Length() uint32
	Deleted() bool
	EndClock() uint32
	LastID() ID
	ContainsClock(clock uint32) bool
}

type baseStruct struct {
	id     ID
	length uint32
}

func newBaseStruct(id ID, length uint32) (baseStruct, error) {
	if length == 0 {
		return baseStruct{}, ErrInvalidLength
	}
	if _, err := addClock(id.Clock, length); err != nil {
		return baseStruct{}, err
	}

	return baseStruct{
		id:     id,
		length: length,
	}, nil
}

// ID retorna o primeiro endereço lógico coberto pela struct.
func (s baseStruct) ID() ID {
	return s.id
}

// Length retorna o número de clocks cobertos pela struct.
func (s baseStruct) Length() uint32 {
	return s.length
}

// EndClock retorna o clock exclusivo do final da struct.
func (s baseStruct) EndClock() uint32 {
	end, err := s.checkedEndClock()
	if err != nil {
		return s.id.Clock
	}
	return end
}

// LastID retorna o último endereço lógico pertencente à struct.
func (s baseStruct) LastID() ID {
	end, err := s.checkedEndClock()
	if err != nil || end == s.id.Clock {
		return s.id
	}
	return ID{
		Client: s.id.Client,
		Clock:  end - 1,
	}
}

// ContainsClock informa se clock está dentro do range da struct.
func (s baseStruct) ContainsClock(clock uint32) bool {
	end, err := s.checkedEndClock()
	return err == nil && clock >= s.id.Clock && clock < end
}

func (s baseStruct) checkedEndClock() (uint32, error) {
	if s.length == 0 {
		return 0, ErrInvalidLength
	}

	end, err := addClock(s.id.Clock, s.length)
	if err != nil {
		return 0, err
	}

	return end, nil
}

// GC modela uma struct já coletada. No Yjs ela é sempre tratada como deletada.
type GC struct {
	baseStruct
}

// NewGC valida e constrói uma GC mínima.
func NewGC(id ID, length uint32) (GC, error) {
	base, err := newBaseStruct(id, length)
	if err != nil {
		return GC{}, err
	}
	return GC{baseStruct: base}, nil
}

// Kind identifica a categoria da struct.
func (GC) Kind() Kind {
	return KindGC
}

// Deleted reflete a semântica do Yjs: GC representa conteúdo removido.
func (GC) Deleted() bool {
	return true
}

// Skip modela lacunas temporárias usadas durante leitura/merge de updates.
type Skip struct {
	baseStruct
}

// NewSkip valida e constrói uma Skip mínima.
func NewSkip(id ID, length uint32) (Skip, error) {
	base, err := newBaseStruct(id, length)
	if err != nil {
		return Skip{}, err
	}
	return Skip{baseStruct: base}, nil
}

// Kind identifica a categoria da struct.
func (Skip) Kind() Kind {
	return KindSkip
}

// Deleted em Skip é sempre falso, pois a struct só reserva clocks.
func (Skip) Deleted() bool {
	return false
}
