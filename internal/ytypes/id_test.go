package ytypes

import (
	"errors"
	"math"
	"testing"

	ybinary "github.com/drksbr/yjs-crdt-golang-server/internal/binary"
	"github.com/drksbr/yjs-crdt-golang-server/internal/varint"
)

func TestIDEqual(t *testing.T) {
	t.Parallel()

	left := ID{Client: 7, Clock: 10}

	if !left.Equal(ID{Client: 7, Clock: 10}) {
		t.Fatalf("Equal() = false, want true")
	}

	if left.Equal(ID{Client: 8, Clock: 10}) {
		t.Fatalf("Equal() = true, want false")
	}
}

func TestIDOffset(t *testing.T) {
	t.Parallel()

	got, err := ID{Client: 5, Clock: 41}.Offset(9)
	if err != nil {
		t.Fatalf("Offset() unexpected error: %v", err)
	}

	want := ID{Client: 5, Clock: 50}
	if got != want {
		t.Fatalf("Offset() = %+v, want %+v", got, want)
	}
}

func TestIDOffsetRejectsOverflow(t *testing.T) {
	t.Parallel()

	_, err := ID{Client: 1, Clock: math.MaxUint32}.Offset(1)
	if !errors.Is(err, ErrStructOverflow) {
		t.Fatalf("Offset() error = %v, want ErrStructOverflow", err)
	}
}

func TestIDEncodeDecodeRoundTrip(t *testing.T) {
	t.Parallel()

	want := ID{Client: 300, Clock: 16384}
	encoded := AppendID(nil, want)

	got, n, err := DecodeID(encoded)
	if err != nil {
		t.Fatalf("DecodeID() unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("DecodeID() = %+v, want %+v", got, want)
	}
	if n != len(encoded) {
		t.Fatalf("DecodeID() consumed = %d, want %d", n, len(encoded))
	}
}

func TestReadIDWithBinaryReader(t *testing.T) {
	t.Parallel()

	want := ID{Client: 7, Clock: 42}
	reader := ybinary.NewReader(AppendID(nil, want))

	got, n, err := ReadID(reader)
	if err != nil {
		t.Fatalf("ReadID() unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("ReadID() = %+v, want %+v", got, want)
	}
	if n != reader.Offset() {
		t.Fatalf("ReadID() consumed = %d, reader offset = %d", n, reader.Offset())
	}
}

func TestReadIDPropagatesVarintErrors(t *testing.T) {
	t.Parallel()

	reader := ybinary.NewReader([]byte{0x80})
	_, _, err := ReadID(reader)
	if !errors.Is(err, varint.ErrUnexpectedEOF) {
		t.Fatalf("ReadID() error = %v, want ErrUnexpectedEOF", err)
	}
}
