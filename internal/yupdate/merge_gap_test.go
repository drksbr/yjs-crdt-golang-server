package yupdate

import (
	"bytes"
	"testing"

	"github.com/drksbr/yjs-crdt-golang-server/internal/ytypes"
)

func TestMergeUpdatesV1PreservesMixedStructuralTypesAcrossSyntheticGap(t *testing.T) {
	t.Parallel()

	left := buildUpdate(
		clientBlock{
			client: 17,
			clock:  0,
			structs: []structEncoding{
				itemType(rootParent("doc"), typeRefYText, ""),
				itemBinary(rootParent("doc"), []byte{0xde, 0xad}),
				itemFormat(rootParent("doc"), "bold", appendAnyBool(nil, true)),
			},
		},
	)
	right := buildUpdate(
		clientBlock{
			client: 17,
			clock:  5,
			structs: []structEncoding{
				itemDoc(rootParent("doc"), "guid-mix", appendAnyString(nil, "subdoc")),
				itemAny(rootParent("doc"), appendAnyString(nil, "alpha"), appendAnyBool(nil, false)),
			},
		},
	)

	anyFirst := appendAnyString(nil, "alpha")
	anySecond := appendAnyBool(nil, false)

	cases := []struct {
		name  string
		left  []byte
		right []byte
	}{
		{
			name:  "ordered_updates",
			left:  left,
			right: right,
		},
		{
			name:  "reversed_updates",
			left:  right,
			right: left,
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			merged, err := MergeUpdatesV1(tt.left, tt.right)
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

			prefixType, ok := decoded.Structs[0].(*ytypes.Item)
			if !ok {
				t.Fatalf("Structs[0] type = %T, want *ytypes.Item", decoded.Structs[0])
			}
			typeContent := prefixType.Content.(ParsedContent)
			if prefixType.ID() != (ytypes.ID{Client: 17, Clock: 0}) {
				t.Fatalf("Structs[0].ID() = %+v, want {Client:17 Clock:0}", prefixType.ID())
			}
			if typeContent.ContentRef() != itemContentType || typeContent.EmbeddedType() != typeRefYText {
				t.Fatalf("Structs[0] content = %#v, want type content for YText", typeContent)
			}

			prefixBinary, ok := decoded.Structs[1].(*ytypes.Item)
			if !ok {
				t.Fatalf("Structs[1] type = %T, want *ytypes.Item", decoded.Structs[1])
			}
			binaryContent := prefixBinary.Content.(ParsedContent)
			if prefixBinary.ID() != (ytypes.ID{Client: 17, Clock: 1}) {
				t.Fatalf("Structs[1].ID() = %+v, want {Client:17 Clock:1}", prefixBinary.ID())
			}
			if binaryContent.ContentRef() != itemContentBinary {
				t.Fatalf("Structs[1] content = %#v, want binary content", binaryContent)
			}

			prefixFormat, ok := decoded.Structs[2].(*ytypes.Item)
			if !ok {
				t.Fatalf("Structs[2] type = %T, want *ytypes.Item", decoded.Structs[2])
			}
			formatContent := prefixFormat.Content.(ParsedContent)
			if prefixFormat.ID() != (ytypes.ID{Client: 17, Clock: 2}) {
				t.Fatalf("Structs[2].ID() = %+v, want {Client:17 Clock:2}", prefixFormat.ID())
			}
			if formatContent.ContentRef() != itemContentFormat || formatContent.TypeName != "bold" || formatContent.IsCountable() {
				t.Fatalf("Structs[2] content = %#v, want bold format", formatContent)
			}

			skip, ok := decoded.Structs[3].(ytypes.Skip)
			if !ok {
				t.Fatalf("Structs[3] type = %T, want ytypes.Skip", decoded.Structs[3])
			}
			if skip.ID().Clock != 3 || skip.Length() != 2 {
				t.Fatalf("Structs[3] = %#v, want synthetic skip at clock 3 len 2", skip)
			}

			suffixDoc, ok := decoded.Structs[4].(*ytypes.Item)
			if !ok {
				t.Fatalf("Structs[4] type = %T, want *ytypes.Item", decoded.Structs[4])
			}
			docContent := suffixDoc.Content.(ParsedContent)
			if suffixDoc.ID() != (ytypes.ID{Client: 17, Clock: 5}) {
				t.Fatalf("Structs[4].ID() = %+v, want {Client:17 Clock:5}", suffixDoc.ID())
			}
			if docContent.ContentRef() != itemContentDoc || docContent.TypeName != "guid-mix" {
				t.Fatalf("Structs[4] content = %#v, want doc content with guid-mix", docContent)
			}

			suffixAny, ok := decoded.Structs[5].(*ytypes.Item)
			if !ok {
				t.Fatalf("Structs[5] type = %T, want *ytypes.Item", decoded.Structs[5])
			}
			anyContent := suffixAny.Content.(ParsedContent)
			if suffixAny.ID() != (ytypes.ID{Client: 17, Clock: 6}) {
				t.Fatalf("Structs[5].ID() = %+v, want {Client:17 Clock:6}", suffixAny.ID())
			}
			if anyContent.ContentRef() != itemContentAny || len(anyContent.Any) != 2 {
				t.Fatalf("Structs[5] content = %#v, want any content with two values", anyContent)
			}
			if !bytes.Equal(anyContent.Any[0], anyFirst) || !bytes.Equal(anyContent.Any[1], anySecond) {
				t.Fatalf("Structs[5].Any = %#v, want %#v and %#v", anyContent.Any, anyFirst, anySecond)
			}
		})
	}
}
