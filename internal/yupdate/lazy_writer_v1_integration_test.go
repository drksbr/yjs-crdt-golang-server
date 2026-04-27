package yupdate

import (
	"bytes"
	"testing"

	"yjs-go-bridge/internal/ytypes"
)

func TestLazyWriterV1IntegrationRoundTripsDiffAndIntersectOutputs(t *testing.T) {
	t.Parallel()

	t.Run("diff preserves sliced metadata through encode", func(t *testing.T) {
		t.Parallel()

		update := buildUpdate(
			clientBlock{
				client: 22,
				clock:  0,
				structs: []structEncoding{
					itemStringWithOptions(itemWireOptions{
						origin:      idPtr(1, 3),
						rightOrigin: idPtr(2, 9),
					}, "hello"),
					itemAny(rootParent("doc"),
						appendAnyString(nil, "x"),
						appendAnyBool(nil, true),
					),
				},
			},
			deleteRange{client: 22, clock: 30, length: 2},
		)

		got, err := DiffUpdateV1(update, encodeStateVectorEntry(22, 2))
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
		firstContent := first.Content.(ParsedContent)
		if first.ID() != (ytypes.ID{Client: 22, Clock: 2}) || firstContent.ContentRef() != itemContentString || firstContent.Text != "llo" {
			t.Fatalf("Structs[0] = id=%+v content=%#v, want client=22 clock=2 text=llo", first.ID(), firstContent)
		}
		if first.Origin == nil || *first.Origin != (ytypes.ID{Client: 22, Clock: 1}) {
			t.Fatalf("first.Origin = %+v, want {Client:22 Clock:1}", first.Origin)
		}
		if first.RightOrigin == nil || *first.RightOrigin != (ytypes.ID{Client: 2, Clock: 9}) {
			t.Fatalf("first.RightOrigin = %+v, want {Client:2 Clock:9}", first.RightOrigin)
		}

		second, ok := decoded.Structs[1].(*ytypes.Item)
		if !ok {
			t.Fatalf("Structs[1] type = %T, want *ytypes.Item", decoded.Structs[1])
		}
		secondContent := second.Content.(ParsedContent)
		if second.ID() != (ytypes.ID{Client: 22, Clock: 5}) || secondContent.ContentRef() != itemContentAny || len(secondContent.Any) != 2 {
			t.Fatalf("Structs[1] = id=%+v content=%#v, want client=22 clock=5 any len 2", second.ID(), secondContent)
		}

		if !decoded.DeleteSet.Has(ytypes.ID{Client: 22, Clock: 30}) || !decoded.DeleteSet.Has(ytypes.ID{Client: 22, Clock: 31}) {
			t.Fatalf("DeleteSet = %#v, want client 22 clock 30 and 31", decoded.DeleteSet)
		}

		assertEncodedV1RoundTripMatches(t, got, decoded)
	})

	t.Run("intersect preserves synthetic skip and parent metadata through encode", func(t *testing.T) {
		t.Parallel()

		update := buildUpdate(
			clientBlock{
				client: 33,
				clock:  0,
				structs: []structEncoding{
					itemString(rootParent("doc"), "ab"),
					itemJSONWithOptions(itemWireOptions{
						parent:    idParent(4, 7),
						parentSub: "title",
					}, `"x"`, `"y"`),
				},
			},
			deleteRange{client: 33, clock: 20, length: 2},
			deleteRange{client: 34, clock: 5, length: 1},
		)

		ids := NewContentIDs()
		_ = ids.Inserts.Add(33, 1, 1)
		_ = ids.Inserts.Add(33, 3, 1)
		_ = ids.Deletes.Add(33, 21, 1)

		got, err := IntersectUpdateWithContentIDsV1(update, ids)
		if err != nil {
			t.Fatalf("IntersectUpdateWithContentIDsV1() unexpected error: %v", err)
		}

		decoded, err := DecodeV1(got)
		if err != nil {
			t.Fatalf("DecodeV1(intersection) unexpected error: %v", err)
		}

		if len(decoded.Structs) != 3 {
			t.Fatalf("len(Structs) = %d, want 3", len(decoded.Structs))
		}

		first, ok := decoded.Structs[0].(*ytypes.Item)
		if !ok {
			t.Fatalf("Structs[0] type = %T, want *ytypes.Item", decoded.Structs[0])
		}
		firstContent := first.Content.(ParsedContent)
		if first.ID() != (ytypes.ID{Client: 33, Clock: 1}) || firstContent.ContentRef() != itemContentString || firstContent.Text != "b" {
			t.Fatalf("Structs[0] = id=%+v content=%#v, want client=33 clock=1 text=b", first.ID(), firstContent)
		}

		skip, ok := decoded.Structs[1].(ytypes.Skip)
		if !ok {
			t.Fatalf("Structs[1] type = %T, want ytypes.Skip", decoded.Structs[1])
		}
		if skip.ID() != (ytypes.ID{Client: 33, Clock: 2}) || skip.Length() != 1 {
			t.Fatalf("Structs[1] = %#v, want skip at client=33 clock=2 len=1", skip)
		}

		third, ok := decoded.Structs[2].(*ytypes.Item)
		if !ok {
			t.Fatalf("Structs[2] type = %T, want *ytypes.Item", decoded.Structs[2])
		}
		thirdContent := third.Content.(ParsedContent)
		if third.ID() != (ytypes.ID{Client: 33, Clock: 3}) || thirdContent.ContentRef() != itemContentJSON || len(thirdContent.JSON) != 1 || thirdContent.JSON[0] != `"y"` {
			t.Fatalf("Structs[2] = id=%+v content=%#v, want client=33 clock=3 JSON [\"y\"]", third.ID(), thirdContent)
		}
		if third.Origin == nil || *third.Origin != (ytypes.ID{Client: 33, Clock: 2}) {
			t.Fatalf("third.Origin = %+v, want {Client:33 Clock:2}", third.Origin)
		}
		if third.Parent.Kind() != ytypes.ParentNone {
			t.Fatalf("third.Parent = %+v, want ParentNone after sliced item gains origin", third.Parent)
		}
		if third.ParentSub != "" {
			t.Fatalf("third.ParentSub = %q, want empty after sliced item gains origin", third.ParentSub)
		}

		if !decoded.DeleteSet.Has(ytypes.ID{Client: 33, Clock: 21}) {
			t.Fatalf("DeleteSet = %#v, want client 33 clock 21", decoded.DeleteSet)
		}
		if decoded.DeleteSet.Has(ytypes.ID{Client: 33, Clock: 20}) || decoded.DeleteSet.Has(ytypes.ID{Client: 34, Clock: 5}) {
			t.Fatalf("DeleteSet = %#v, want only the selected delete", decoded.DeleteSet)
		}

		assertEncodedV1RoundTripMatches(t, got, decoded)
	})
}

func assertEncodedV1RoundTripMatches(t *testing.T, want []byte, decoded *DecodedUpdate) {
	t.Helper()

	encoded, err := EncodeV1(decoded)
	if err != nil {
		t.Fatalf("EncodeV1() unexpected error: %v", err)
	}
	if !bytes.Equal(encoded, want) {
		t.Fatalf("EncodeV1(decoded) = %v, want %v", encoded, want)
	}
}
