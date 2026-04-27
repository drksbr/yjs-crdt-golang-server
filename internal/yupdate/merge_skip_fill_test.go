package yupdate

import (
	"bytes"
	"testing"

	"yjs-go-bridge/internal/ytypes"
)

func TestMergeUpdatesV1FillsSyntheticSkipInMiddleOfExistingGap(t *testing.T) {
	t.Parallel()

	base := buildUpdate(
		clientBlock{
			client: 70,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "ab"),
			},
		},
		deleteRange{
			client: 70,
			clock:  100,
			length: 1,
		},
	)
	suffix := buildUpdate(
		clientBlock{
			client: 70,
			clock:  6,
			structs: []structEncoding{
				itemBinary(rootParent("doc"), []byte{0x42}),
			},
		},
		deleteRange{
			client: 70,
			clock:  101,
			length: 1,
		},
	)
	filler := buildUpdate(
		clientBlock{
			client: 70,
			clock:  3,
			structs: []structEncoding{
				itemString(rootParent("doc"), "xy"),
			},
		},
		deleteRange{
			client: 70,
			clock:  102,
			length: 1,
		},
	)

	merged := mergeAndCanonical(t, base, suffix, filler)
	decoded, err := DecodeV1(merged)
	if err != nil {
		t.Fatalf("DecodeV1(merged) unexpected error: %v", err)
	}

	clientStructs := structsForClient(decoded, 70)
	if len(clientStructs) != 5 {
		t.Fatalf("client 70 structs = %d, want 5", len(clientStructs))
	}

	first, ok := clientStructs[0].(*ytypes.Item)
	if !ok {
		t.Fatalf("client 70 first struct = %T, want *ytypes.Item", clientStructs[0])
	}
	firstContent := first.Content.(ParsedContent)
	if first.ID().Clock != 0 || firstContent.ContentRef() != itemContentString || firstContent.Text != "ab" {
		t.Fatalf("client 70 first struct = id=%+v content=%#v, want string ab at clock 0", first.ID(), firstContent)
	}

	firstGap, ok := clientStructs[1].(ytypes.Skip)
	if !ok {
		t.Fatalf("client 70 second struct = %T, want ytypes.Skip", clientStructs[1])
	}
	if firstGap.ID().Clock != 2 || firstGap.Length() != 1 {
		t.Fatalf("client 70 second struct = %#v, want skip at clock 2 len 1", firstGap)
	}

	middle, ok := clientStructs[2].(*ytypes.Item)
	if !ok {
		t.Fatalf("client 70 third struct = %T, want *ytypes.Item", clientStructs[2])
	}
	middleContent := middle.Content.(ParsedContent)
	if middle.ID().Clock != 3 || middleContent.ContentRef() != itemContentString || middleContent.Text != "xy" {
		t.Fatalf("client 70 third struct = id=%+v content=%#v, want string xy at clock 3", middle.ID(), middleContent)
	}

	secondGap, ok := clientStructs[3].(ytypes.Skip)
	if !ok {
		t.Fatalf("client 70 fourth struct = %T, want ytypes.Skip", clientStructs[3])
	}
	if secondGap.ID().Clock != 5 || secondGap.Length() != 1 {
		t.Fatalf("client 70 fourth struct = %#v, want skip at clock 5 len 1", secondGap)
	}

	last, ok := clientStructs[4].(*ytypes.Item)
	if !ok {
		t.Fatalf("client 70 fifth struct = %T, want *ytypes.Item", clientStructs[4])
	}
	lastContent := last.Content.(ParsedContent)
	if last.ID().Clock != 6 || lastContent.ContentRef() != itemContentBinary {
		t.Fatalf("client 70 fifth struct = id=%+v content=%#v, want binary at clock 6", last.ID(), lastContent)
	}

	ranges := decoded.DeleteSet.Ranges(70)
	if len(ranges) != 1 || ranges[0].Clock != 100 || ranges[0].Length != 3 {
		t.Fatalf("client 70 delete ranges = %#v, want [{100 3}]", ranges)
	}

	for _, perm := range permutations([][]byte{base, suffix, filler}) {
		got := mergeAndCanonical(t, perm...)
		if !bytes.Equal(got, merged) {
			t.Fatalf("merge permutation = %v, want %v", got, merged)
		}
	}
}
