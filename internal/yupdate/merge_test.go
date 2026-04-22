package yupdate

import (
	"bytes"
	"testing"

	"yjs-go-bridge/internal/ytypes"
)

func TestEncodeV1RoundTripsDecodedUpdate(t *testing.T) {
	t.Parallel()

	update := buildUpdate(
		clientBlock{
			client: 3,
			clock:  0,
			structs: []structEncoding{
				itemDeleted(rootParent("doc"), 2),
				itemString(rootParent("doc"), "abc"),
				itemType(rootParent("doc"), typeRefYXmlElement, "p"),
				itemDoc(rootParent("doc"), "guid-1", appendAnyObject(nil, map[string][]byte{
					"name": appendAnyString(nil, "subdoc"),
				})),
			},
		},
		deleteRange{client: 3, clock: 4, length: 2},
	)

	decoded, err := DecodeV1(update)
	if err != nil {
		t.Fatalf("DecodeV1() unexpected error: %v", err)
	}

	encoded, err := EncodeV1(decoded)
	if err != nil {
		t.Fatalf("EncodeV1() unexpected error: %v", err)
	}

	if !bytes.Equal(encoded, update) {
		t.Fatalf("EncodeV1() = %v, want %v", encoded, update)
	}
}

func TestMergeUpdatesV1Empty(t *testing.T) {
	t.Parallel()

	merged, err := MergeUpdatesV1()
	if err != nil {
		t.Fatalf("MergeUpdatesV1() unexpected error: %v", err)
	}

	if !bytes.Equal(merged, encodeEmptyUpdateV1()) {
		t.Fatalf("MergeUpdatesV1() = %v, want empty update", merged)
	}
}

func TestMergeUpdatesV1DeduplicatesSameUpdate(t *testing.T) {
	t.Parallel()

	update := buildUpdate(
		clientBlock{
			client: 1,
			clock:  0,
			structs: []structEncoding{
				itemDeleted(rootParent("doc"), 3),
			},
		},
		deleteRange{client: 1, clock: 1, length: 1},
	)

	merged, err := MergeUpdatesV1(update, update)
	if err != nil {
		t.Fatalf("MergeUpdatesV1() unexpected error: %v", err)
	}

	if !bytes.Equal(merged, update) {
		t.Fatalf("MergeUpdatesV1() = %v, want identical update", merged)
	}
}

func TestMergeUpdatesV1AddsSkipForGapAndMergesDeleteSet(t *testing.T) {
	t.Parallel()

	left := buildUpdate(
		clientBlock{
			client: 4,
			clock:  0,
			structs: []structEncoding{
				itemDeleted(rootParent("doc"), 2),
			},
		},
		deleteRange{client: 4, clock: 1, length: 1},
	)
	right := buildUpdate(
		clientBlock{
			client: 4,
			clock:  5,
			structs: []structEncoding{
				gc(1),
			},
		},
		deleteRange{client: 4, clock: 7, length: 2},
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
		t.Fatalf("len(Structs) = %d, want 3", len(decoded.Structs))
	}
	if decoded.Structs[1].Kind() != ytypes.KindSkip || decoded.Structs[1].ID().Clock != 2 || decoded.Structs[1].Length() != 3 {
		t.Fatalf("Structs[1] = %#v, want Skip at clock 2 len 3", decoded.Structs[1])
	}
	if !decoded.DeleteSet.Has(ytypes.ID{Client: 4, Clock: 1}) || !decoded.DeleteSet.Has(ytypes.ID{Client: 4, Clock: 8}) {
		t.Fatalf("DeleteSet = %#v, want merged ranges", decoded.DeleteSet)
	}
}

func TestMergeUpdatesV1SlicesOverlappingDeletedStructs(t *testing.T) {
	t.Parallel()

	left := buildUpdate(
		clientBlock{
			client: 9,
			clock:  0,
			structs: []structEncoding{
				itemDeleted(rootParent("doc"), 5),
			},
		},
	)
	right := buildUpdate(
		clientBlock{
			client: 9,
			clock:  3,
			structs: []structEncoding{
				itemDeleted(rootParent("doc"), 4),
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

	if len(decoded.Structs) != 2 {
		t.Fatalf("len(Structs) = %d, want 2", len(decoded.Structs))
	}
	if decoded.Structs[0].ID().Clock != 0 || decoded.Structs[0].Length() != 5 {
		t.Fatalf("Structs[0] = %#v, want first deleted len 5", decoded.Structs[0])
	}
	if decoded.Structs[1].ID().Clock != 5 || decoded.Structs[1].Length() != 2 {
		t.Fatalf("Structs[1] = %#v, want sliced suffix at clock 5 len 2", decoded.Structs[1])
	}
}

func TestMergeUpdatesV1SlicesOverlappingStringStructs(t *testing.T) {
	t.Parallel()

	left := buildUpdate(
		clientBlock{
			client: 2,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "hello"),
			},
		},
	)
	right := buildUpdate(
		clientBlock{
			client: 2,
			clock:  2,
			structs: []structEncoding{
				itemString(rootParent("doc"), "llo!"),
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

	if len(decoded.Structs) != 2 {
		t.Fatalf("len(Structs) = %d, want 2", len(decoded.Structs))
	}
	item, ok := decoded.Structs[1].(*ytypes.Item)
	if !ok {
		t.Fatalf("Structs[1] type = %T, want *ytypes.Item", decoded.Structs[1])
	}
	content := item.Content.(ParsedContent)
	if item.ID().Clock != 5 || content.Text != "!" || content.Length() != 1 {
		t.Fatalf("sliced string = item=%+v content=%#v, want clock 5 text ! len 1", item.ID(), content)
	}
}
