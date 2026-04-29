package yupdate

import (
	"bytes"
	"testing"

	"github.com/drksbr/yjs-crdt-golang-server/internal/ytypes"
)

func TestMergeUpdatesV1PartiallyFillsSyntheticSkipAndIsPermutationStable(t *testing.T) {
	t.Parallel()

	left := buildUpdate(
		clientBlock{
			client: 31,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "ab"),
			},
		},
	)
	fill := buildUpdate(
		clientBlock{
			client: 31,
			clock:  3,
			structs: []structEncoding{
				itemString(rootParent("doc"), "d"),
			},
		},
	)
	right := buildUpdate(
		clientBlock{
			client: 31,
			clock:  5,
			structs: []structEncoding{
				itemString(rootParent("doc"), "ef"),
			},
		},
	)

	want := buildUpdate(
		clientBlock{
			client: 31,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "ab"),
				skip(1),
				itemString(rootParent("doc"), "d"),
				skip(1),
				itemString(rootParent("doc"), "ef"),
			},
		},
	)

	cases := []struct {
		name    string
		updates [][]byte
	}{
		{
			name:    "left_fill_right",
			updates: [][]byte{left, fill, right},
		},
		{
			name:    "left_right_fill",
			updates: [][]byte{left, right, fill},
		},
		{
			name:    "fill_left_right",
			updates: [][]byte{fill, left, right},
		},
		{
			name:    "fill_right_left",
			updates: [][]byte{fill, right, left},
		},
		{
			name:    "right_left_fill",
			updates: [][]byte{right, left, fill},
		},
		{
			name:    "right_fill_left",
			updates: [][]byte{right, fill, left},
		},
	}

	for _, tt := range cases {
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

			if len(decoded.Structs) != 5 {
				t.Fatalf("len(Structs) = %d, want 5", len(decoded.Structs))
			}

			assertStringStruct(t, decoded.Structs[0], 31, 0, "ab")

			firstSkip, ok := decoded.Structs[1].(ytypes.Skip)
			if !ok {
				t.Fatalf("Structs[1] type = %T, want ytypes.Skip", decoded.Structs[1])
			}
			if firstSkip.ID().Clock != 2 || firstSkip.Length() != 1 {
				t.Fatalf("Structs[1] = %#v, want Skip at clock 2 len 1", firstSkip)
			}

			assertStringStruct(t, decoded.Structs[2], 31, 3, "d")

			secondSkip, ok := decoded.Structs[3].(ytypes.Skip)
			if !ok {
				t.Fatalf("Structs[3] type = %T, want ytypes.Skip", decoded.Structs[3])
			}
			if secondSkip.ID().Clock != 4 || secondSkip.Length() != 1 {
				t.Fatalf("Structs[3] = %#v, want Skip at clock 4 len 1", secondSkip)
			}

			assertStringStruct(t, decoded.Structs[4], 31, 5, "ef")
		})
	}
}
