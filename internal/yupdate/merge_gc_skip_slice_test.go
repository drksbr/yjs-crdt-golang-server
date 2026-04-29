package yupdate

import (
	"testing"

	"github.com/drksbr/yjs-crdt-golang-server/internal/ytypes"
)

func TestMergeUpdatesV1SlicesOverlappingGC(t *testing.T) {
	t.Parallel()

	left := buildUpdate(
		clientBlock{
			client: 21,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "ab"),
			},
		},
	)
	right := buildUpdate(
		clientBlock{
			client: 21,
			clock:  1,
			structs: []structEncoding{
				gc(2),
				itemString(rootParent("doc"), "z"),
			},
		},
	)

	merged, err := MergeUpdatesV1(left, right)
	if err != nil {
		t.Fatalf("MergeUpdatesV1() unexpected error: %v", err)
	}

	decoded, err := DecodeV1(merged)
	if err != nil {
		t.Fatalf("DecodeV1(merged) unexpected error: %v", err)
	}

	if len(decoded.Structs) != 3 {
		t.Fatalf("len(Structs) = %d, want 3; structs = %#v", len(decoded.Structs), decoded.Structs)
	}

	if item, ok := decoded.Structs[0].(*ytypes.Item); !ok {
		t.Fatalf("Structs[0] type = %T, want *ytypes.Item", decoded.Structs[0])
	} else if item.ID().Clock != 0 || item.Length() != 2 {
		t.Fatalf("Structs[0] = %#v, want item at clock 0 len 2", item)
	}

	if gcStruct, ok := decoded.Structs[1].(ytypes.GC); !ok {
		t.Fatalf("Structs[1] type = %T, want ytypes.GC", decoded.Structs[1])
	} else if gcStruct.ID().Clock != 2 || gcStruct.Length() != 1 {
		t.Fatalf("Structs[1] = %#v, want sliced GC at clock 2 len 1", gcStruct)
	}

	if item, ok := decoded.Structs[2].(*ytypes.Item); !ok {
		t.Fatalf("Structs[2] type = %T, want *ytypes.Item", decoded.Structs[2])
	} else if item.ID().Clock != 3 || item.Length() != 1 {
		t.Fatalf("Structs[2] = %#v, want item at clock 3 len 1", item)
	}
}
