package yupdate

import (
	"bytes"
	"testing"

	"yjs-go-bridge/internal/ytypes"
)

func TestMergeUpdatesV1ContinuesAfterExplicitSkipInsideOverlap(t *testing.T) {
	t.Parallel()

	left := buildUpdate(
		clientBlock{
			client: 80,
			clock:  0,
			structs: []structEncoding{
				gc(2),
				skip(2),
				gc(1),
			},
		},
	)
	right := buildUpdate(
		clientBlock{
			client: 80,
			clock:  1,
			structs: []structEncoding{
				gc(3),
			},
		},
	)
	want := buildUpdate(
		clientBlock{
			client: 80,
			clock:  0,
			structs: []structEncoding{
				gc(2),
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

			clientStructs := structsForClient(decoded, 80)
			if len(clientStructs) != 3 {
				t.Fatalf("client 80 structs = %d, want 3", len(clientStructs))
			}

			if clientStructs[0].Kind() != ytypes.KindGC || clientStructs[0].ID().Clock != 0 || clientStructs[0].Length() != 2 {
				t.Fatalf("client 80 struct[0] = %#v, want GC at clock 0 len 2", clientStructs[0])
			}
			if clientStructs[1].Kind() != ytypes.KindGC || clientStructs[1].ID().Clock != 2 || clientStructs[1].Length() != 2 {
				t.Fatalf("client 80 struct[1] = %#v, want GC at clock 2 len 2", clientStructs[1])
			}
			if clientStructs[2].Kind() != ytypes.KindGC || clientStructs[2].ID().Clock != 4 || clientStructs[2].Length() != 1 {
				t.Fatalf("client 80 struct[2] = %#v, want GC at clock 4 len 1", clientStructs[2])
			}
		})
	}
}
