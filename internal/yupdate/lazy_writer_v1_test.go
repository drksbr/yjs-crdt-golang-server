package yupdate

import (
	"errors"
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

func TestLazyWriterV1RejectsClientReentry(t *testing.T) {
	t.Parallel()

	update := buildUpdate(
		clientBlock{
			client: 9,
			clock:  0,
			structs: []structEncoding{
				itemDeleted(rootParent("doc"), 1),
			},
		},
		clientBlock{
			client: 3,
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
	if err := writer.write(decoded.Structs[0], 0, 0); !errors.Is(err, ErrInvalidClientOrder) {
		t.Fatalf("writer.write(client reentry) error = %v, want ErrInvalidClientOrder", err)
	}
}
