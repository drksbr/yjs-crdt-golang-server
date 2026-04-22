package yupdate

import (
	"bytes"
	"testing"

	"yjs-go-bridge/internal/varint"
	"yjs-go-bridge/internal/ytypes"
)

func TestDecodeStateVectorV1(t *testing.T) {
	t.Parallel()

	sv := encodeStateVectorEntry(1, 3, 9, 12)
	got, err := DecodeStateVectorV1(sv)
	if err != nil {
		t.Fatalf("DecodeStateVectorV1() unexpected error: %v", err)
	}

	if len(got) != 2 || got[1] != 3 || got[9] != 12 {
		t.Fatalf("DecodeStateVectorV1() = %v, want map[1:3 9:12]", got)
	}
}

func TestDiffUpdateV1WithEmptyStateVectorReturnsWholeUpdate(t *testing.T) {
	t.Parallel()

	update := buildUpdate(
		clientBlock{
			client: 5,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "abc"),
				gc(1),
			},
		},
		deleteRange{client: 5, clock: 2, length: 1},
	)

	got, err := DiffUpdateV1(update, encodeStateVectorEntry())
	if err != nil {
		t.Fatalf("DiffUpdateV1() unexpected error: %v", err)
	}

	if !bytes.Equal(got, update) {
		t.Fatalf("DiffUpdateV1() = %v, want %v", got, update)
	}
}

func TestDiffUpdateV1SlicesFirstNewStructAndKeepsRestOfClient(t *testing.T) {
	t.Parallel()

	update := buildUpdate(
		clientBlock{
			client: 2,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "hello"),
				gc(2),
			},
		},
	)

	got, err := DiffUpdateV1(update, encodeStateVectorEntry(2, 3))
	if err != nil {
		t.Fatalf("DiffUpdateV1() unexpected error: %v", err)
	}

	decoded, err := DecodeV1(got)
	if err != nil {
		t.Fatalf("DecodeV1(diff) unexpected error: %v", err)
	}

	if len(decoded.Structs) != 2 {
		t.Fatalf("len(Structs) = %d, want 2", len(decoded.Structs))
	}

	first, ok := decoded.Structs[0].(*ytypes.Item)
	if !ok {
		t.Fatalf("Structs[0] type = %T, want *ytypes.Item", decoded.Structs[0])
	}
	content := first.Content.(ParsedContent)
	if first.ID().Clock != 3 || content.Text != "lo" || content.Length() != 2 {
		t.Fatalf("first diff item = id=%+v content=%#v, want clock=3 text=lo len=2", first.ID(), content)
	}

	if decoded.Structs[1].Kind() != ytypes.KindGC || decoded.Structs[1].ID().Clock != 5 || decoded.Structs[1].Length() != 2 {
		t.Fatalf("Structs[1] = %#v, want GC at clock 5 len 2", decoded.Structs[1])
	}
}

func TestDiffUpdateV1SkipsLeadingSkipAndOnlyEmitsNewTail(t *testing.T) {
	t.Parallel()

	update := buildUpdate(
		clientBlock{
			client: 7,
			clock:  0,
			structs: []structEncoding{
				itemDeleted(rootParent("doc"), 1),
				skip(2),
				itemDeleted(rootParent("doc"), 2),
			},
		},
	)

	got, err := DiffUpdateV1(update, encodeStateVectorEntry(7, 2))
	if err != nil {
		t.Fatalf("DiffUpdateV1() unexpected error: %v", err)
	}

	decoded, err := DecodeV1(got)
	if err != nil {
		t.Fatalf("DecodeV1(diff) unexpected error: %v", err)
	}

	if len(decoded.Structs) != 1 {
		t.Fatalf("len(Structs) = %d, want 1", len(decoded.Structs))
	}
	if decoded.Structs[0].ID().Clock != 3 || decoded.Structs[0].Length() != 2 {
		t.Fatalf("Structs[0] = %#v, want deleted tail at clock 3 len 2", decoded.Structs[0])
	}
}

func TestDiffUpdateV1PreservesDeleteSetWhenNoStructsRemain(t *testing.T) {
	t.Parallel()

	update := buildUpdate(
		clientBlock{
			client: 1,
			clock:  0,
			structs: []structEncoding{
				itemDeleted(rootParent("doc"), 3),
			},
		},
		deleteRange{client: 1, clock: 10, length: 2},
	)

	got, err := DiffUpdateV1(update, encodeStateVectorEntry(1, 3))
	if err != nil {
		t.Fatalf("DiffUpdateV1() unexpected error: %v", err)
	}

	decoded, err := DecodeV1(got)
	if err != nil {
		t.Fatalf("DecodeV1(diff) unexpected error: %v", err)
	}

	if len(decoded.Structs) != 0 {
		t.Fatalf("len(Structs) = %d, want 0", len(decoded.Structs))
	}
	if !decoded.DeleteSet.Has(ytypes.ID{Client: 1, Clock: 10}) || !decoded.DeleteSet.Has(ytypes.ID{Client: 1, Clock: 11}) {
		t.Fatalf("DeleteSet = %#v, want preserved delete set", decoded.DeleteSet)
	}
}

func TestDiffUpdateV1RejectsMalformedStateVector(t *testing.T) {
	t.Parallel()

	_, err := DiffUpdateV1(buildUpdate(), []byte{0x81})
	if err == nil {
		t.Fatalf("DiffUpdateV1() error = nil, want malformed state vector error")
	}
}

func encodeStateVectorEntry(entries ...uint32) []byte {
	out := varint.Append(nil, uint32(len(entries)/2))
	for i := 0; i < len(entries); i += 2 {
		out = varint.Append(out, entries[i])
		out = varint.Append(out, entries[i+1])
	}
	return out
}
