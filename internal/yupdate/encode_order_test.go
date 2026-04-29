package yupdate

import (
	"bytes"
	"testing"

	"github.com/drksbr/yjs-crdt-golang-server/internal/ytypes"
)

func TestEncodeV1NormalizesOutOfOrderStructsWithinSameClient(t *testing.T) {
	t.Parallel()

	update := buildUpdate(
		clientBlock{
			client: 9,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "ab"),
				gc(1),
				itemDeleted(rootParent("doc"), 2),
			},
		},
		clientBlock{
			client: 2,
			clock:  0,
			structs: []structEncoding{
				itemJSON(rootParent("doc"), `"x"`),
			},
		},
		deleteRange{client: 9, clock: 10, length: 1},
	)

	decoded, err := DecodeV1(update)
	if err != nil {
		t.Fatalf("DecodeV1() unexpected error: %v", err)
	}

	outOfOrder := &DecodedUpdate{
		Structs: []ytypes.Struct{
			decoded.Structs[2],
			decoded.Structs[3],
			decoded.Structs[0],
			decoded.Structs[1],
		},
		DeleteSet: decoded.DeleteSet.Clone(),
	}

	encoded, err := EncodeV1(outOfOrder)
	if err != nil {
		t.Fatalf("EncodeV1() unexpected error: %v", err)
	}

	if !bytes.Equal(encoded, update) {
		t.Fatalf("EncodeV1(outOfOrder) = %v, want canonical %v", encoded, update)
	}

	roundTrip, err := DecodeV1(encoded)
	if err != nil {
		t.Fatalf("DecodeV1(EncodeV1(outOfOrder)) unexpected error: %v", err)
	}

	if len(roundTrip.Structs) != 4 {
		t.Fatalf("len(Structs) = %d, want 4", len(roundTrip.Structs))
	}

	if roundTrip.Structs[0].ID() != (ytypes.ID{Client: 9, Clock: 0}) || roundTrip.Structs[0].Length() != 2 {
		t.Fatalf("Structs[0] = %#v, want first client-9 item at clock 0 len 2", roundTrip.Structs[0])
	}
	if roundTrip.Structs[1].ID() != (ytypes.ID{Client: 9, Clock: 2}) || roundTrip.Structs[1].Length() != 1 {
		t.Fatalf("Structs[1] = %#v, want client-9 GC at clock 2 len 1", roundTrip.Structs[1])
	}
	if roundTrip.Structs[2].ID() != (ytypes.ID{Client: 9, Clock: 3}) || roundTrip.Structs[2].Length() != 2 {
		t.Fatalf("Structs[2] = %#v, want client-9 deleted item at clock 3 len 2", roundTrip.Structs[2])
	}
	if roundTrip.Structs[3].ID() != (ytypes.ID{Client: 2, Clock: 0}) || roundTrip.Structs[3].Length() != 1 {
		t.Fatalf("Structs[3] = %#v, want client-2 JSON item at clock 0 len 1", roundTrip.Structs[3])
	}
}
