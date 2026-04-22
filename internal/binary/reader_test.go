package binary

import (
	"errors"
	"testing"
)

func TestReaderInitialState(t *testing.T) {
	t.Parallel()

	reader := NewReader([]byte{0x10, 0x20, 0x30})

	if got := reader.Offset(); got != 0 {
		t.Fatalf("Offset() = %d, want 0", got)
	}

	if got := reader.Remaining(); got != 3 {
		t.Fatalf("Remaining() = %d, want 3", got)
	}
}

func TestReaderReadByteAndReadN(t *testing.T) {
	t.Parallel()

	reader := NewReader([]byte{0x10, 0x20, 0x30})

	gotByte, err := reader.ReadByte()
	if err != nil {
		t.Fatalf("ReadByte() unexpected error: %v", err)
	}

	if gotByte != 0x10 {
		t.Fatalf("ReadByte() = 0x%x, want 0x10", gotByte)
	}

	gotBytes, err := reader.ReadN(2)
	if err != nil {
		t.Fatalf("ReadN() unexpected error: %v", err)
	}

	want := []byte{0x20, 0x30}
	if len(gotBytes) != len(want) {
		t.Fatalf("len(ReadN()) = %d, want %d", len(gotBytes), len(want))
	}

	for i := range want {
		if gotBytes[i] != want[i] {
			t.Fatalf("ReadN()[%d] = 0x%x, want 0x%x", i, gotBytes[i], want[i])
		}
	}

	if got := reader.Offset(); got != 3 {
		t.Fatalf("Offset() = %d, want 3", got)
	}

	if got := reader.Remaining(); got != 0 {
		t.Fatalf("Remaining() = %d, want 0", got)
	}
}

func TestReaderReadNZeroDoesNotAdvance(t *testing.T) {
	t.Parallel()

	reader := NewReader([]byte{0x10})

	got, err := reader.ReadN(0)
	if err != nil {
		t.Fatalf("ReadN(0) unexpected error: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("len(ReadN(0)) = %d, want 0", len(got))
	}

	if got := reader.Offset(); got != 0 {
		t.Fatalf("Offset() = %d, want 0", got)
	}
}

func TestReaderReadByteUnexpectedEOF(t *testing.T) {
	t.Parallel()

	reader := NewReader(nil)

	_, err := reader.ReadByte()
	if !errors.Is(err, ErrUnexpectedEOF) {
		t.Fatalf("ReadByte() error = %v, want ErrUnexpectedEOF", err)
	}

	var parseErr *ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("ReadByte() error type = %T, want *ParseError", err)
	}

	if parseErr.Op != "ReadByte" || parseErr.Offset != 0 || parseErr.Need != 1 || parseErr.Remaining != 0 {
		t.Fatalf("ReadByte() parse error = %+v, want op=ReadByte offset=0 need=1 remaining=0", parseErr)
	}

	if got := reader.Offset(); got != 0 {
		t.Fatalf("Offset() after error = %d, want 0", got)
	}
}

func TestReaderReadNUnexpectedEOFDoesNotAdvance(t *testing.T) {
	t.Parallel()

	reader := NewReader([]byte{0x10, 0x20})

	_, err := reader.ReadN(3)
	if !errors.Is(err, ErrUnexpectedEOF) {
		t.Fatalf("ReadN() error = %v, want ErrUnexpectedEOF", err)
	}

	var parseErr *ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("ReadN() error type = %T, want *ParseError", err)
	}

	if parseErr.Op != "ReadN" || parseErr.Offset != 0 || parseErr.Need != 3 || parseErr.Remaining != 2 {
		t.Fatalf("ReadN() parse error = %+v, want op=ReadN offset=0 need=3 remaining=2", parseErr)
	}

	if got := reader.Offset(); got != 0 {
		t.Fatalf("Offset() after error = %d, want 0", got)
	}
}

func TestReaderReadNNegativeSizeDoesNotAdvance(t *testing.T) {
	t.Parallel()

	reader := NewReader([]byte{0x10, 0x20})

	_, err := reader.ReadN(-1)
	if !errors.Is(err, ErrInvalidReadSize) {
		t.Fatalf("ReadN(-1) error = %v, want ErrInvalidReadSize", err)
	}

	var parseErr *ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("ReadN(-1) error type = %T, want *ParseError", err)
	}

	if parseErr.Op != "ReadN" || parseErr.Offset != 0 || parseErr.Size != -1 {
		t.Fatalf("ReadN(-1) parse error = %+v, want op=ReadN offset=0 size=-1", parseErr)
	}

	if got := reader.Offset(); got != 0 {
		t.Fatalf("Offset() after error = %d, want 0", got)
	}
}
