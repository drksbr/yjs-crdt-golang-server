package ytypes

import (
	"math"

	"github.com/drksbr/yjs-crdt-golang-server/internal/varint"
)

// ID representa o endereço lógico básico usado pelo Yjs: cliente + clock.
// O clock é contínuo por cliente e endereça uma posição no histórico do autor.
type ID struct {
	Client uint32
	Clock  uint32
}

// Equal informa se dois identificadores apontam para o mesmo endereço lógico.
func (id ID) Equal(other ID) bool {
	return id.Client == other.Client && id.Clock == other.Clock
}

// Offset retorna um novo ID avançado por diff clocks.
// A validação evita que código futuro monte ranges inalcançáveis em uint32.
func (id ID) Offset(diff uint32) (ID, error) {
	clock, err := addClock(id.Clock, diff)
	if err != nil {
		return ID{}, err
	}

	return ID{
		Client: id.Client,
		Clock:  clock,
	}, nil
}

// AppendID codifica o identificador no formato varuint usado pelo Yjs.
func AppendID(dst []byte, id ID) []byte {
	dst = varint.Append(dst, id.Client)
	return varint.Append(dst, id.Clock)
}

// DecodeID lê um identificador de src e informa quantos bytes foram consumidos.
func DecodeID(src []byte) (ID, int, error) {
	client, nClient, err := varint.Decode(src)
	if err != nil {
		return ID{}, nClient, err
	}

	clock, nClock, err := varint.Decode(src[nClient:])
	if err != nil {
		return ID{}, nClient + nClock, err
	}

	return ID{Client: client, Clock: clock}, nClient + nClock, nil
}

// ReadID lê um identificador sequencialmente a partir de um ByteReader.
func ReadID(reader varint.ByteReader) (ID, int, error) {
	client, nClient, err := varint.Read(reader)
	if err != nil {
		return ID{}, nClient, err
	}

	clock, nClock, err := varint.Read(reader)
	if err != nil {
		return ID{}, nClient + nClock, err
	}

	return ID{Client: client, Clock: clock}, nClient + nClock, nil
}

func addClock(clock, delta uint32) (uint32, error) {
	next := uint64(clock) + uint64(delta)
	if next > math.MaxUint32 {
		return 0, ErrStructOverflow
	}
	return uint32(next), nil
}
