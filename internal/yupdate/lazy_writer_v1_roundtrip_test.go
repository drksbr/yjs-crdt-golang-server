package yupdate

import (
	"testing"

	"yjs-go-bridge/internal/ytypes"
)

type lazyWriterWriteStep struct {
	structIndex int
	startOffset uint32
	endTrim     uint32
}

func TestLazyWriterV1RoundTripIntegrationCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		update     func() []byte
		writes     []lazyWriterWriteStep
		assertions func(*testing.T, *DecodedUpdate)
	}{
		{
			name: "multi-client with delete set and mixed structs",
			update: func() []byte {
				return buildUpdate(
					clientBlock{
						client: 21,
						clock:  0,
						structs: []structEncoding{
							itemStringWithOptions(itemWireOptions{
								parent:    rootParent("doc"),
								parentSub: "content",
							}, "hello"),
							gc(1),
						},
					},
					clientBlock{
						client: 13,
						clock:  0,
						structs: []structEncoding{
							itemAny(rootParent("doc"), appendAnyString(nil, "a"), appendAnyBool(nil, true), appendAnyString(nil, "b")),
						},
					},
					deleteRange{client: 21, clock: 2, length: 1},
					deleteRange{client: 13, clock: 1, length: 1},
				)
			},
			writes: []lazyWriterWriteStep{
				{structIndex: 0},
				{structIndex: 1},
				{structIndex: 2},
			},
			assertions: func(t *testing.T, decoded *DecodedUpdate) {
				t.Helper()
				if len(decoded.Structs) != 3 {
					t.Fatalf("len(Structs) = %d, want 3", len(decoded.Structs))
				}

				first, ok := decoded.Structs[0].(*ytypes.Item)
				if !ok {
					t.Fatalf("Structs[0] type = %T, want *ytypes.Item", decoded.Structs[0])
				}
				if first.ParentSub != "content" {
					t.Fatalf("first.ParentSub = %q, want content", first.ParentSub)
				}

				second, ok := decoded.Structs[1].(ytypes.GC)
				if !ok {
					t.Fatalf("Structs[1] type = %T, want ytypes.GC", decoded.Structs[1])
				}
				if second.ID() != (ytypes.ID{Client: 21, Clock: 5}) {
					t.Fatalf("Structs[1] = %+v, want GC at client 21 clock 5", second.ID())
				}

				if !decoded.DeleteSet.Has(ytypes.ID{Client: 21, Clock: 2}) || !decoded.DeleteSet.Has(ytypes.ID{Client: 13, Clock: 1}) {
					t.Fatalf("DeleteSet = %#v, want client 21 clock 2 and client 13 clock 1", decoded.DeleteSet)
				}
				if decoded.DeleteSet.Has(ytypes.ID{Client: 21, Clock: 1}) || decoded.DeleteSet.Has(ytypes.ID{Client: 13, Clock: 0}) {
					t.Fatalf("DeleteSet = %#v, want only selected ranges", decoded.DeleteSet)
				}
			},
		},
		{
			name: "sliced writes preserve windows and round-trip encode/decode",
			update: func() []byte {
				return buildUpdate(
					clientBlock{
						client: 7,
						clock:  0,
						structs: []structEncoding{
							itemStringWithOptions(itemWireOptions{
								parent: rootParent("doc"),
							}, "hello"),
							itemAny(rootParent("doc"), appendAnyString(nil, "x"), appendAnyString(nil, "y"), appendAnyBool(nil, true)),
						},
					},
					clientBlock{
						client: 3,
						clock:  0,
						structs: []structEncoding{
							itemJSON(rootParent("doc"), `"a"`, `"b"`),
						},
					},
				)
			},
			writes: []lazyWriterWriteStep{
				{structIndex: 0, startOffset: 2, endTrim: 1},
				{structIndex: 1, startOffset: 0, endTrim: 1},
				{structIndex: 2},
			},
			assertions: func(t *testing.T, decoded *DecodedUpdate) {
				t.Helper()
				if len(decoded.Structs) != 3 {
					t.Fatalf("len(Structs) = %d, want 3", len(decoded.Structs))
				}

				sliced, ok := decoded.Structs[0].(*ytypes.Item)
				if !ok {
					t.Fatalf("Structs[0] type = %T, want *ytypes.Item", decoded.Structs[0])
				}
				content := sliced.Content.(ParsedContent)
				if content.ContentRef() != itemContentString || content.Length() == 0 {
					t.Fatalf("first content = %#v, want non-empty string content", content)
				}

				trimmed, ok := decoded.Structs[1].(*ytypes.Item)
				if !ok {
					t.Fatalf("Structs[1] type = %T, want *ytypes.Item", decoded.Structs[1])
				}
				trimmedContent := trimmed.Content.(ParsedContent)
				if trimmedContent.ContentRef() != itemContentAny || len(trimmedContent.Any) != 2 {
					t.Fatalf("Structs[1] = %#v content=%#v, want any with len 2", decoded.Structs[1], trimmedContent)
				}

				third, ok := decoded.Structs[2].(*ytypes.Item)
				if !ok {
					t.Fatalf("Structs[2] type = %T, want *ytypes.Item", decoded.Structs[2])
				}
				thirdContent := third.Content.(ParsedContent)
				if thirdContent.ContentRef() != itemContentJSON || len(thirdContent.JSON) != 2 {
					t.Fatalf("Structs[2] = %#v content=%#v, want json with len 2", third, thirdContent)
				}
			},
		},
		{
			name: "utf16 boundary slices preserve round-trip",
			update: func() []byte {
				return buildUpdate(
					clientBlock{
						client: 42,
						clock:  0,
						structs: []structEncoding{
							itemString(rootParent("doc"), "🙂a"),
						},
					},
				)
			},
			writes: []lazyWriterWriteStep{
				{structIndex: 0, endTrim: 2},
			},
			assertions: func(t *testing.T, decoded *DecodedUpdate) {
				t.Helper()
				if len(decoded.Structs) != 1 {
					t.Fatalf("len(Structs) = %d, want 1", len(decoded.Structs))
				}

				item, ok := decoded.Structs[0].(*ytypes.Item)
				if !ok {
					t.Fatalf("Structs[0] type = %T, want *ytypes.Item", decoded.Structs[0])
				}
				content := item.Content.(ParsedContent)
				if content.Text != "�" {
					t.Fatalf("Structs[0].text = %q, want replacement char", content.Text)
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			update := tt.update()
			decoded, err := DecodeV1(update)
			if err != nil {
				t.Fatalf("DecodeV1() unexpected error: %v", err)
			}

			writer := newLazyWriterV1()
			for _, step := range tt.writes {
				if step.structIndex < 0 || step.structIndex >= len(decoded.Structs) {
					t.Fatalf("invalid structIndex=%d for update with %d structs", step.structIndex, len(decoded.Structs))
				}
				if err := writer.write(decoded.Structs[step.structIndex], step.startOffset, step.endTrim); err != nil {
					t.Fatalf("writer.write(index=%d, start=%d, trim=%d) unexpected error: %v", step.structIndex, step.startOffset, step.endTrim, err)
				}
			}

			structBlock, err := writer.finish(nil)
			if err != nil {
				t.Fatalf("writer.finish() unexpected error: %v", err)
			}

			output := AppendDeleteSetBlockV1(structBlock, decoded.DeleteSet)
			roundTrip, err := DecodeV1(output)
			if err != nil {
				t.Fatalf("DecodeV1(writer output) unexpected error: %v", err)
			}

			assertEncodedV1RoundTripMatches(t, output, roundTrip)

			if tt.assertions != nil {
				tt.assertions(t, roundTrip)
			}
		})
	}
}
