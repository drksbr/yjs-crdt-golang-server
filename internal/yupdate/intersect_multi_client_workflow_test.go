package yupdate

import (
	"testing"

	"github.com/drksbr/yjs-crdt-golang-server/internal/ytypes"
)

func TestMultiClientIntersectWorkflowWithMixedRefsAndDeleteSetFilter(t *testing.T) {
	t.Parallel()

	u1 := buildUpdate(
		clientBlock{
			client: 1,
			clock:  0,
			structs: []structEncoding{
				itemType(rootParent("doc"), typeRefYXmlElement, "p"),
				itemDoc(rootParent("doc"), "guid-doc", appendAnyString(nil, "subdoc")),
			},
		},
		deleteRange{client: 1, clock: 11, length: 2},
		clientBlock{
			client: 2,
			clock:  0,
			structs: []structEncoding{
				itemBinary(rootParent("doc"), []byte{0xde, 0xad}),
			},
		},
		deleteRange{client: 2, clock: 20, length: 1},
	)
	u2 := buildUpdate(
		clientBlock{
			client: 3,
			clock:  0,
			structs: []structEncoding{
				itemFormat(rootParent("doc"), "bold", appendAnyBool(nil, true)),
			},
		},
		clientBlock{
			client: 4,
			clock:  0,
			structs: []structEncoding{
				itemEmbed(rootParent("doc"), appendAnyObjectFields(nil,
					anyField{key: "kind", value: appendAnyString(nil, "mention")},
				)),
			},
		},
		clientBlock{
			client: 1,
			clock:  2,
			structs: []structEncoding{
				itemAny(rootParent("doc"), appendAnyString(nil, "u")),
			},
		},
		deleteRange{client: 4, clock: 7, length: 2},
	)

	merged, err := MergeUpdatesV1(u1, u2)
	if err != nil {
		t.Fatalf("MergeUpdatesV1() unexpected error: %v", err)
	}

	diff, err := DiffUpdateV1(merged, encodeStateVectorEntry(1, 1, 2, 0, 3, 0, 4, 0))
	if err != nil {
		t.Fatalf("DiffUpdateV1() unexpected error: %v", err)
	}

	ids := NewContentIDs()
	_ = ids.Inserts.Add(1, 1, 2)
	_ = ids.Inserts.Add(2, 0, 1)
	_ = ids.Inserts.Add(3, 0, 1)
	_ = ids.Inserts.Add(4, 0, 1)
	_ = ids.Deletes.Add(1, 12, 1)
	_ = ids.Deletes.Add(2, 20, 1)
	_ = ids.Deletes.Add(4, 8, 1)

	intersection, err := IntersectUpdateWithContentIDsV1(diff, ids)
	if err != nil {
		t.Fatalf("IntersectUpdateWithContentIDsV1() unexpected error: %v", err)
	}

	decoded, err := DecodeV1(intersection)
	if err != nil {
		t.Fatalf("DecodeV1(intersection) unexpected error: %v", err)
	}

	if len(decoded.Structs) != 5 {
		t.Fatalf("len(decoded.Structs) = %d, want 5", len(decoded.Structs))
	}

	assertStruct(t, decoded.Structs, 1, 1, itemContentDoc)
	assertStruct(t, decoded.Structs, 1, 2, itemContentAny)
	assertStruct(t, decoded.Structs, 2, 0, itemContentBinary)
	assertStruct(t, decoded.Structs, 3, 0, itemContentFormat)
	assertStruct(t, decoded.Structs, 4, 0, itemContentEmbed)
	if hasStruct(decoded.Structs, 1, 0) {
		t.Fatalf("Structs=%s\nshould not keep client=1 clock=0 because diff state vector excluded it", describeStructs(decoded.Structs))
	}

	assertDeleteSetRanges(t, decoded.DeleteSet, map[uint32][]ytypes.DeleteRange{
		1: {{Clock: 12, Length: 1}},
		2: {{Clock: 20, Length: 1}},
		4: {{Clock: 8, Length: 1}},
	})
}

func assertStruct(t *testing.T, structs []ytypes.Struct, client, clock uint32, contentRef byte) {
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
			return
		}
	}

	t.Fatalf("Structs=%s\nmissing client=%d clock=%d contentRef=%d", describeStructs(structs), client, clock, contentRef)
}

func hasStruct(structs []ytypes.Struct, client, clock uint32) bool {
	for _, current := range structs {
		item, ok := current.(*ytypes.Item)
		if !ok {
			continue
		}
		if item.ID().Client == client && item.ID().Clock == clock {
			return true
		}
	}
	return false
}
