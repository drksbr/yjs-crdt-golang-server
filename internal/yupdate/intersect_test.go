package yupdate

import (
	"bytes"
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
	_ = ids.Inserts.Add(18, 1, 2)
	_ = ids.Inserts.Add(18, 1, 2)

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

func TestIntersectUpdateWithContentIDsV1SelectsMeta9RefsAcrossGaps(t *testing.T) {
	t.Parallel()

	anyFirst := appendAnyString(nil, "x")
	anySecond := appendAnyString(nil, "y")

	update := buildUpdate(
		clientBlock{
			client: 40,
			clock:  0,
			structs: []structEncoding{
				itemType(rootParent("doc"), typeRefYXmlElement, "p"),
				itemDoc(rootParent("doc"), "guid-meta9", appendAnyString(nil, "subdoc")),
				itemBinary(rootParent("doc"), []byte{0xde, 0xad}),
				itemEmbed(rootParent("doc"), appendAnyObjectFields(nil,
					anyField{key: "kind", value: appendAnyString(nil, "mention")},
				)),
				itemFormat(rootParent("doc"), "bold", appendAnyBool(nil, true)),
				itemAny(rootParent("doc"), anyFirst, anySecond),
			},
		},
	)
	ids := NewContentIDs()
	_ = ids.Inserts.Add(40, 0, 6)

	got, err := IntersectUpdateWithContentIDsV1(update, ids)
	if err != nil {
		t.Fatalf("IntersectUpdateWithContentIDsV1() unexpected error: %v", err)
	}

	decoded, err := DecodeV1(got)
	if err != nil {
		t.Fatalf("DecodeV1(intersection) unexpected error: %v", err)
	}

	if len(decoded.Structs) != 6 {
		t.Fatalf("len(Structs) = %d, want 6", len(decoded.Structs))
	}

	for i, expectedClock := range []uint32{0, 1, 2, 3, 4, 5} {
		item, ok := decoded.Structs[i].(*ytypes.Item)
		if !ok {
			t.Fatalf("Structs[%d] type = %T, want *ytypes.Item", i, decoded.Structs[i])
		}
		if item.ID().Clock != expectedClock {
			t.Fatalf("Structs[%d].ID.Clock = %d, want %d", i, item.ID().Clock, expectedClock)
		}
	}

	expectedRefs := []byte{
		itemContentType,
		itemContentDoc,
		itemContentBinary,
		itemContentEmbed,
		itemContentFormat,
		itemContentAny,
	}
	for i, expectedRef := range expectedRefs {
		item := decoded.Structs[i].(*ytypes.Item)
		content := item.Content.(ParsedContent)
		if content.ContentRef() != expectedRef {
			t.Fatalf("Structs[%d].ContentRef = %d, want %d", i, content.ContentRef(), expectedRef)
		}
	}

	anyContent := decoded.Structs[5].(*ytypes.Item).Content.(ParsedContent)
	if len(anyContent.Any) != 1 || !bytes.Equal(anyContent.Any[0], anyFirst) {
		t.Fatalf("anyContent = %#v, want [%#v]", anyContent.Any, anyFirst)
	}
}

func TestIntersectUpdateWithContentIDsV1HandlesAscendingMultiClientSlices(t *testing.T) {
	t.Parallel()

	update := buildUpdate(
		clientBlock{
			client: 4,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "abcd"),
			},
		},
		clientBlock{
			client: 12,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "wxyz"),
			},
		},
	)

	ids := NewContentIDs()
	_ = ids.Inserts.Add(4, 1, 2)
	_ = ids.Inserts.Add(12, 1, 2)

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
	firstContent := first.Content.(ParsedContent)
	if first.ID() != (ytypes.ID{Client: 4, Clock: 1}) || firstContent.Text != "bc" {
		t.Fatalf("Structs[0] = id=%+v content=%#v, want client=4 clock=1 text=bc", first.ID(), firstContent)
	}

	second, ok := decoded.Structs[1].(*ytypes.Item)
	if !ok {
		t.Fatalf("Structs[1] type = %T, want *ytypes.Item", decoded.Structs[1])
	}
	secondContent := second.Content.(ParsedContent)
	if second.ID() != (ytypes.ID{Client: 12, Clock: 1}) || secondContent.Text != "xy" {
		t.Fatalf("Structs[1] = id=%+v content=%#v, want client=12 clock=1 text=xy", second.ID(), secondContent)
	}
}

