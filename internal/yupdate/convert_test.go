package yupdate

import (
	"bytes"
	"errors"
	"testing"

	"github.com/drksbr/yjs-crdt-golang-server/internal/varint"
)

func TestConvertUpdateToV1(t *testing.T) {
	t.Parallel()

	v1 := buildUpdate(
		clientBlock{
			client: 6,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "ab"),
				gc(1),
			},
		},
		deleteRange{client: 6, clock: 10, length: 1},
	)
	v2 := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	tests := []struct {
		name    string
		input   []byte
		want    []byte
		wantErr error
	}{
		{
			name:  "v1_round_trip",
			input: v1,
			want:  v1,
		},
		{
			name:  "empty_v1_update",
			input: encodeEmptyUpdateV1(),
			want:  encodeEmptyUpdateV1(),
		},
		{
			name:    "v2_rejected",
			input:   v2,
			wantErr: ErrUnsupportedUpdateFormatV2,
		},
		{
			name:    "malformed_payload",
			input:   []byte{0x80},
			wantErr: varint.ErrUnexpectedEOF,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ConvertUpdateToV1(tt.input)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("ConvertUpdateToV1() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("ConvertUpdateToV1() unexpected error: %v", err)
			}
			if !bytes.Equal(got, tt.want) {
				t.Fatalf("ConvertUpdateToV1() = %v, want %v", got, tt.want)
			}
		})
	}
}
