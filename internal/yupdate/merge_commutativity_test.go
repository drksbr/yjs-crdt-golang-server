package yupdate

import (
	"bytes"
	"testing"

	"yjs-go-bridge/internal/ytypes"
)

func TestMergeUpdatesV1CommutativeAndAssociativeWithOverlaps(t *testing.T) {
	t.Parallel()

	firstAny := appendAnyString(nil, "alpha")
	secondAny := appendAnyBool(nil, true)
	thirdAny := appendAnyString(nil, "omega")
	docAnyOpts := appendAnyObject(nil, map[string][]byte{
		"kind": appendAnyString(nil, "meta"),
	})

	first := buildUpdate(
		clientBlock{
			client: 41,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "abcd"),
			},
		},
		clientBlock{
			client: 42,
			clock:  0,
			structs: []structEncoding{
				itemType(rootParent("doc"), typeRefYXmlElement, "p"),
			},
		},
		deleteRange{
			client: 41,
			clock:  200,
			length: 1,
		},
	)
	second := buildUpdate(
		clientBlock{
			client: 41,
			clock:  2,
			structs: []structEncoding{
				itemAny(rootParent("doc"), firstAny, secondAny, thirdAny),
			},
		},
		clientBlock{
			client: 42,
			clock:  1,
			structs: []structEncoding{
				itemFormat(rootParent("doc"), "bold", appendAnyBool(nil, true)),
			},
		},
		deleteRange{
			client: 42,
			clock:  120,
			length: 2,
		},
	)
	third := buildUpdate(
		clientBlock{
			client: 41,
			clock:  6,
			structs: []structEncoding{
				itemDoc(rootParent("doc"), "guid-41", docAnyOpts),
			},
		},
		clientBlock{
			client: 42,
			clock:  3,
			structs: []structEncoding{
				itemBinary(rootParent("doc"), []byte{0xDE, 0xAD, 0xBE, 0xEF}),
			},
		},
		deleteRange{
			client: 42,
			clock:  122,
			length: 1,
		},
	)

	tests := []struct {
		name    string
		updates [][]byte
	}{
		{
			name:    "multi_client_overlapping_refs",
			updates: [][]byte{first, second, third},
		},
		{
			name:    "duplicate_merge_inputs_stability",
			updates: [][]byte{first, second, third, first},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			merged := mergeAndCanonical(t, tt.updates...)
			decoded, err := DecodeV1(merged)
			if err != nil {
				t.Fatalf("DecodeV1(merged) unexpected error: %v", err)
			}

			client41 := structsForClient(decoded, 41)
			if len(client41) != 4 {
				t.Fatalf("client 41 structs = %d, want 4", len(client41))
			}
			stringItem := client41[0].(*ytypes.Item)
			stringContent := stringItem.Content.(ParsedContent)
			if stringItem.ID().Client != 41 || stringItem.ID().Clock != 0 || stringContent.ContentRef() != itemContentString || stringContent.Text != "abcd" {
				t.Fatalf("client 41 first struct = %#v, want string at clock 0 text abcd", client41[0])
			}
			anyItem := client41[1].(*ytypes.Item)
			anyContent := anyItem.Content.(ParsedContent)
			if anyItem.ID().Client != 41 || anyItem.ID().Clock != 4 || anyContent.ContentRef() != itemContentAny || len(anyContent.Any) != 1 {
				t.Fatalf("client 41 second struct = %#v, want 1-value any at clock 4", client41[1])
			}
			if !bytes.Equal(anyContent.Any[0], thirdAny) {
				t.Fatalf("client 41 any value = %#v, want thirdAny", anyContent.Any)
			}
			skipItem, ok := client41[2].(ytypes.Skip)
			if !ok {
				t.Fatalf("client 41 third struct = %T, want ytypes.Skip", client41[2])
			}
			if skipItem.ID().Clock != 5 || skipItem.Length() != 1 {
				t.Fatalf("client 41 skip = %#v, want skip at clock 5 len 1", skipItem)
			}
			docItem := client41[3].(*ytypes.Item)
			docContent := docItem.Content.(ParsedContent)
			if docItem.ID().Client != 41 || docItem.ID().Clock != 6 || docContent.ContentRef() != itemContentDoc || docContent.TypeName != "guid-41" {
				t.Fatalf("client 41 fourth struct = %#v, want doc clock 6", client41[3])
			}

			client42 := structsForClient(decoded, 42)
			if len(client42) != 4 {
				t.Fatalf("client 42 structs = %d, want 4", len(client42))
			}
			typeItem := client42[0].(*ytypes.Item)
			typeContent := typeItem.Content.(ParsedContent)
			if typeItem.ID().Client != 42 || typeItem.ID().Clock != 0 || typeContent.ContentRef() != itemContentType || typeContent.TypeName != "p" {
				t.Fatalf("client 42 first struct = %#v, want type xml element at clock 0", client42[0])
			}
			formatItem := client42[1].(*ytypes.Item)
			formatContent := formatItem.Content.(ParsedContent)
			if formatItem.ID().Client != 42 || formatItem.ID().Clock != 1 || formatContent.ContentRef() != itemContentFormat || formatContent.TypeName != "bold" || formatContent.IsCountable() {
				t.Fatalf("client 42 second struct = %#v, want format at clock 1", client42[1])
			}
			skipItem42, ok := client42[2].(ytypes.Skip)
			if !ok {
				t.Fatalf("client 42 third struct = %T, want ytypes.Skip", client42[2])
			}
			if skipItem42.ID().Client != 42 || skipItem42.ID().Clock != 2 || skipItem42.Length() != 1 {
				t.Fatalf("client 42 skip = %#v, want skip at clock 2 len 1", skipItem42)
			}
			binaryItem := client42[3].(*ytypes.Item)
			binaryContent := binaryItem.Content.(ParsedContent)
			if binaryItem.ID().Client != 42 || binaryItem.ID().Clock != 3 || binaryContent.ContentRef() != itemContentBinary {
				t.Fatalf("client 42 fourth struct = %#v, want binary at clock 3", client42[3])
			}

			if len(decoded.DeleteSet.Clients()) != 2 {
				t.Fatalf("delete set clients = %v, want two clients", decoded.DeleteSet.Clients())
			}
			r41 := decoded.DeleteSet.Ranges(41)
			if len(r41) != 1 || r41[0].Clock != 200 || r41[0].Length != 1 {
				t.Fatalf("client 41 delete ranges = %#v, want [{200 1}]", r41)
			}
			r42 := decoded.DeleteSet.Ranges(42)
			if len(r42) != 1 || r42[0].Clock != 120 || r42[0].Length != 3 {
				t.Fatalf("client 42 delete ranges = %#v, want [{120 3}]", r42)
			}

			unique := mergeAndCanonical(t, first, second, third)
			if !bytes.Equal(merged, unique) {
				t.Fatalf("merged with duplicates = %v, want unique merge = %v", merged, unique)
			}

			for _, perm := range permutations(tt.updates) {
				got := mergeAndCanonical(t, perm...)
				if !bytes.Equal(got, merged) {
					t.Fatalf("merged permutation = %v, want %v", got, merged)
				}
			}

			for _, perm := range permutations([][]byte{first, second, third}) {
				leftAssoc := mergeAndCanonical(t, mergeAndCanonical(t, perm[0], perm[1]), perm[2])
				if !bytes.Equal(leftAssoc, merged) {
					t.Fatalf("left-associative merge = %v, want %v", leftAssoc, merged)
				}
				rightAssoc := mergeAndCanonical(t, perm[0], mergeAndCanonical(t, perm[1], perm[2]))
				if !bytes.Equal(rightAssoc, merged) {
					t.Fatalf("right-associative merge = %v, want %v", rightAssoc, merged)
				}
			}
		})
	}
}