func TestIntersectUpdateWithContentIDsV1CombinesMultiClientGapsAndDeletes(t *testing.T) {
	t.Parallel()

	anyFirst := appendAnyString(nil, "u")
	anySecond := appendAnyString(nil, "v")
	anyThird := appendAnyString(nil, "w")

	update := buildUpdate(
		clientBlock{
			client: 101,
			clock:  0,
			structs: []structEncoding{
				itemAny(rootParent("doc"), anyFirst, anySecond, anyThird, appendAnyString(nil, "x")),
				itemString(rootParent("doc"), "xy"),
			},
		},
		clientBlock{
			client: 33,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "wxyz"),
			},
		},
		deleteRange{client: 55, clock: 5, length: 2},
		deleteRange{client: 101, clock: 4, length: 3},
	)

	ids := NewContentIDs()
	_ = ids.Inserts.Add(101, 1, 2)
	_ = ids.Inserts.Add(101, 4, 1)
	_ = ids.Inserts.Add(33, 2, 1)
	_ = ids.Deletes.Add(55, 6, 1)
	_ = ids.Deletes.Add(101, 5, 1)

	got, err := IntersectUpdateWithContentIDsV1(update, ids)
	if err != nil {
		t.Fatalf("IntersectUpdateWithContentIDsV1() unexpected error: %v", err)
	}

	decoded, err := DecodeV1(got)
	if err != nil {
		t.Fatalf("DecodeV1(intersection) unexpected error: %v", err)
	}

	if len(decoded.Structs) != 4 {
		t.Fatalf("len(Structs) = %d, want 4", len(decoded.Structs))
	}

	anyResult, ok := decoded.Structs[0].(*ytypes.Item)
	if !ok {
		t.Fatalf("Structs[0] type = %T, want *ytypes.Item", decoded.Structs[0])
	}
	if anyResult.ID() != (ytypes.ID{Client: 101, Clock: 1}) {
		t.Fatalf("Structs[0].ID = %+v, want client=101 clock=1", anyResult.ID())
	}
	anyContent := anyResult.Content.(ParsedContent)
	if anyContent.ContentRef() != itemContentAny {
		t.Fatalf("Structs[0] contentref = %d, want %d", anyContent.ContentRef(), itemContentAny)
	}
	if len(anyContent.Any) != 2 || !bytes.Equal(anyContent.Any[0], anySecond) || !bytes.Equal(anyContent.Any[1], anyThird) {
		t.Fatalf("anyContent = %#v, want [%#v %#v]", anyContent.Any, anySecond, anyThird)
	}

	skip, ok := decoded.Structs[1].(ytypes.Skip)
	if !ok {
		t.Fatalf("Structs[1] type = %T, want ytypes.Skip", decoded.Structs[1])
	}
	if skip.ID().Clock != 3 || skip.Length() != 1 {
		t.Fatalf("Structs[1] = %+v, want synthetic skip at clock 3 len 1", skip)
	}

	stringResult, ok := decoded.Structs[2].(*ytypes.Item)
	if !ok {
		t.Fatalf("Structs[2] type = %T, want *ytypes.Item", decoded.Structs[2])
	}
	stringContent := stringResult.Content.(ParsedContent)
	if stringResult.ID() != (ytypes.ID{Client: 101, Clock: 4}) || stringContent.Text != "x" {
		t.Fatalf("Structs[2] = id=%+v content=%#v, want client=101 clock=4 text=x", stringResult.ID(), stringContent)
	}

	client33Result, ok := decoded.Structs[3].(*ytypes.Item)
	if !ok {
		t.Fatalf("Structs[3] type = %T, want *ytypes.Item", decoded.Structs[3])
	}
	client33Content := client33Result.Content.(ParsedContent)
	if client33Result.ID() != (ytypes.ID{Client: 33, Clock: 2}) || client33Content.Text != "y" {
		t.Fatalf("Structs[3] = id=%+v content=%#v, want client=33 clock=2 text=y", client33Result.ID(), client33Content)
	}

	if !decoded.DeleteSet.Has(ytypes.ID{Client: 55, Clock: 6}) {
		t.Fatalf("DeleteSet = %#v, want clock 6 for client 55", decoded.DeleteSet)
	}
	if decoded.DeleteSet.Has(ytypes.ID{Client: 55, Clock: 5}) {
		t.Fatalf("DeleteSet = %#v, want to exclude clock 5 for client 55", decoded.DeleteSet)
	}
	if !decoded.DeleteSet.Has(ytypes.ID{Client: 101, Clock: 5}) {
		t.Fatalf("DeleteSet = %#v, want clock 5 for client 101", decoded.DeleteSet)
	}
	if decoded.DeleteSet.Has(ytypes.ID{Client: 101, Clock: 4}) {
		t.Fatalf("DeleteSet = %#v, want to exclude clock 4 for client 101", decoded.DeleteSet)
	}
}
