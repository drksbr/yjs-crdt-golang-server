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

func TestMergeUpdatesV1OverlappingItemAfterDeletedGapUsesJsOffset(t *testing.T) {
	t.Parallel()

	left := buildUpdate(
		clientBlock{
			client: 15,
			clock:  0,
			structs: []structEncoding{
				itemDeleted(rootParent("doc"), 6),
			},
		},
	)
	right := buildUpdate(
		clientBlock{
			client: 15,
			clock:  5,
			structs: []structEncoding{
				itemString(rootParent("doc"), "abcd"),
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

	deleted, ok := decoded.Structs[0].(*ytypes.Item)
	if !ok {
		t.Fatalf("Structs[0] type = %T, want *ytypes.Item", decoded.Structs[0])
	}
	deletedContent := deleted.Content.(ParsedContent)
	if deletedContent.ContentRef() != itemContentDeleted || deleted.ID().Clock != 0 || deleted.Length() != 6 {
		t.Fatalf("Structs[0] = %#v, want deleted item at clock 0 len 6", decoded.Structs[0])
	}

	item, ok := decoded.Structs[1].(*ytypes.Item)
	if !ok {
		t.Fatalf("Structs[1] type = %T, want *ytypes.Item", decoded.Structs[1])
	}
	content := item.Content.(ParsedContent)
	if item.ID().Clock != 6 || content.Text != "bcd" {
		t.Fatalf("Structs[1] = id=%+v content=%#v, want item at clock 6 text \"bcd\"", item.ID(), content)
	}
}

func TestMergeUpdatesV1OverlappingMeta9StructsDeduplicatesByClock(t *testing.T) {
	t.Parallel()

	updateA := buildUpdate(
		clientBlock{
			client: 13,
			clock:  0,
			structs: []structEncoding{
				itemType(rootParent("doc"), typeRefYXmlElement, "p"),
				itemDoc(rootParent("doc"), "guid-meta9", appendAnyString(nil, "left")),
				itemBinary(rootParent("doc"), []byte{0xde, 0xad}),
				itemEmbed(rootParent("doc"), appendAnyObjectFields(nil,
					anyField{key: "kind", value: appendAnyString(nil, "mention")},
				)),
				itemFormat(rootParent("doc"), "bold", appendAnyBool(nil, true)),
				itemAny(rootParent("doc"), appendAnyString(nil, "a"), appendAnyBool(nil, true)),
			},
		},
	)
	updateB := buildUpdate(
		clientBlock{
			client: 13,
			clock:  0,
			structs: []structEncoding{
				itemType(rootParent("doc"), typeRefYXmlElement, "p"),
				itemDoc(rootParent("doc"), "guid-meta9", appendAnyString(nil, "left")),
				itemBinary(rootParent("doc"), []byte{0xde, 0xad}),
				itemEmbed(rootParent("doc"), appendAnyObjectFields(nil,
					anyField{key: "kind", value: appendAnyString(nil, "mention")},
				)),
				itemFormat(rootParent("doc"), "bold", appendAnyBool(nil, true)),
				itemAny(rootParent("doc"), appendAnyString(nil, "a"), appendAnyBool(nil, true)),
			},
		},
	)

	merged, err := MergeUpdatesV1(updateA, updateB)
	if err != nil {
		t.Fatalf("MergeUpdatesV1() unexpected error: %v", err)
	}

	decoded, err := DecodeV1(merged)
	if err != nil {
		t.Fatalf("DecodeV1(merged) unexpected error: %v", err)
	}

	if len(decoded.Structs) != 6 {
		t.Fatalf("len(Structs) = %d, want 6", len(decoded.Structs))
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
		item, ok := decoded.Structs[i].(*ytypes.Item)
		if !ok {
			t.Fatalf("Structs[%d] type = %T, want *ytypes.Item", i, decoded.Structs[i])
		}
		content := item.Content.(ParsedContent)
		if content.ContentRef() != expectedRef {
			t.Fatalf("Structs[%d].ContentRef = %d, want %d", i, content.ContentRef(), expectedRef)
		}
		if expectedRef == itemContentType && content.TypeName != "p" {
			t.Fatalf("Structs[%d].TypeName = %q, want \"p\"", i, content.TypeName)
		}
		if expectedRef == itemContentFormat && (content.TypeName != "bold" || content.IsCountable()) {
			t.Fatalf("Structs[%d].format = %#v, want non-countable format \"bold\"", i, content)
		}
		if expectedRef == itemContentDoc && content.TypeName != "guid-meta9" {
			t.Fatalf("Structs[%d].TypeName = %q, want \"guid-meta9\"", i, content.TypeName)
		}
		if expectedRef == itemContentAny && len(content.Any) != 2 {
			t.Fatalf("Structs[%d].Any len = %d, want 2", i, len(content.Any))
		}
	}
}

func TestMergeUpdatesV1AnyOverlapSlicesTailFromSecondUpdate(t *testing.T) {
	t.Parallel()

	leftFirst := appendAnyString(nil, "left-1")
	leftSecond := appendAnyBool(nil, true)
	rightSecond := appendAnyString(nil, "right-2")
	rightThird := appendAnyBool(nil, false)

	left := buildUpdate(
		clientBlock{
			client: 14,
			clock:  0,
			structs: []structEncoding{
				itemAny(rootParent("doc"), leftFirst, leftSecond),
			},
		},
	)
	right := buildUpdate(
		clientBlock{
			client: 14,
			clock:  1,
			structs: []structEncoding{
				itemAny(rootParent("doc"), rightSecond, rightThird),
				itemFormat(rootParent("doc"), "bold", appendAnyBool(nil, true)),
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
		t.Fatalf("len(Structs) = %d, want 3", len(decoded.Structs))
	}

	first, ok := decoded.Structs[0].(*ytypes.Item)
	if !ok {
		t.Fatalf("Structs[0] type = %T, want *ytypes.Item", decoded.Structs[0])
	}
	content := first.Content.(ParsedContent)
	if content.ContentRef() != itemContentAny || len(content.Any) != 2 {
		t.Fatalf("first = %#v, want any length 2", content)
	}
	if !bytes.Equal(content.Any[0], leftFirst) || !bytes.Equal(content.Any[1], leftSecond) {
		t.Fatalf("first.Any = %#v, want left-first/second payloads", content.Any)
	}

	second, ok := decoded.Structs[1].(*ytypes.Item)
	if !ok {
		t.Fatalf("Structs[1] type = %T, want *ytypes.Item", decoded.Structs[1])
	}
	content = second.Content.(ParsedContent)
	if content.ContentRef() != itemContentAny || second.ID().Clock != 2 || len(content.Any) != 1 {
		t.Fatalf("second = id=%+v content=%#v, want clock 2 and any length 1", second.ID(), content)
	}
	if !bytes.Equal(content.Any[0], rightThird) {
		t.Fatalf("second.Any = %#v, want %#v", content.Any, rightThird)
	}

	third, ok := decoded.Structs[2].(*ytypes.Item)
	if !ok {
		t.Fatalf("Structs[2] type = %T, want *ytypes.Item", decoded.Structs[2])
	}
	content = third.Content.(ParsedContent)
	if content.ContentRef() != itemContentFormat || third.ID().Clock != 3 || content.TypeName != "bold" {
		t.Fatalf("third = id=%+v content=%#v, want format struct at clock 3", third.ID(), content)
	}
}

func TestMergeUpdatesV1FillsExplicitSkipFromInput(t *testing.T) {
	t.Parallel()

	left := buildUpdate(
		clientBlock{
			client: 90,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "ab"),
				skip(3),
				itemBinary(rootParent("doc"), []byte{0x7f}),
			},
		},
	)
	fill := buildUpdate(
		clientBlock{
			client: 90,
			clock:  3,
			structs: []structEncoding{
				itemString(rootParent("doc"), "xy"),
			},
		},
	)

	want := buildUpdate(
		clientBlock{
			client: 90,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "ab"),
				skip(1),
				itemString(rootParent("doc"), "xy"),
				itemBinary(rootParent("doc"), []byte{0x7f}),
			},
		},
	)

	for _, updates := range permutations([][]byte{left, fill}) {
		merged, err := MergeUpdatesV1(updates...)
		if err != nil {
			t.Fatalf("MergeUpdatesV1() unexpected error: %v", err)
		}
		if !bytes.Equal(merged, want) {
			t.Fatalf("MergeUpdatesV1() = %v, want %v", merged, want)
		}
	}
}
