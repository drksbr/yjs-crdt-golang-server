package yupdate

import (
	"fmt"
	"testing"

	"github.com/drksbr/yjs-crdt-golang-server/internal/ytypes"
)

func TestMeta9MultiClientMergeDiffIntersectWorkflow(t *testing.T) {
	t.Parallel()

	u1 := buildUpdate(
		clientBlock{
			client: 1,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "ab"),
			},
		},
		clientBlock{
			client: 2,
			clock:  0,
			structs: []structEncoding{
				itemBinary(rootParent("doc"), []byte{0x01}),
			},
		},
	)
	u2 := buildUpdate(
		clientBlock{
			client: 1,
			clock:  2,
			structs: []structEncoding{
				itemString(rootParent("doc"), "cd"),
			},
		},
		clientBlock{
			client: 2,
			clock:  1,
			structs: []structEncoding{
				itemAny(rootParent("doc"), appendAnyString(nil, "x"), appendAnyBool(nil, true)),
			},
		},
	)

	merged, err := MergeUpdatesV1(u1, u2)
	if err != nil {
		t.Fatalf("MergeUpdatesV1() unexpected error: %v", err)
	}

	decodedMerged, err := DecodeV1(merged)
	if err != nil {
		t.Fatalf("DecodeV1(merged) unexpected error: %v", err)
	}
	if len(decodedMerged.Structs) != 4 {
		t.Fatalf("len(decodedMerged.Structs) = %d, want 4", len(decodedMerged.Structs))
	}

	assertBinaryStruct(t, decodedMerged.Structs[0], 2, 0, []byte{0x01})
	assertAnyStructLen(t, decodedMerged.Structs[1], 2, 1, 2)
	assertStringStruct(t, decodedMerged.Structs[2], 1, 0, "ab")
	assertStringStruct(t, decodedMerged.Structs[3], 1, 2, "cd")

	diff, err := DiffUpdateV1(merged, encodeStateVectorEntry(1, 1, 2, 1))
	if err != nil {
		t.Fatalf("DiffUpdateV1() unexpected error: %v", err)
	}

	decodedDiff, err := DecodeV1(diff)
	if err != nil {
		t.Fatalf("DecodeV1(diff) unexpected error: %v", err)
	}
	if len(decodedDiff.Structs) != 3 {
		t.Fatalf("len(decodedDiff.Structs) = %d, want 3", len(decodedDiff.Structs))
	}

	assertAnyStructLen(t, decodedDiff.Structs[0], 2, 1, 2)
	assertStringStruct(t, decodedDiff.Structs[1], 1, 1, "b")
	assertStringStruct(t, decodedDiff.Structs[2], 1, 2, "cd")

	ids := NewContentIDs()
	_ = ids.Inserts.Add(2, 1, 1)
	_ = ids.Inserts.Add(1, 1, 2)

	intersection, err := IntersectUpdateWithContentIDsV1(merged, ids)
	if err != nil {
		t.Fatalf("IntersectUpdateWithContentIDsV1() unexpected error: %v", err)
	}

	decodedIntersection, err := DecodeV1(intersection)
	if err != nil {
		t.Fatalf("DecodeV1(intersection) unexpected error: %v", err)
	}
	if len(decodedIntersection.Structs) != 2 {
		t.Fatalf("len(decodedIntersection.Structs) = %d, want 2; structs=%s", len(decodedIntersection.Structs), describeStructs(decodedIntersection.Structs))
	}

	// O client 2 é descartado porque o range selecionado começa exatamente na
	// struct seguinte do client sem escrita prévia, preservando a semântica
	// observada no caminho atual de intersect.
	assertStringStruct(t, decodedIntersection.Structs[0], 1, 1, "b")
	assertStringStruct(t, decodedIntersection.Structs[1], 1, 2, "c")
}

func assertBinaryStruct(t *testing.T, current ytypes.Struct, client, clock uint32, want []byte) {
	t.Helper()

	item, ok := current.(*ytypes.Item)
	if !ok {
		t.Fatalf("struct type = %T, want *ytypes.Item", current)
	}
	content := item.Content.(ParsedContent)
	if item.ID().Client != client || item.ID().Clock != clock || content.ContentRef() != itemContentBinary {
		t.Fatalf("item = id=%+v content=%#v, want binary client=%d clock=%d", item.ID(), content, client, clock)
	}
	if len(content.Raw) != len(want)+1 {
		t.Fatalf("binary raw len = %d, want %d", len(content.Raw), len(want)+1)
	}
	for i, b := range want {
		if content.Raw[i+1] != b {
			t.Fatalf("binary[%d] = 0x%x, want 0x%x", i, content.Raw[i+1], b)
		}
	}
}

func assertAnyStructLen(t *testing.T, current ytypes.Struct, client, clock uint32, wantLen int) {
	t.Helper()

	item, ok := current.(*ytypes.Item)
	if !ok {
		t.Fatalf("struct type = %T, want *ytypes.Item", current)
	}
	content := item.Content.(ParsedContent)
	if item.ID().Client != client || item.ID().Clock != clock || content.ContentRef() != itemContentAny || len(content.Any) != wantLen {
		t.Fatalf("item = id=%+v content=%#v, want any client=%d clock=%d len=%d", item.ID(), content, client, clock, wantLen)
	}
}

func describeStructs(structs []ytypes.Struct) string {
	parts := make([]string, 0, len(structs))
	for _, current := range structs {
		item, ok := current.(*ytypes.Item)
		if !ok {
			parts = append(parts, fmt.Sprintf("%T{id=%+v len=%d}", current, current.ID(), current.Length()))
			continue
		}
		content, _ := item.Content.(ParsedContent)
		parts = append(parts, fmt.Sprintf("Item{id=%+v ref=%d text=%q any=%d raw=%x}", item.ID(), content.ContentRef(), content.Text, len(content.Any), content.Raw))
	}
	return fmt.Sprintf("%v", parts)
}