func TestMergeUpdatesV1CommutativityAndRefsSecondPattern(t *testing.T) {
	t.Parallel()

	firstA := appendAnyString(nil, "left")
	firstB := appendAnyString(nil, "middle")
	firstC := appendAnyString(nil, "right")

	first := buildUpdate(
		clientBlock{
			client: 50,
			clock:  0,
			structs: []structEncoding{
				itemAny(rootParent("doc"), firstA, firstB, firstC),
			},
		},
		clientBlock{
			client: 60,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "alpha"),
			},
		},
		deleteRange{
			client: 50,
			clock:  150,
			length: 1,
		},
	)

	second := buildUpdate(
		clientBlock{
			client: 50,
			clock:  1,
			structs: []structEncoding{
				itemString(rootParent("doc"), "XYZ"),
			},
		},
		clientBlock{
			client: 60,
			clock:  2,
			structs: []structEncoding{
				itemFormat(rootParent("doc"), "italic", appendAnyBool(nil, true)),
			},
		},
		deleteRange{
			client: 60,
			clock:  90,
			length: 2,
		},
	)

	third := buildUpdate(
		clientBlock{
			client: 50,
			clock:  5,
			structs: []structEncoding{
				itemBinary(rootParent("doc"), []byte{0x01}),
			},
		},
		clientBlock{
			client: 60,
			clock:  5,
			structs: []structEncoding{
				itemDoc(rootParent("doc"), "sub-doc-60", appendAnyString(nil, "v")),
			},
		},
		deleteRange{
			client: 60,
			clock:  92,
			length: 1,
		},
	)

	updates := [][]byte{first, second, third}
	merged := mergeAndCanonical(t, updates...)
	decoded, err := DecodeV1(merged)
	if err != nil {
		t.Fatalf("DecodeV1(merged) unexpected error: %v", err)
	}

	client50 := structsForClient(decoded, 50)
	if len(client50) != 4 {
		t.Fatalf("client 50 structs = %d, want 4", len(client50))
	}
	anyItem := client50[0].(*ytypes.Item)
	anyContent := anyItem.Content.(ParsedContent)
	if anyItem.ID().Client != 50 || anyItem.ID().Clock != 0 || anyContent.ContentRef() != itemContentAny || len(anyContent.Any) != 3 {
		t.Fatalf("client 50 first struct = %#v, want any at clock 0 with 3 values", client50[0])
	}
	if !equalAnyValues(anyContent.Any, firstA, firstB, firstC) {
		t.Fatalf("client 50 any values = %#v, want [left middle right]", anyContent.Any)
	}
	stringItem := client50[1].(*ytypes.Item)
	stringContent := stringItem.Content.(ParsedContent)
	if stringItem.ID().Client != 50 || stringItem.ID().Clock != 3 || stringContent.ContentRef() != itemContentString || stringContent.Text != "Z" {
		t.Fatalf("client 50 second struct = %#v, want suffix string at clock 3 text Z", client50[1])
	}
	skipItem := client50[2].(ytypes.Skip)
	if skipItem.ID().Clock != 4 || skipItem.Length() != 1 {
		t.Fatalf("client 50 third struct = %#v, want skip at clock 4 len 1", client50[2])
	}
	binaryItem := client50[3].(*ytypes.Item)
	binaryContent := binaryItem.Content.(ParsedContent)
	if binaryItem.ID().Client != 50 || binaryItem.ID().Clock != 5 || binaryContent.ContentRef() != itemContentBinary {
		t.Fatalf("client 50 fourth struct = %#v, want binary at clock 5", client50[3])
	}

	client60 := structsForClient(decoded, 60)
	if len(client60) != 2 {
		t.Fatalf("client 60 structs = %d, want 2", len(client60))
	}
	stringItem60 := client60[0].(*ytypes.Item)
	stringContent60 := stringItem60.Content.(ParsedContent)
	if stringItem60.ID().Client != 60 || stringItem60.ID().Clock != 0 || stringContent60.ContentRef() != itemContentString || stringContent60.Text != "alpha" {
		t.Fatalf("client 60 first struct = %#v, want string at clock 0", client60[0])
	}
	docItem60 := client60[1].(*ytypes.Item)
	docContent60 := docItem60.Content.(ParsedContent)
	if docItem60.ID().Client != 60 || docItem60.ID().Clock != 5 || docContent60.ContentRef() != itemContentDoc || docContent60.TypeName != "sub-doc-60" {
		t.Fatalf("client 60 second struct = %#v, want doc at clock 5", client60[1])
	}

	r50 := decoded.DeleteSet.Ranges(50)
	if len(r50) != 1 || r50[0].Clock != 150 || r50[0].Length != 1 {
		t.Fatalf("client 50 delete ranges = %#v, want [{150 1}]", r50)
	}
	r60 := decoded.DeleteSet.Ranges(60)
	if len(r60) != 1 || r60[0].Clock != 90 || r60[0].Length != 3 {
		t.Fatalf("client 60 delete ranges = %#v, want [{90 3}]", r60)
	}

	for _, perm := range permutations(updates) {
		got := mergeAndCanonical(t, perm...)
		if !bytes.Equal(got, merged) {
			t.Fatalf("merge permutation = %v, want %v", got, merged)
		}
	}
}

