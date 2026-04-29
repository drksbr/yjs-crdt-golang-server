package yupdate

import (
	"bytes"
	"slices"
	"testing"

	"github.com/drksbr/yjs-crdt-golang-server/internal/ytypes"
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

func TestEncodeV1RoundTripsLessExercisedRefs(t *testing.T) {
	t.Parallel()

	update := buildUpdate(
		clientBlock{
			client: 13,
			clock:  0,
			structs: []structEncoding{
				itemType(rootParent("doc"), typeRefYXmlElement, "p"),
				itemDoc(rootParent("doc"), "guid-meta9", appendAnyObject(nil, map[string][]byte{
					"name": appendAnyString(nil, "subdoc"),
				})),
				itemBinary(rootParent("doc"), []byte{0xde, 0xad}),
				itemEmbed(rootParent("doc"), appendAnyObjectFields(nil,
					anyField{key: "kind", value: appendAnyString(nil, "mention")},
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

	typeContent := decoded.Structs[0].(*ytypes.Item).Content.(ParsedContent)
	if typeContent.ContentRef() != itemContentType || !typeContent.IsCountable() || typeContent.EmbeddedType() != typeRefYXmlElement || typeContent.TypeName != "p" {
		t.Fatalf("type content = %#v, want countable xml element type p", typeContent)
	}

	doc := decoded.Structs[1].(*ytypes.Item).Content.(ParsedContent)
	if doc.ContentRef() != itemContentDoc || doc.TypeName != "guid-meta9" {
		t.Fatalf("doc content = %#v, want guid guid-meta9", doc)
	}

	binary := decoded.Structs[2].(*ytypes.Item).Content.(ParsedContent)
	if binary.ContentRef() != itemContentBinary || !binary.IsCountable() || binary.Length() != 1 {
		t.Fatalf("binary content = %#v, want countable binary at length 1", binary)
	}

	embed := decoded.Structs[3].(*ytypes.Item).Content.(ParsedContent)
	if embed.ContentRef() != itemContentEmbed || !embed.IsCountable() {
		t.Fatalf("embed content = %#v, want countable embed", embed)
	}

	format := decoded.Structs[4].(*ytypes.Item).Content.(ParsedContent)
	if format.ContentRef() != itemContentFormat || format.IsCountable() || format.TypeName != "bold" {
		t.Fatalf("format content = %#v, want non-countable format key bold", format)
	}

	anyContent := decoded.Structs[5].(*ytypes.Item).Content.(ParsedContent)
	if anyContent.ContentRef() != itemContentAny || len(anyContent.Any) != 2 {
		t.Fatalf("any content = %#v, want two raw values", anyContent)
	}
}

func TestMeta9MergeDiffIntersectWorkflowAcrossThreeOutOfOrderUpdates(t *testing.T) {
	t.Parallel()

	u1 := buildUpdate(
		clientBlock{
			client: 1,
			clock:  4,
			structs: []structEncoding{
				itemString(rootParent("doc"), "ef"),
			},
		},
		deleteRange{client: 1, clock: 100, length: 2},
	)
	u2 := buildUpdate(
		clientBlock{
			client: 1,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "ab"),
			},
		},
		deleteRange{client: 1, clock: 104, length: 1},
	)
	u3 := buildUpdate(
		clientBlock{
			client: 1,
			clock:  2,
			structs: []structEncoding{
				itemString(rootParent("doc"), "CD"),
			},
		},
		deleteRange{client: 1, clock: 200, length: 1},
	)

	merged, err := MergeUpdatesV1(u1, u2, u3)
	if err != nil {
		t.Fatalf("MergeUpdatesV1() unexpected error: %v", err)
	}

	decodedMerged, err := DecodeV1(merged)
	if err != nil {
		t.Fatalf("DecodeV1(merged) unexpected error: %v", err)
	}
	if len(decodedMerged.Structs) != 3 {
		t.Fatalf("len(decodedMerged.Structs) = %d, want 3", len(decodedMerged.Structs))
	}
	assertStringStruct(t, decodedMerged.Structs[0], 1, 0, "ab")
	assertStringStruct(t, decodedMerged.Structs[1], 1, 2, "CD")
	assertStringStruct(t, decodedMerged.Structs[2], 1, 4, "ef")

	if len(decodedMerged.DeleteSet.Clients()) != 1 || len(decodedMerged.DeleteSet.Ranges(1)) != 3 {
		t.Fatalf("DeleteSet = %#v, want one client with three merged ranges", decodedMerged.DeleteSet)
	}
	for _, id := range []ytypes.ID{
		{Client: 1, Clock: 100},
		{Client: 1, Clock: 101},
		{Client: 1, Clock: 104},
		{Client: 1, Clock: 200},
	} {
		if !decodedMerged.DeleteSet.Has(id) {
			t.Fatalf("DeleteSet missing %+v in %#v", id, decodedMerged.DeleteSet)
		}
	}

	diff, err := DiffUpdateV1(merged, encodeStateVectorEntry(1, 3))
	if err != nil {
		t.Fatalf("DiffUpdateV1() unexpected error: %v", err)
	}
	decodedDiff, err := DecodeV1(diff)
	if err != nil {
		t.Fatalf("DecodeV1(diff) unexpected error: %v", err)
	}
	if len(decodedDiff.Structs) != 2 {
		t.Fatalf("len(decodedDiff.Structs) = %d, want 2", len(decodedDiff.Structs))
	}
	assertStringStruct(t, decodedDiff.Structs[0], 1, 3, "D")
	assertStringStruct(t, decodedDiff.Structs[1], 1, 4, "ef")
	for _, id := range []ytypes.ID{
		{Client: 1, Clock: 100},
		{Client: 1, Clock: 101},
		{Client: 1, Clock: 104},
		{Client: 1, Clock: 200},
	} {
		if !decodedDiff.DeleteSet.Has(id) {
			t.Fatalf("diff delete set missing %+v in %#v", id, decodedDiff.DeleteSet)
		}
	}

	ids := NewContentIDs()
	_ = ids.Inserts.Add(1, 1, 4)
	_ = ids.Deletes.Add(1, 104, 1)

	intersection, err := IntersectUpdateWithContentIDsV1(merged, ids)
	if err != nil {
		t.Fatalf("IntersectUpdateWithContentIDsV1() unexpected error: %v", err)
	}
	decodedIntersection, err := DecodeV1(intersection)
	if err != nil {
		t.Fatalf("DecodeV1(intersection) unexpected error: %v", err)
	}
	if len(decodedIntersection.Structs) != 3 {
		t.Fatalf("len(decodedIntersection.Structs) = %d, want 3", len(decodedIntersection.Structs))
	}
	assertStringStruct(t, decodedIntersection.Structs[0], 1, 1, "b")
	assertStringStruct(t, decodedIntersection.Structs[1], 1, 2, "CD")
	assertStringStruct(t, decodedIntersection.Structs[2], 1, 4, "e")

	if len(decodedIntersection.DeleteSet.Clients()) != 1 || len(decodedIntersection.DeleteSet.Ranges(1)) != 1 {
		t.Fatalf("intersection DeleteSet = %#v, want one client with one selected range", decodedIntersection.DeleteSet)
	}
	if !decodedIntersection.DeleteSet.Has(ytypes.ID{Client: 1, Clock: 104}) {
		t.Fatalf("intersection delete set = %#v, want clock 104 only", decodedIntersection.DeleteSet)
	}
	if decodedIntersection.DeleteSet.Has(ytypes.ID{Client: 1, Clock: 100}) || decodedIntersection.DeleteSet.Has(ytypes.ID{Client: 1, Clock: 200}) {
		t.Fatalf("intersection delete set = %#v, want to exclude non-selected deletes", decodedIntersection.DeleteSet)
	}
}

func assertStringStruct(t *testing.T, current ytypes.Struct, client, clock uint32, text string) {
	t.Helper()

	item, ok := current.(*ytypes.Item)
	if !ok {
		t.Fatalf("struct type = %T, want *ytypes.Item", current)
	}
	content, ok := item.Content.(ParsedContent)
	if !ok {
		t.Fatalf("item content type = %T, want ParsedContent", item.Content)
	}
	if item.ID().Client != client || item.ID().Clock != clock || content.ContentRef() != itemContentString || content.Text != text {
		t.Fatalf("item = id=%+v content=%#v, want client=%d clock=%d text=%q", item.ID(), content, client, clock, text)
	}
}
