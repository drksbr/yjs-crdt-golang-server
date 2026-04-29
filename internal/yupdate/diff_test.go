package yupdate

import (
	"bytes"
	"testing"

	"github.com/drksbr/yjs-crdt-golang-server/internal/varint"
	"github.com/drksbr/yjs-crdt-golang-server/internal/ytypes"
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

func TestDiffUpdateV1FiltersMeta9StructsAndTailAnyContent(t *testing.T) {
	t.Parallel()

	update := buildUpdate(
		clientBlock{
			client: 18,
			clock:  0,
			structs: []structEncoding{
				itemType(rootParent("doc"), typeRefYXmlElement, "p"),
				itemDoc(rootParent("doc"), "guid-meta9", appendAnyString(nil, "subdoc")),
				itemBinary(rootParent("doc"), []byte{0xde, 0xad}),
				itemEmbed(rootParent("doc"), appendAnyObjectFields(nil,
					anyField{key: "kind", value: appendAnyString(nil, "mention")},
				)),
				itemFormat(rootParent("doc"), "bold", appendAnyBool(nil, true)),
				itemAny(rootParent("doc"), appendAnyString(nil, "x"), appendAnyString(nil, "y"), appendAnyString(nil, "z")),
			},
		},
	)

	got, err := DiffUpdateV1(update, encodeStateVectorEntry(18, 2))
	if err != nil {
		t.Fatalf("DiffUpdateV1() unexpected error: %v", err)
	}

	decoded, err := DecodeV1(got)
	if err != nil {
		t.Fatalf("DecodeV1(diff) unexpected error: %v", err)
	}

	if len(decoded.Structs) != 4 {
		t.Fatalf("len(Structs) = %d, want 4", len(decoded.Structs))
	}

	binary, ok := decoded.Structs[0].(*ytypes.Item)
	if !ok {
		t.Fatalf("Structs[0] type = %T, want *ytypes.Item", decoded.Structs[0])
	}
	content := binary.Content.(ParsedContent)
	if binary.ID().Clock != 2 || content.ContentRef() != itemContentBinary || content.Length() != 1 {
		t.Fatalf("Struct[0] = id=%+v content=%#v, want binary clock 2", binary.ID(), content)
	}

	embed, ok := decoded.Structs[1].(*ytypes.Item)
	if !ok {
		t.Fatalf("Structs[1] type = %T, want *ytypes.Item", decoded.Structs[1])
	}
	content = embed.Content.(ParsedContent)
	if embed.ID().Clock != 3 || content.ContentRef() != itemContentEmbed {
		t.Fatalf("Struct[1] = id=%+v content=%#v, want embed clock 3", embed.ID(), content)
	}

	format, ok := decoded.Structs[2].(*ytypes.Item)
	if !ok {
		t.Fatalf("Structs[2] type = %T, want *ytypes.Item", decoded.Structs[2])
	}
	content = format.Content.(ParsedContent)
	if format.ID().Clock != 4 || content.ContentRef() != itemContentFormat || content.TypeName != "bold" {
		t.Fatalf("Struct[2] = id=%+v content=%#v, want format clock 4", format.ID(), content)
	}

	anyContent, ok := decoded.Structs[3].(*ytypes.Item)
	if !ok {
		t.Fatalf("Structs[3] type = %T, want *ytypes.Item", decoded.Structs[3])
	}
	any := anyContent.Content.(ParsedContent)
	if anyContent.ID().Clock != 5 || any.ContentRef() != itemContentAny || len(any.Any) != 3 {
		t.Fatalf("Struct[3] = id=%+v content=%#v, want any clock 5 with 3 values", anyContent.ID(), any)
	}
}

func TestDiffUpdateV1SlicesAnyAfterNonStringPrefix(t *testing.T) {
	t.Parallel()

	first := appendAnyString(nil, "x")
	second := appendAnyString(nil, "y")
	third := appendAnyString(nil, "z")

	update := buildUpdate(
		clientBlock{
			client: 19,
			clock:  0,
			structs: []structEncoding{
				itemType(rootParent("doc"), typeRefYXmlText, "text"),
				itemBinary(rootParent("doc"), []byte{0x01}),
				itemFormat(rootParent("doc"), "italic", appendAnyBool(nil, true)),
				itemAny(rootParent("doc"), first, second, third),
			},
		},
	)

	got, err := DiffUpdateV1(update, encodeStateVectorEntry(19, 5))
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
	anyContent, ok := decoded.Structs[0].(*ytypes.Item)
	if !ok {
		t.Fatalf("Structs[0] type = %T, want *ytypes.Item", decoded.Structs[0])
	}
	any := anyContent.Content.(ParsedContent)
	if anyContent.ID().Clock != 5 || any.ContentRef() != itemContentAny || len(any.Any) != 1 || !bytes.Equal(any.Any[0], third) {
		t.Fatalf("any tail = id=%+v content=%#v, want clock 5 and one value z", anyContent.ID(), any)
	}
}

func TestDiffUpdateV1FiltersMultipleClientsWithMixedRefsAndPartialSlices(t *testing.T) {
	t.Parallel()

	update := buildUpdate(
		clientBlock{
			client: 11,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "abcdef"),
			},
		},
		clientBlock{
			client: 4,
			clock:  0,
			structs: []structEncoding{
				skip(2),
				itemDeleted(rootParent("doc"), 3),
				itemBinary(rootParent("doc"), []byte{0xde, 0xad}),
				itemAny(rootParent("doc"), appendAnyString(nil, "x"), appendAnyBool(nil, true)),
			},
		},
		deleteRange{client: 4, clock: 20, length: 2},
	)

	got, err := DiffUpdateV1(update, encodeStateVectorEntry(11, 4, 4, 3))
	if err != nil {
		t.Fatalf("DiffUpdateV1() unexpected error: %v", err)
	}

	decoded, err := DecodeV1(got)
	if err != nil {
		t.Fatalf("DecodeV1(diff) unexpected error: %v", err)
	}

	if len(decoded.Structs) != 4 {
		t.Fatalf("len(Structs) = %d, want 4", len(decoded.Structs))
	}

	first, ok := decoded.Structs[0].(*ytypes.Item)
	if !ok {
		t.Fatalf("Structs[0] type = %T, want *ytypes.Item", decoded.Structs[0])
	}
	firstContent := first.Content.(ParsedContent)
	if first.ID() != (ytypes.ID{Client: 11, Clock: 4}) || first.Length() != 2 || firstContent.ContentRef() != itemContentString || firstContent.Text != "ef" {
		t.Fatalf("Structs[0] = id=%+v len=%d content=%#v, want id=client 11 clock 4 len 2 text \"ef\"", first.ID(), first.Length(), firstContent)
	}

	second, ok := decoded.Structs[1].(*ytypes.Item)
	if !ok {
		t.Fatalf("Structs[1] type = %T, want *ytypes.Item", decoded.Structs[1])
	}
	secondContent := second.Content.(ParsedContent)
	if second.ID() != (ytypes.ID{Client: 4, Clock: 3}) || second.Length() != 2 || secondContent.ContentRef() != itemContentDeleted {
		t.Fatalf("Structs[1] = id=%+v len=%d content=%#v, want id=client 4 clock 3 len 2 deleted", second.ID(), second.Length(), secondContent)
	}

	third, ok := decoded.Structs[2].(*ytypes.Item)
	if !ok {
		t.Fatalf("Structs[2] type = %T, want *ytypes.Item", decoded.Structs[2])
	}
	thirdContent := third.Content.(ParsedContent)
	if third.ID() != (ytypes.ID{Client: 4, Clock: 5}) || third.Length() != 1 || thirdContent.ContentRef() != itemContentBinary {
		t.Fatalf("Structs[2] = id=%+v len=%d content=%#v, want id=client 4 clock 5 len 1 binary", third.ID(), third.Length(), thirdContent)
	}

	fourth, ok := decoded.Structs[3].(*ytypes.Item)
	if !ok {
		t.Fatalf("Structs[3] type = %T, want *ytypes.Item", decoded.Structs[3])
	}
	fourthContent := fourth.Content.(ParsedContent)
	if fourth.ID() != (ytypes.ID{Client: 4, Clock: 6}) || len(fourthContent.Any) != 2 || fourthContent.ContentRef() != itemContentAny {
		t.Fatalf("Structs[3] = id=%+v content=%#v, want id=client 4 clock 6 any len 2", fourth.ID(), fourthContent)
	}

	if !decoded.DeleteSet.Has(ytypes.ID{Client: 4, Clock: 20}) || !decoded.DeleteSet.Has(ytypes.ID{Client: 4, Clock: 21}) {
		t.Fatalf("DeleteSet = %#v, want client 4 clock 20 and 21", decoded.DeleteSet)
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
