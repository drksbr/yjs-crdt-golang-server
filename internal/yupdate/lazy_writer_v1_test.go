package yupdate

import (
	"testing"

	"yjs-go-bridge/internal/ytypes"
)

func TestLazyWriterV1WritesClientFragmentsWithOffsets(t *testing.T) {
	t.Parallel()

	update := buildUpdate(
		clientBlock{
			client: 9,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "hello"),
				gc(1),
			},
		},
		clientBlock{
			client: 3,
			clock:  1,
			structs: []structEncoding{
				itemDeleted(rootParent("doc"), 2),
			},
		},
	)

	decoded, err := DecodeV1(update)
	if err != nil {
		t.Fatalf("DecodeV1() unexpected error: %v", err)
	}

	writer := newLazyWriterV1()
	if err := writer.write(decoded.Structs[0], 1, 1); err != nil {
		t.Fatalf("writer.write(first) unexpected error: %v", err)
	}
	skip, err := ytypes.NewSkip(ytypes.ID{Client: 9, Clock: 4}, 1)
	if err != nil {
		t.Fatalf("NewSkip() unexpected error: %v", err)
	}
	if err := writer.write(skip, 0, 0); err != nil {
		t.Fatalf("writer.write(skip) unexpected error: %v", err)
	}
	if err := writer.write(decoded.Structs[1], 0, 0); err != nil {
		t.Fatalf("writer.write(second) unexpected error: %v", err)
	}
	if err := writer.write(decoded.Structs[2], 0, 0); err != nil {
		t.Fatalf("writer.write(third) unexpected error: %v", err)
	}

	structBlock, err := writer.finish(nil)
	if err != nil {
		t.Fatalf("writer.finish() unexpected error: %v", err)
	}

	got, err := DecodeV1(AppendDeleteSetBlockV1(structBlock, ytypes.NewDeleteSet()))
	if err != nil {
		t.Fatalf("DecodeV1(writer output) unexpected error: %v", err)
	}

	if len(got.Structs) != 4 {
		t.Fatalf("len(Structs) = %d, want 4", len(got.Structs))
	}

	first, ok := got.Structs[0].(*ytypes.Item)
	if !ok {
		t.Fatalf("Structs[0] type = %T, want *ytypes.Item", got.Structs[0])
	}
	firstContent := first.Content.(ParsedContent)
	if first.ID().Client != 9 || first.ID().Clock != 1 || firstContent.Text != "ell" {
		t.Fatalf("Structs[0] = id=%+v content=%#v, want client=9 clock=1 text=ell", first.ID(), firstContent)
	}

	if got.Structs[1].Kind() != ytypes.KindSkip || got.Structs[1].ID() != (ytypes.ID{Client: 9, Clock: 4}) {
		t.Fatalf("Structs[1] = %#v, want Skip at client=9 clock=4", got.Structs[1])
	}

	if got.Structs[2].Kind() != ytypes.KindGC || got.Structs[2].ID() != (ytypes.ID{Client: 9, Clock: 5}) {
		t.Fatalf("Structs[2] = %#v, want GC at client=9 clock=5", got.Structs[2])
	}

	third, ok := got.Structs[3].(*ytypes.Item)
	if !ok {
		t.Fatalf("Structs[3] type = %T, want *ytypes.Item", got.Structs[3])
	}
	if third.ID() != (ytypes.ID{Client: 3, Clock: 1}) || third.Length() != 2 {
		t.Fatalf("Structs[3] = id=%+v len=%d, want client=3 clock=1 len=2", third.ID(), third.Length())
	}
}

func TestLazyWriterV1AcceptsClientOrderAsEncountered(t *testing.T) {
	t.Parallel()

	update := buildUpdate(
		clientBlock{
			client: 3,
			clock:  0,
			structs: []structEncoding{
				itemDeleted(rootParent("doc"), 1),
			},
		},
		clientBlock{
			client: 9,
			clock:  0,
			structs: []structEncoding{
				itemDeleted(rootParent("doc"), 1),
			},
		},
	)

	decoded, err := DecodeV1(update)
	if err != nil {
		t.Fatalf("DecodeV1() unexpected error: %v", err)
	}

	writer := newLazyWriterV1()
	if err := writer.write(decoded.Structs[0], 0, 0); err != nil {
		t.Fatalf("writer.write(first) unexpected error: %v", err)
	}
	if err := writer.write(decoded.Structs[1], 0, 0); err != nil {
		t.Fatalf("writer.write(second) unexpected error: %v", err)
	}

	structBlock, err := writer.finish(nil)
	if err != nil {
		t.Fatalf("writer.finish() unexpected error: %v", err)
	}

	got, err := DecodeV1(AppendDeleteSetBlockV1(structBlock, ytypes.NewDeleteSet()))
	if err != nil {
		t.Fatalf("DecodeV1(writer output) unexpected error: %v", err)
	}

	if len(got.Structs) != 2 {
		t.Fatalf("len(Structs) = %d, want 2", len(got.Structs))
	}
	if got.Structs[0].ID() != (ytypes.ID{Client: 3, Clock: 0}) {
		t.Fatalf("Structs[0] = %#v, want client 3 clock 0", got.Structs[0])
	}
	if got.Structs[1].ID() != (ytypes.ID{Client: 9, Clock: 0}) {
		t.Fatalf("Structs[1] = %#v, want client 9 clock 0", got.Structs[1])
	}
}

