package yupdate

import (
	"bytes"
	"slices"
	"testing"

	"yjs-go-bridge/internal/ytypes"
)

func TestEncodeV1RoundTripsExtendedContentRefs(t *testing.T) {
	t.Parallel()

	update := buildUpdate(
		clientBlock{
			client: 8,
			clock:  0,
			structs: []structEncoding{
				itemJSON(rootParent("doc"), `"a"`, `{"k":1}`),
				itemBinary(rootParent("doc"), []byte{0xde, 0xad, 0xbe, 0xef}),
				itemEmbed(rootParent("doc"), appendAnyObjectFields(nil,
					anyField{key: "kind", value: appendAnyString(nil, "mention")},
					anyField{key: "open", value: appendAnyBool(nil, true)},
				)),
				itemFormat(rootParent("doc"), "bold", appendAnyBool(nil, true)),
				itemAny(rootParent("doc"), appendAnyString(nil, "x"), appendAnyBool(nil, false)),
			},
		},
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

	if got := decoded.Structs[0].(*ytypes.Item).Content.(ParsedContent).JSON; !slices.Equal(got, []string{`"a"`, `{"k":1}`}) {
		t.Fatalf("JSON content = %#v, want two original JSON entries", got)
	}
	format := decoded.Structs[3].(*ytypes.Item).Content.(ParsedContent)
	if format.ContentRef() != itemContentFormat || format.IsCountable() || format.TypeName != "bold" {
		t.Fatalf("format content = %#v, want non-countable format key bold", format)
	}
	anyContent := decoded.Structs[4].(*ytypes.Item).Content.(ParsedContent)
	if anyContent.ContentRef() != itemContentAny || len(anyContent.Any) != 2 {
		t.Fatalf("any content = %#v, want two raw any values", anyContent)
	}
}

func TestEncodeV1RoundTripsItemWireMetadata(t *testing.T) {
	t.Parallel()

	update := buildUpdate(
		clientBlock{
			client: 5,
			clock:  0,
			structs: []structEncoding{
				itemStringWithOptions(itemWireOptions{
					origin: idPtr(1, 1),
				}, "ab"),
				itemStringWithOptions(itemWireOptions{
					rightOrigin: idPtr(2, 9),
				}, "z"),
				itemJSONWithOptions(itemWireOptions{
					parent:    idParent(4, 7),
					parentSub: "title",
				}, `"x"`, `"y"`),
			},
		},
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

	first := decoded.Structs[0].(*ytypes.Item)
	if first.Origin == nil || *first.Origin != (ytypes.ID{Client: 1, Clock: 1}) {
		t.Fatalf("first origin = %+v, want {Client:1 Clock:1}", first.Origin)
	}
	if first.Parent.Kind() != ytypes.ParentNone {
		t.Fatalf("first parent kind = %v, want ParentNone", first.Parent.Kind())
	}

	second := decoded.Structs[1].(*ytypes.Item)
	if second.RightOrigin == nil || *second.RightOrigin != (ytypes.ID{Client: 2, Clock: 9}) {
		t.Fatalf("second right origin = %+v, want {Client:2 Clock:9}", second.RightOrigin)
	}

	third := decoded.Structs[2].(*ytypes.Item)
	parentID, ok := third.Parent.ID()
	if !ok || parentID != (ytypes.ID{Client: 4, Clock: 7}) {
		t.Fatalf("third parent = %+v, want parent id {Client:4 Clock:7}", third.Parent)
	}
	if third.ParentSub != "title" {
		t.Fatalf("third parentSub = %q, want title", third.ParentSub)
	}
}

func TestMergeUpdatesV1SlicesOverlappingJSONStructs(t *testing.T) {
	t.Parallel()

	left := buildUpdate(
		clientBlock{
			client: 6,
			clock:  0,
			structs: []structEncoding{
				itemJSON(rootParent("doc"), `"a"`, `"b"`, `"c"`),
			},
		},
	)
	right := buildUpdate(
		clientBlock{
			client: 6,
			clock:  2,
			structs: []structEncoding{
				itemJSON(rootParent("doc"), `"c"`, `"d"`, `"e"`),
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

	item := decoded.Structs[1].(*ytypes.Item)
	content := item.Content.(ParsedContent)
	if item.ID().Clock != 3 || content.Length() != 2 || !slices.Equal(content.JSON, []string{`"d"`, `"e"`}) {
		t.Fatalf("merged JSON tail = id=%+v content=%#v, want clock 3 and JSON [\"d\",\"e\"]", item.ID(), content)
	}
}

func TestDiffUpdateV1SlicesAnyContentTail(t *testing.T) {
	t.Parallel()

	first := appendAnyString(nil, "a")
	second := appendAnyBool(nil, true)
	third := appendAnyString(nil, "b")
	fourth := appendAnyBool(nil, false)

	update := buildUpdate(
		clientBlock{
			client: 10,
			clock:  0,
			structs: []structEncoding{
				itemAny(rootParent("doc"), first, second, third, fourth),
			},
		},
	)

	got, err := DiffUpdateV1(update, encodeStateVectorEntry(10, 2))
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

	item := decoded.Structs[0].(*ytypes.Item)
	content := item.Content.(ParsedContent)
	if item.ID().Clock != 2 {
		t.Fatalf("item clock = %d, want 2", item.ID().Clock)
	}
	if item.Origin == nil || *item.Origin != (ytypes.ID{Client: 10, Clock: 1}) {
		t.Fatalf("item origin = %+v, want {Client:10 Clock:1}", item.Origin)
	}
	if content.Length() != 2 || len(content.Any) != 2 {
		t.Fatalf("content = %#v, want two raw any values", content)
	}
	if !bytes.Equal(content.Any[0], third) || !bytes.Equal(content.Any[1], fourth) {
		t.Fatalf("content.Any = %v, want tail values", content.Any)
	}
}

func TestIntersectUpdateWithContentIDsV1SelectsJSONWindow(t *testing.T) {
	t.Parallel()

	update := buildUpdate(
		clientBlock{
			client: 12,
			clock:  0,
			structs: []structEncoding{
				itemJSON(rootParent("doc"), `"a"`, `"b"`, `"c"`, `"d"`),
			},
		},
	)
	ids := NewContentIDs()
	_ = ids.Inserts.Add(12, 1, 2)

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

	item := decoded.Structs[0].(*ytypes.Item)
	content := item.Content.(ParsedContent)
	if item.ID().Clock != 1 {
		t.Fatalf("item clock = %d, want 1", item.ID().Clock)
	}
	if item.Origin == nil || *item.Origin != (ytypes.ID{Client: 12, Clock: 0}) {
		t.Fatalf("item origin = %+v, want {Client:12 Clock:0}", item.Origin)
	}
	if !slices.Equal(content.JSON, []string{`"b"`, `"c"`}) {
		t.Fatalf("content.JSON = %#v, want [\"b\",\"c\"]", content.JSON)
	}
}
