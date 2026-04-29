package varint

import (
	"errors"
	"io"

	ybinary "github.com/drksbr/yjs-crdt-golang-server/internal/binary"
)

const maxVarUintLen = 5

var (
	// ErrUnexpectedEOF indica que a sequência terminou antes do byte final.
	ErrUnexpectedEOF = errors.New("varint: fim inesperado da entrada")
	// ErrOverflow indica que o valor não cabe na faixa compatível 0..0xffffffff.
	ErrOverflow = errors.New("varint: varuint excede uint32")
	// ErrNonCanonical indica uma codificação válida em bits, mas maior que o necessário.
	ErrNonCanonical = errors.New("varint: codificacao nao canonica")
)

// ByteReader é a forma mínima de integrar a leitura com um leitor binário.
// A intenção é que um futuro *internal/binary.Reader satisfaça esta interface
// sem que o formato precise conhecer detalhes do pacote de leitura.
type ByteReader interface {
	ReadByte() (byte, error)
}

// Append codifica value em formato varuint canônico e acrescenta o resultado em dst.
//
// Decisão de compatibilidade:
// o lib0 aceita escrita ampla em alguns caminhos, mas readVarUint opera como uint32.
// Este pacote fixa ambos os lados em uint32 para preservar o comportamento efetivo
// esperado no ecossistema Yjs e evitar valores que o decoder oficial rejeitaria.
func Append(dst []byte, value uint32) []byte {
	for value >= 0x80 {
		dst = append(dst, byte(value)|0x80)
		value >>= 7
	}
	return append(dst, byte(value))
}

// Read decodifica um varuint canônico a partir de r.
//
// Invariantes:
// - no máximo 5 bytes são aceitos para uint32
// - o quinto byte só pode carregar os 4 bits restantes do valor
// - representações maiores que o necessário são rejeitadas para evitar ambiguidades
func Read(r ByteReader) (uint32, int, error) {
	var value uint32

	for i := 0; i < maxVarUintLen; i++ {
		b, err := r.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, ybinary.ErrUnexpectedEOF) {
				return 0, i, ErrUnexpectedEOF
			}
			return 0, i, err
		}

		n := i + 1
		if i == maxVarUintLen-1 {
			// Em uint32, depois de 4 grupos de 7 bits restam apenas 4 bits úteis.
			if b&0x80 != 0 || b > 0x0f {
				return 0, n, ErrOverflow
			}
		}

		value |= uint32(b&0x7f) << (7 * i)
		if b&0x80 == 0 {
			if encodedLen(value) != n {
				return 0, n, ErrNonCanonical
			}
			return value, n, nil
		}
	}

	return 0, maxVarUintLen, ErrOverflow
}

// Decode decodifica um varuint canônico a partir de src e informa quantos bytes foram consumidos.
func Decode(src []byte) (uint32, int, error) {
	reader := sliceReader{buf: src}
	return Read(&reader)
}

func encodedLen(value uint32) int {
	n := 1
	for value >= 0x80 {
		value >>= 7
		n++
	}
	return n
}

type sliceReader struct {
	buf []byte
	pos int
}

func (r *sliceReader) ReadByte() (byte, error) {
	if r.pos >= len(r.buf) {
		return 0, io.EOF
	}

	b := r.buf[r.pos]
	r.pos++
	return b, nil
}
