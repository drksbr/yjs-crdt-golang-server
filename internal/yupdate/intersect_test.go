package yupdate

import (
	"testing"

	"yjs-go-bridge/internal/ytypes"
)

func TestIntersectUpdateWithContentIDsV1SelectsMiddleStringSlice(t *testing.T) {
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
		deleteRange{client: 2, clock: 9, length: 2},
	)
	ids := NewContentIDs()
	_ = ids.Inserts.Add(2, 1, 3)
	_ = ids.Deletes.Add(2, 10, 1)

	got, err := IntersectUpdateWithContentIDsV1(update, ids)
	if err != nil {
		t.Fatalf("IntersectUpdateWithContentIDsV1() unexpected error: %v", err)
	}

	decoded, err := DecodeV1(got)
	if err != nil {
		t.Fatalf("DecodeV1(intersection) unexpected error: %v", err)
	}

	if len(decoded.Structs) != 1 {
		t.Fatalf("len(Structs) = %d, want 1", len(decoded.Structs))
	}
	item, ok := decoded.Structs[0].(*ytypes.Item)
	if !ok {
		t.Fatalf("Structs[0] type = %T, want *ytypes.Item", decoded.Structs[0])
	}
	content := item.Content.(ParsedContent)
	if item.ID().Clock != 1 || content.Text != "ell" || content.Length() != 3 {
		t.Fatalf("intersection item = id=%+v content=%#v, want clock=1 text=ell len=3", item.ID(), content)
	}
	if !decoded.DeleteSet.Has(ytypes.ID{Client: 2, Clock: 10}) || decoded.DeleteSet.Has(ytypes.ID{Client: 2, Clock: 9}) {
		t.Fatalf("DeleteSet = %#v, want only clock 10", decoded.DeleteSet)
	}
}

func TestIntersectUpdateWithContentIDsV1EmitsSyntheticSkipAfterFirstWrite(t *testing.T) {
	t.Parallel()

	update := buildUpdate(
		clientBlock{
			client: 7,
			clock:  0,
			structs: []structEncoding{
				itemDeleted(rootParent("doc"), 2),
				itemDeleted(rootParent("doc"), 2),
			},
		},
	)
	ids := NewContentIDs()
	_ = ids.Inserts.Add(7, 0, 1)
	_ = ids.Inserts.Add(7, 2, 1)

	got, err := IntersectUpdateWithContentIDsV1(update, ids)
	if err != nil {
		t.Fatalf("IntersectUpdateWithContentIDsV1() unexpected error: %v", err)
	}

	decoded, err := DecodeV1(got)
	if err != nil {
		t.Fatalf("DecodeV1(intersection) unexpected error: %v", err)
	}

	if len(decoded.Structs) != 3 {
		t.Fatalf("len(Structs) = %d, want 3", len(decoded.Structs))
	}
	if decoded.Structs[0].ID().Clock != 0 || decoded.Structs[0].Length() != 1 {
		t.Fatalf("Structs[0] = %#v, want first slice at clock 0 len 1", decoded.Structs[0])
	}
	if decoded.Structs[1].Kind() != ytypes.KindSkip || decoded.Structs[1].ID().Clock != 1 || decoded.Structs[1].Length() != 1 {
		t.Fatalf("Structs[1] = %#v, want synthetic skip at clock 1 len 1", decoded.Structs[1])
	}
	if decoded.Structs[2].ID().Clock != 2 || decoded.Structs[2].Length() != 1 {
		t.Fatalf("Structs[2] = %#v, want second slice at clock 2 len 1", decoded.Structs[2])
	}
}

func TestIntersectUpdateWithContentIDsV1MatchesUpstreamOnLaterStructSelection(t *testing.T) {
	t.Parallel()

	update := buildUpdate(
		clientBlock{
			client: 9,
			clock:  0,
			structs: []structEncoding{
				itemDeleted(rootParent("doc"), 2),
				itemDeleted(rootParent("doc"), 2),
			},
		},
	)
	ids := NewContentIDs()
	_ = ids.Inserts.Add(9, 2, 2)

	got, err := IntersectUpdateWithContentIDsV1(update, ids)
	if err != nil {
		t.Fatalf("IntersectUpdateWithContentIDsV1() unexpected error: %v", err)
	}

	decoded, err := DecodeV1(got)
	if err != nil {
		t.Fatalf("DecodeV1(intersection) unexpected error: %v", err)
	}

	if len(decoded.Structs) != 0 {
		t.Fatalf("len(Structs) = %d, want 0 to match upstream sparse-selection behavior", len(decoded.Structs))
	}
}

