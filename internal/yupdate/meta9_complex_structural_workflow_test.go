package yupdate

import (
	"bytes"
	"testing"

	"github.com/drksbr/yjs-crdt-golang-server/internal/ytypes"
)

func TestMeta9ComplexStructuralMergeDiffIntersectWorkflow(t *testing.T) {
	t.Parallel()

	anyA := appendAnyString(nil, "a")
	anyB := appendAnyString(nil, "b")
	anyC := appendAnyString(nil, "c")
	anyQ := appendAnyString(nil, "q")
	anyR := appendAnyString(nil, "r")

	base := buildUpdate(
		clientBlock{
			client: 70,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "ab"),
				itemJSON(rootParent("doc"), `"j0"`, `"j1"`, `"j2"`),
				itemAny(rootParent("doc"), anyA, anyB, anyC),
				itemDeleted(rootParent("doc"), 2),
			},
		},
		clientBlock{
			client: 12,
			clock:  0,
			structs: []structEncoding{
				itemBinary(rootParent("doc"), []byte{0xca}),
				itemString(rootParent("doc"), "wxyz"),
			},
		},
		deleteRange{client: 70, clock: 30, length: 2},
	)
	tail := buildUpdate(
		clientBlock{
			client: 70,
			clock:  10,
			structs: []structEncoding{
				itemStringWithOptions(itemWireOptions{origin: idPtr(70, 9)}, "tail"),
			},
		},
		clientBlock{
			client: 12,
			clock:  5,
			structs: []structEncoding{
				itemAny(rootParent("doc"), anyQ, anyR),
			},
		},
		deleteRange{client: 70, clock: 31, length: 3},
		deleteRange{client: 12, clock: 9, length: 1},
	)
	meta := buildUpdate(
		clientBlock{
			client: 90,
			clock:  0,
			structs: []structEncoding{
				itemFormat(rootParent("doc"), "lang", appendAnyString(nil, "go")),
				itemDoc(rootParent("doc"), "guid-complex", appendAnyString(nil, "subdoc")),
			},
		},
	)

	merged, err := MergeUpdatesV1(tail, base, meta)
	if err != nil {
		t.Fatalf("MergeUpdatesV1() unexpected error: %v", err)
	}
	decodedMerged, err := DecodeV1(merged)
	if err != nil {
		t.Fatalf("DecodeV1(merged) unexpected error: %v", err)
	}
	assertStruct(t, decodedMerged.Structs, 70, 0, itemContentString)
	assertStruct(t, decodedMerged.Structs, 70, 2, itemContentJSON)
	assertStruct(t, decodedMerged.Structs, 70, 5, itemContentAny)
	assertStruct(t, decodedMerged.Structs, 70, 10, itemContentString)
	assertStruct(t, decodedMerged.Structs, 12, 5, itemContentAny)
	assertStruct(t, decodedMerged.Structs, 90, 0, itemContentFormat)
	assertStruct(t, decodedMerged.Structs, 90, 1, itemContentDoc)
	assertDeleteSetRanges(t, decodedMerged.DeleteSet, map[uint32][]ytypes.DeleteRange{
		70: {{Clock: 30, Length: 4}},
		12: {{Clock: 9, Length: 1}},
	})

	diff, err := DiffUpdateV1(merged, encodeStateVectorEntry(70, 3, 12, 1))
	if err != nil {
		t.Fatalf("DiffUpdateV1() unexpected error: %v", err)
	}
	decodedDiff, err := DecodeV1(diff)
	if err != nil {
		t.Fatalf("DecodeV1(diff) unexpected error: %v", err)
	}
	jsonTail := findParsedItem(t, decodedDiff.Structs, 70, 3, itemContentJSON)
	if len(jsonTail.JSON) != 2 || jsonTail.JSON[0] != `"j1"` || jsonTail.JSON[1] != `"j2"` {
		t.Fatalf("diff JSON tail = %#v, want [\"j1\" \"j2\"]", jsonTail.JSON)
	}
	anyTail := findParsedItem(t, decodedDiff.Structs, 70, 5, itemContentAny)
	if len(anyTail.Any) != 3 || !bytes.Equal(anyTail.Any[0], anyA) || !bytes.Equal(anyTail.Any[2], anyC) {
		t.Fatalf("diff Any tail = %#v, want [a b c]", anyTail.Any)
	}
	stringTail := findParsedItem(t, decodedDiff.Structs, 70, 10, itemContentString)
	if stringTail.Text != "tail" {
		t.Fatalf("diff tail string = %q, want tail", stringTail.Text)
	}
	client12Text := findParsedItem(t, decodedDiff.Structs, 12, 1, itemContentString)
	if client12Text.Text != "wxyz" {
		t.Fatalf("diff client 12 text = %q, want wxyz", client12Text.Text)
	}

	ids := NewContentIDs()
	_ = ids.Inserts.Add(70, 4, 3)
	_ = ids.Inserts.Add(70, 10, 2)
	_ = ids.Inserts.Add(12, 2, 2)
	_ = ids.Inserts.Add(90, 0, 2)
	_ = ids.Deletes.Add(70, 32, 1)
	_ = ids.Deletes.Add(12, 9, 1)

	intersection, err := IntersectUpdateWithContentIDsV1(diff, ids)
	if err != nil {
		t.Fatalf("IntersectUpdateWithContentIDsV1() unexpected error: %v", err)
	}
	decodedIntersection, err := DecodeV1(intersection)
	if err != nil {
		t.Fatalf("DecodeV1(intersection) unexpected error: %v", err)
	}

	selectedJSON := findParsedItem(t, decodedIntersection.Structs, 70, 4, itemContentJSON)
	if len(selectedJSON.JSON) != 1 || selectedJSON.JSON[0] != `"j2"` {
		t.Fatalf("intersection JSON = %#v, want [\"j2\"]", selectedJSON.JSON)
	}
	selectedAny := findParsedItem(t, decodedIntersection.Structs, 70, 5, itemContentAny)
	if len(selectedAny.Any) != 2 || !bytes.Equal(selectedAny.Any[0], anyA) || !bytes.Equal(selectedAny.Any[1], anyB) {
		t.Fatalf("intersection Any = %#v, want [a b]", selectedAny.Any)
	}
	assertSkipStruct(t, decodedIntersection.Structs, 70, 7, 3)
	selectedTail := findParsedItem(t, decodedIntersection.Structs, 70, 10, itemContentString)
	if selectedTail.Text != "ta" {
		t.Fatalf("intersection tail string = %q, want ta", selectedTail.Text)
	}
	selectedClient12 := findParsedItem(t, decodedIntersection.Structs, 12, 2, itemContentString)
	if selectedClient12.Text != "xy" {
		t.Fatalf("intersection client 12 string = %q, want xy", selectedClient12.Text)
	}
	assertStruct(t, decodedIntersection.Structs, 90, 0, itemContentFormat)
	assertStruct(t, decodedIntersection.Structs, 90, 1, itemContentDoc)
	assertDeleteSetRanges(t, decodedIntersection.DeleteSet, map[uint32][]ytypes.DeleteRange{
		70: {{Clock: 32, Length: 1}},
		12: {{Clock: 9, Length: 1}},
	})
}

func findParsedItem(t *testing.T, structs []ytypes.Struct, client, clock uint32, contentRef byte) ParsedContent {
	t.Helper()

	for _, current := range structs {
		item, ok := current.(*ytypes.Item)
		if !ok {
			continue
		}
		content, ok := item.Content.(ParsedContent)
		if !ok {
			continue
		}
		if item.ID().Client == client && item.ID().Clock == clock && content.ContentRef() == contentRef {
			return content
		}
	}
	t.Fatalf("Structs=%s\nmissing parsed item client=%d clock=%d contentRef=%d", describeStructs(structs), client, clock, contentRef)
	return ParsedContent{}
}

func assertSkipStruct(t *testing.T, structs []ytypes.Struct, client, clock, length uint32) {
	t.Helper()

	for _, current := range structs {
		if current.Kind() != ytypes.KindSkip {
			continue
		}
		if current.ID().Client == client && current.ID().Clock == clock && current.Length() == length {
			return
		}
	}
	t.Fatalf("Structs=%s\nmissing skip client=%d clock=%d length=%d", describeStructs(structs), client, clock, length)
}