func mergeAndCanonical(t *testing.T, updates ...[]byte) []byte {
	t.Helper()

	merged, err := MergeUpdatesV1(updates...)
	if err != nil {
		t.Fatalf("MergeUpdatesV1() unexpected error: %v", err)
	}

	decoded, err := DecodeV1(merged)
	if err != nil {
		t.Fatalf("DecodeV1(merged) unexpected error: %v", err)
	}

	canonical, err := EncodeV1(decoded)
	if err != nil {
		t.Fatalf("EncodeV1(merged decoded) unexpected error: %v", err)
	}
	return canonical
}

func structsForClient(decoded *DecodedUpdate, client uint32) []ytypes.Struct {
	out := make([]ytypes.Struct, 0)
	for _, current := range decoded.Structs {
		if current.ID().Client == client {
			out = append(out, current)
		}
	}
	return out
}

func permutations[T any](values []T) [][]T {
	n := len(values)
	if n == 0 {
		return nil
	}

	out := make([][]T, 0)
	current := make([]T, 0, n)
	used := make([]bool, n)

	var backtrack func()
	backtrack = func() {
		if len(current) == n {
			next := make([]T, n)
			copy(next, current)
			out = append(out, next)
			return
		}
		for i := 0; i < n; i++ {
			if used[i] {
				continue
			}
			used[i] = true
			current = append(current, values[i])
			backtrack()
			current = current[:len(current)-1]
			used[i] = false
		}
	}
	backtrack()

	return out
}

func equalAnyValues(values [][]byte, expected ...[]byte) bool {
	if len(values) != len(expected) {
		return false
	}
	for i := range values {
		if !bytes.Equal(values[i], expected[i]) {
			return false
		}
	}
	return true
}
