package varint

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	ybinary "github.com/drksbr/yjs-crdt-golang-server/internal/binary"
)

func TestAppendAndDecodeRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value uint32
		want  []byte
	}{
		{name: "zero", value: 0, want: []byte{0x00}},
		{name: "one", value: 1, want: []byte{0x01}},
		{name: "seven_bits", value: 127, want: []byte{0x7f}},
		{name: "two_bytes", value: 128, want: []byte{0x80, 0x01}},
		{name: "mixed_bits", value: 300, want: []byte{0xac, 0x02}},
		{name: "three_bytes", value: 16384, want: []byte{0x80, 0x80, 0x01}},
		{name: "max_uint32", value: 0xffffffff, want: []byte{0xff, 0xff, 0xff, 0xff, 0x0f}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := Append(nil, tt.value)
			if !bytes.Equal(got, tt.want) {
				t.Fatalf("Append() = %v, want %v", got, tt.want)
			}

			value, n, err := Decode(got)
			if err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if value != tt.value {
				t.Fatalf("Decode() value = %d, want %d", value, tt.value)
			}
			if n != len(tt.want) {
				t.Fatalf("Decode() consumed = %d, want %d", n, len(tt.want))
			}
		})
	}
}

func TestReadRoundTripWithByteReader(t *testing.T) {
	t.Parallel()

	tests := []uint32{0, 1, 127, 128, 255, 4096, 0xffffffff}
	for _, want := range tests {
		want := want
		t.Run(stringName(want), func(t *testing.T) {
			t.Parallel()

			reader := bytes.NewReader(Append(nil, want))
			got, n, err := Read(reader)
			if err != nil {
				t.Fatalf("Read() error = %v", err)
			}
			if got != want {
				t.Fatalf("Read() value = %d, want %d", got, want)
			}
			if n != encodedLen(want) {
				t.Fatalf("Read() consumed = %d, want %d", n, encodedLen(want))
			}
		})
	}
}

func TestReadMapsUnexpectedEOFFromBinaryReader(t *testing.T) {
	t.Parallel()

	reader := ybinary.NewReader([]byte{0x80})

	_, n, err := Read(reader)
	if !errors.Is(err, ErrUnexpectedEOF) {
		t.Fatalf("Read() error = %v, want %v", err, ErrUnexpectedEOF)
	}
	if n != 1 {
		t.Fatalf("Read() consumed = %d, want %d", n, 1)
	}
}

func TestDecodeRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   []byte
		wantErr error
		wantN   int
	}{
		{name: "empty", input: nil, wantErr: ErrUnexpectedEOF, wantN: 0},
		{name: "truncated", input: []byte{0x80}, wantErr: ErrUnexpectedEOF, wantN: 1},
		{name: "overflow_high_bits", input: []byte{0xff, 0xff, 0xff, 0xff, 0x10}, wantErr: ErrOverflow, wantN: 5},
		{name: "overflow_continuation", input: []byte{0xff, 0xff, 0xff, 0xff, 0x8f}, wantErr: ErrOverflow, wantN: 5},
		{name: "non_canonical_zero", input: []byte{0x80, 0x00}, wantErr: ErrNonCanonical, wantN: 2},
		{name: "non_canonical_one", input: []byte{0x81, 0x00}, wantErr: ErrNonCanonical, wantN: 2},
		{name: "non_canonical_five_bytes", input: []byte{0x80, 0x80, 0x80, 0x80, 0x00}, wantErr: ErrNonCanonical, wantN: 5},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, n, err := Decode(tt.input)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Decode() error = %v, want %v", err, tt.wantErr)
			}
			if n != tt.wantN {
				t.Fatalf("Decode() consumed = %d, want %d", n, tt.wantN)
			}
		})
	}
}

func TestDecodeConsumesSingleValue(t *testing.T) {
	t.Parallel()

	input := append(Append(nil, 300), 0x7f)
	value, n, err := Decode(input)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if value != 300 {
		t.Fatalf("Decode() value = %d, want %d", value, 300)
	}
	if n != 2 {
		t.Fatalf("Decode() consumed = %d, want %d", n, 2)
	}
}

func stringName(value uint32) string {
	return fmt.Sprintf("value_%d", value)
}
