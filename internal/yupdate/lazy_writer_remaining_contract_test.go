package yupdate

import (
	"bytes"
	"testing"

	"github.com/drksbr/yjs-crdt-golang-server/internal/ytypes"
)

func TestLazyWriterV1FinishOnEmptyWriterProducesEmptyUpdate(t *testing.T) {
	t.Parallel()

	writer := newLazyWriterV1()
	got, err := writer.finish(nil)
	if err != nil {
		t.Fatalf("writer.finish() unexpected error: %v", err)
	}

	full := AppendDeleteSetBlockV1(got, ytypes.NewDeleteSet())
	want := encodeEmptyUpdateV1()
	if !bytes.Equal(full, want) {
		t.Fatalf("AppendDeleteSetBlockV1(writer.finish()) = %v, want %v", full, want)
	}

	decoded, err := DecodeV1(full)
	if err != nil {
		t.Fatalf("DecodeV1(writer output) unexpected error: %v", err)
	}
	if len(decoded.Structs) != 0 {
		t.Fatalf("len(Structs) = %d, want 0", len(decoded.Structs))
	}
	if decoded.DeleteSet == nil || !decoded.DeleteSet.IsEmpty() {
		t.Fatalf("DeleteSet = %#v, want empty delete set", decoded.DeleteSet)
	}
}

func TestEncodeV1NormalizesInterleavedDecodedStructsThroughLazyWriter(t *testing.T) {
	t.Parallel()

	update := buildUpdate(
		clientBlock{
			client: 4,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "ab"),
				gc(1),
			},
		},
		clientBlock{
			client: 11,
			clock:  0,
			structs: []structEncoding{
				itemJSON(rootParent("doc"), `"x"`),
			},
		},
		deleteRange{client: 4, clock: 9, length: 1},
	)

	decoded, err := DecodeV1(update)
	if err != nil {
		t.Fatalf("DecodeV1() unexpected error: %v", err)
	}

	interleaved := &DecodedUpdate{
		Structs: []ytypes.Struct{
			decoded.Structs[0],
			decoded.Structs[2],
			decoded.Structs[1],
		},
		DeleteSet: decoded.DeleteSet.Clone(),
	}

	encoded, err := EncodeV1(interleaved)
	if err != nil {
		t.Fatalf("EncodeV1() unexpected error: %v", err)
	}

	roundTrip, err := DecodeV1(encoded)
	if err != nil {
		t.Fatalf("DecodeV1(EncodeV1(decoded)) unexpected error: %v", err)
	}

	assertEncodedV1RoundTripMatches(t, encoded, roundTrip)

	if len(roundTrip.Structs) != 3 {
		t.Fatalf("len(Structs) = %d, want 3", len(roundTrip.Structs))
	}

	first, ok := roundTrip.Structs[0].(*ytypes.Item)
	if !ok {
		t.Fatalf("Structs[0] type = %T, want *ytypes.Item", roundTrip.Structs[0])
	}
	firstContent := first.Content.(ParsedContent)
	if first.ID() != (ytypes.ID{Client: 11, Clock: 0}) || firstContent.ContentRef() != itemContentJSON || len(firstContent.JSON) != 1 || firstContent.JSON[0] != `"x"` {
		t.Fatalf("Structs[0] = id=%+v content=%#v, want client=11 clock=0 JSON [\"x\"]", first.ID(), firstContent)
	}

	second, ok := roundTrip.Structs[1].(*ytypes.Item)
	if !ok {
		t.Fatalf("Structs[1] type = %T, want *ytypes.Item", roundTrip.Structs[1])
	}
	secondContent := second.Content.(ParsedContent)
	if second.ID() != (ytypes.ID{Client: 4, Clock: 0}) || secondContent.ContentRef() != itemContentString || secondContent.Text != "ab" {
		t.Fatalf("Structs[1] = id=%+v content=%#v, want client=4 clock=0 text=ab", second.ID(), secondContent)
	}

	third, ok := roundTrip.Structs[2].(ytypes.GC)
	if !ok {
		t.Fatalf("Structs[2] type = %T, want ytypes.GC", roundTrip.Structs[2])
	}
	if third.ID() != (ytypes.ID{Client: 4, Clock: 2}) || third.Length() != 1 {
		t.Fatalf("Structs[2] = %+v, want GC at client=4 clock=2 len=1", third)
	}

	if !roundTrip.DeleteSet.Has(ytypes.ID{Client: 4, Clock: 9}) {
		t.Fatalf("DeleteSet = %#v, want client 4 clock 9", roundTrip.DeleteSet)
	}
	if roundTrip.DeleteSet.Has(ytypes.ID{Client: 11, Clock: 0}) {
		t.Fatalf("DeleteSet = %#v, want no delete for client 11", roundTrip.DeleteSet)
	}
}
