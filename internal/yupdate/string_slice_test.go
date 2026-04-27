package yupdate

import (
	"testing"

	"yjs-go-bridge/internal/ytypes"
)

func TestParsedContentSliceWindowReplacesBrokenSurrogateTail(t *testing.T) {
	t.Parallel()

	content := ParsedContent{
		Ref:       itemContentString,
		LengthVal: utf16Length("🙂a"),
		Countable: true,
		Raw:       appendVarStringV1(nil, "🙂a"),
		Text:      "🙂a",
	}

	got, err := content.SliceWindow(0, 2)
	if err != nil {
		t.Fatalf("SliceWindow() unexpected error: %v", err)
	}

	if got.Text != "�" || got.Length() != 1 {
		t.Fatalf("SliceWindow() = text=%q len=%d, want text=%q len=1", got.Text, got.Length(), "�")
	}
}

func TestDiffUpdateV1ReplacesBrokenSurrogateBoundary(t *testing.T) {
	t.Parallel()

	update := buildUpdate(
		clientBlock{
			client: 3,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "🙂a"),
			},
		},
	)

	got, err := DiffUpdateV1(update, encodeStateVectorEntry(3, 1))
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

	item, ok := decoded.Structs[0].(*ytypes.Item)
	if !ok {
		t.Fatalf("Structs[0] type = %T, want *ytypes.Item", decoded.Structs[0])
	}
	content := item.Content.(ParsedContent)
	if item.ID().Clock != 1 || content.Text != "�a" || content.Length() != 2 {
		t.Fatalf("diff item = id=%+v content=%#v, want clock=1 text=%q len=2", item.ID(), content, "�a")
	}
}

func TestMergeUpdatesV1ReplacesBrokenSurrogateBoundary(t *testing.T) {
	t.Parallel()

	left := buildUpdate(
		clientBlock{
			client: 4,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "🙂"),
			},
		},
	)
	right := buildUpdate(
		clientBlock{
			client: 4,
			clock:  1,
			structs: []structEncoding{
				itemString(rootParent("doc"), "🙂a"),
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
	if item.ID().Clock != 2 || content.Text != "\uFFFDa" || content.Length() != 2 {
		t.Fatalf("merged item = id=%+v content=%#v, want clock=2 text=%q len=2", item.ID(), content, "\uFFFDa")
	}
}

func TestIntersectUpdateWithContentIDsV1ReplacesBrokenSurrogateBoundary(t *testing.T) {
	t.Parallel()

	update := buildUpdate(
		clientBlock{
			client: 6,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "🙂a"),
			},
		},
	)
	ids := NewContentIDs()
	_ = ids.Inserts.Add(6, 1, 2)

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
	if item.ID().Clock != 1 || content.Text != "�a" || content.Length() != 2 {
		t.Fatalf("intersect item = id=%+v content=%#v, want clock=1 text=%q len=2", item.ID(), content, "�a")
	}
}
