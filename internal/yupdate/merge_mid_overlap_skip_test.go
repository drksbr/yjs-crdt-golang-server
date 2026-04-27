package yupdate

import (
	"bytes"
	"testing"

	"yjs-go-bridge/internal/ytypes"
)

func TestMergeUpdatesV1InsertsSyntheticSkipInMidOverlapAndContinuesBothSides(t *testing.T) {
	t.Parallel()

	left := buildUpdate(
		clientBlock{
			client: 70,
			clock:  0,
			structs: []structEncoding{
				gc(2),
			},
		},
		clientBlock{
			client: 70,
			clock:  6,
			structs: []structEncoding{
				gc(2),
			},
		},
	)
	right := buildUpdate(
		clientBlock{
			client: 70,
			clock:  1,
			structs: []structEncoding{
				gc(1),
			},
		},
		clientBlock{
			client: 70,
			clock:  7,
			structs: []structEncoding{
				gc(2),
			},
		},
	)

	want := buildUpdate(
		clientBlock{
			client: 70,
			clock:  0,
			structs: []structEncoding{
				gc(2),
				skip(4),
				gc(2),
				gc(1),
			},
		},
	)

	for _, tt := range []struct {
		name    string
		updates [][]byte
	}{
		{
			name:    "left_right",
			updates: [][]byte{left, right},
		},
		{
			name:    "right_left",
			updates: [][]byte{right, left},
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			merged, err := MergeUpdatesV1(tt.updates...)
			if err != nil {
				t.Fatalf("MergeUpdatesV1() unexpected error: %v", err)
			}

			if !bytes.Equal(merged, want) {
				t.Fatalf("MergeUpdatesV1() = %v, want %v", merged, want)
			}

			decoded, err := DecodeV1(merged)
			if err != nil {
				t.Fatalf("DecodeV1(merged) unexpected error: %v", err)
			}

			if len(decoded.Structs) != 4 {
				t.Fatalf("len(Structs) = %d, want 4", len(decoded.Structs))
			}
			if decoded.Structs[0].Kind() != ytypes.KindGC || decoded.Structs[0].ID().Clock != 0 || decoded.Structs[0].Length() != 2 {
				t.Fatalf("Structs[0] = %#v, want GC at clock 0 len 2", decoded.Structs[0])
			}
			if decoded.Structs[1].Kind() != ytypes.KindSkip || decoded.Structs[1].ID().Clock != 2 || decoded.Structs[1].Length() != 4 {
				t.Fatalf("Structs[1] = %#v, want Skip at clock 2 len 4", decoded.Structs[1])
			}
			if decoded.Structs[2].Kind() != ytypes.KindGC || decoded.Structs[2].ID().Clock != 6 || decoded.Structs[2].Length() != 2 {
				t.Fatalf("Structs[2] = %#v, want GC at clock 6 len 2", decoded.Structs[2])
			}
			if decoded.Structs[3].Kind() != ytypes.KindGC || decoded.Structs[3].ID().Clock != 8 || decoded.Structs[3].Length() != 1 {
				t.Fatalf("Structs[3] = %#v, want GC at clock 8 len 1", decoded.Structs[3])
			}
		})
	}
}