func TestIntersectUpdateWithContentIDsV1AllowsContiguousClientRangesWithoutGap(t *testing.T) {
	t.Parallel()

	update := buildUpdate(
		clientBlock{
			client: 12,
			clock:  0,
			structs: []structEncoding{
				itemDeleted(rootParent("doc"), 1),
				itemDeleted(rootParent("doc"), 1),
				itemDeleted(rootParent("doc"), 1),
			},
		},
	)
	ids := NewContentIDs()
	_ = ids.Inserts.Add(12, 0, 1)
	_ = ids.Inserts.Add(12, 1, 2)

	got, err := IntersectUpdateWithContentIDsV1(update, ids)
	if err != nil {
		t.Fatalf("IntersectUpdateWithContentIDsV1() unexpected error: %v", err)
	}

	decoded, err := DecodeV1(got)
	if err != nil {
		t.Fatalf("DecodeV1(intersection) unexpected error: %v", err)
	}

	if len(decoded.Structs) != 3 {
		t.Fatalf("len(Structs) = %d, want 3", len(decoded.Structs))
	}
	for i, expectedClock := range []uint32{0, 1, 2} {
		if decoded.Structs[i].Kind() == ytypes.KindSkip {
			t.Fatalf("Structs[%d] is skip, want delete struct", i)
		}
		if decoded.Structs[i].ID().Clock != expectedClock {
			t.Fatalf("Structs[%d].ID.Clock = %d, want %d", i, decoded.Structs[i].ID().Clock, expectedClock)
		}
	}
}

func TestIntersectUpdateWithContentIDsV1HandlesPartiallyContiguousStructsWithDuplicateRanges(t *testing.T) {
	t.Parallel()

	update := buildUpdate(
		clientBlock{
			client: 18,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "ab"),
				itemString(rootParent("doc"), "cd"),
			},
		},
	)
	ids := NewContentIDs()
	_ = ids.Inserts.Add(18, 1, 3)
	_ = ids.Inserts.Add(18, 1, 3)

	got, err := IntersectUpdateWithContentIDsV1(update, ids)
	if err != nil {
		t.Fatalf("IntersectUpdateWithContentIDsV1() unexpected error: %v", err)
	}

	decoded, err := DecodeV1(got)
	if err != nil {
		t.Fatalf("DecodeV1(intersection) unexpected error: %v", err)
	}

	if len(decoded.Structs) != 2 {
		t.Fatalf("len(Structs) = %d, want 2", len(decoded.Structs))
	}

	first, ok := decoded.Structs[0].(*ytypes.Item)
	if !ok {
		t.Fatalf("Structs[0] type = %T, want *ytypes.Item", decoded.Structs[0])
	}
	second, ok := decoded.Structs[1].(*ytypes.Item)
	if !ok {
		t.Fatalf("Structs[1] type = %T, want *ytypes.Item", decoded.Structs[1])
	}
	if first.ID().Clock != 1 || first.Content.(ParsedContent).Text != "b" {
		t.Fatalf("first struct = %#v, want clock=1 text=\"b\"", first)
	}
	if second.ID().Clock != 2 || second.Content.(ParsedContent).Text != "c" {
		t.Fatalf("second struct = %#v, want clock=2 text=\"c\"", second)
	}
	if second.ID().Clock-first.ID().Clock != 1 {
		t.Fatalf("partially contiguous writes were split with a gap, want clock delta 1, got %d", second.ID().Clock-first.ID().Clock)
	}
}

func TestIntersectUpdateWithContentIDsV1KeepsOnlyMatchingDeletesWhenNoInserts(t *testing.T) {
	t.Parallel()

	update := buildUpdate(
		clientBlock{
			client: 30,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "abc"),
			},
		},
		deleteRange{client: 30, clock: 10, length: 2},
		deleteRange{client: 31, clock: 5, length: 1},
	)
	ids := NewContentIDs()
	_ = ids.Deletes.Add(30, 11, 1)
	_ = ids.Deletes.Add(31, 7, 1)

	got, err := IntersectUpdateWithContentIDsV1(update, ids)
	if err != nil {
		t.Fatalf("IntersectUpdateWithContentIDsV1() unexpected error: %v", err)
	}

	decoded, err := DecodeV1(got)
	if err != nil {
		t.Fatalf("DecodeV1(intersection) unexpected error: %v", err)
	}

	if len(decoded.Structs) != 0 {
		t.Fatalf("len(Structs) = %d, want 0", len(decoded.Structs))
	}
	if !decoded.DeleteSet.Has(ytypes.ID{Client: 30, Clock: 11}) {
		t.Fatalf("DeleteSet = %#v, want only clock 11 for client 30", decoded.DeleteSet)
	}
	if decoded.DeleteSet.Has(ytypes.ID{Client: 30, Clock: 10}) {
		t.Fatalf("DeleteSet = %#v, want to exclude clock 10", decoded.DeleteSet)
	}
	if decoded.DeleteSet.Has(ytypes.ID{Client: 31, Clock: 5}) {
		t.Fatalf("DeleteSet = %#v, want to exclude client 31", decoded.DeleteSet)
	}
}