func TestLazyWriterV1FinishResetsWriterForReuse(t *testing.T) {
	t.Parallel()

	update := buildUpdate(
		clientBlock{
			client: 9,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "ab"),
			},
		},
		clientBlock{
			client: 3,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "xy"),
			},
		},
	)

	decoded, err := DecodeV1(update)
	if err != nil {
		t.Fatalf("DecodeV1() unexpected error: %v", err)
	}

	writer := newLazyWriterV1()
	if err := writer.write(decoded.Structs[0], 0, 0); err != nil {
		t.Fatalf("writer.write(first pass) unexpected error: %v", err)
	}
	firstBlock, err := writer.finish(nil)
	if err != nil {
		t.Fatalf("writer.finish(first pass) unexpected error: %v", err)
	}

	firstDecoded, err := DecodeV1(AppendDeleteSetBlockV1(firstBlock, ytypes.NewDeleteSet()))
	if err != nil {
		t.Fatalf("DecodeV1(firstBlock) unexpected error: %v", err)
	}
	if len(firstDecoded.Structs) != 1 || firstDecoded.Structs[0].ID() != (ytypes.ID{Client: 9, Clock: 0}) {
		t.Fatalf("firstDecoded.Structs = %#v, want only client 9", firstDecoded.Structs)
	}

	if err := writer.write(decoded.Structs[1], 0, 0); err != nil {
		t.Fatalf("writer.write(second pass) unexpected error: %v", err)
	}
	secondBlock, err := writer.finish(nil)
	if err != nil {
		t.Fatalf("writer.finish(second pass) unexpected error: %v", err)
	}

	secondDecoded, err := DecodeV1(AppendDeleteSetBlockV1(secondBlock, ytypes.NewDeleteSet()))
	if err != nil {
		t.Fatalf("DecodeV1(secondBlock) unexpected error: %v", err)
	}
	if len(secondDecoded.Structs) != 1 || secondDecoded.Structs[0].ID() != (ytypes.ID{Client: 3, Clock: 0}) {
		t.Fatalf("secondDecoded.Structs = %#v, want only client 3", secondDecoded.Structs)
	}
}

func TestLazyWriterV1PreservesSliceMetadata(t *testing.T) {
	t.Parallel()

	update := buildUpdate(
		clientBlock{
			client: 8,
			clock:  0,
			structs: []structEncoding{
				itemStringWithOptions(itemWireOptions{
					origin:      idPtr(8, 41),
					rightOrigin: idPtr(2, 9),
				}, "hello"),
				itemJSONWithOptions(itemWireOptions{
					parent:    idParent(4, 7),
					parentSub: "title",
				}, `"a"`, `"b"`, `"c"`),
			},
		},
	)

	decoded, err := DecodeV1(update)
	if err != nil {
		t.Fatalf("DecodeV1() unexpected error: %v", err)
	}

	writer := newLazyWriterV1()
	if err := writer.write(decoded.Structs[0], 2, 1); err != nil {
		t.Fatalf("writer.write(string slice) unexpected error: %v", err)
	}
	if err := writer.write(decoded.Structs[1], 0, 1); err != nil {
		t.Fatalf("writer.write(json slice) unexpected error: %v", err)
	}

	structBlock, err := writer.finish(nil)
	if err != nil {
		t.Fatalf("writer.finish() unexpected error: %v", err)
	}

	got, err := DecodeV1(AppendDeleteSetBlockV1(structBlock, ytypes.NewDeleteSet()))
	if err != nil {
		t.Fatalf("DecodeV1(writer output) unexpected error: %v", err)
	}

	if len(got.Structs) != 2 {
		t.Fatalf("len(Structs) = %d, want 2", len(got.Structs))
	}

	first, ok := got.Structs[0].(*ytypes.Item)
	if !ok {
		t.Fatalf("Structs[0] type = %T, want *ytypes.Item", got.Structs[0])
	}
	firstContent := first.Content.(ParsedContent)
	if first.ID() != (ytypes.ID{Client: 8, Clock: 2}) || firstContent.Text != "ll" {
		t.Fatalf("Structs[0] = id=%+v content=%#v, want client=8 clock=2 text=ll", first.ID(), firstContent)
	}
	if first.Origin == nil || *first.Origin != (ytypes.ID{Client: 8, Clock: 1}) {
		t.Fatalf("first.Origin = %+v, want {Client:8 Clock:1}", first.Origin)
	}
	if first.RightOrigin == nil || *first.RightOrigin != (ytypes.ID{Client: 2, Clock: 9}) {
		t.Fatalf("first.RightOrigin = %+v, want {Client:2 Clock:9}", first.RightOrigin)
	}

	second, ok := got.Structs[1].(*ytypes.Item)
	if !ok {
		t.Fatalf("Structs[1] type = %T, want *ytypes.Item", got.Structs[1])
	}
	secondContent := second.Content.(ParsedContent)
	if second.ID() != (ytypes.ID{Client: 8, Clock: 4}) || len(secondContent.JSON) != 2 || secondContent.JSON[0] != `"a"` || secondContent.JSON[1] != `"b"` {
		t.Fatalf("Structs[1] = id=%+v content=%#v, want client=8 clock=4 JSON [\"a\",\"b\"]", second.ID(), secondContent)
	}
	parentID, ok := second.Parent.ID()
	if !ok || parentID != (ytypes.ID{Client: 4, Clock: 7}) {
		t.Fatalf("second.Parent = %+v, want parent id {Client:4 Clock:7}", second.Parent)
	}
	if second.ParentSub != "title" {
		t.Fatalf("second.ParentSub = %q, want title", second.ParentSub)
	}
}
