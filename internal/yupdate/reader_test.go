package yupdate

import (
	"errors"
	"testing"

	"yjs-go-bridge/internal/ytypes"
)

func TestDecodeV1ReadsStructsAndDeleteSet(t *testing.T) {
	t.Parallel()

	update := buildUpdate(
		clientBlock{
			client: 1,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "hello"),
				gc(2),
				skip(3),
			},
		},
		deleteRange{client: 1, clock: 4, length: 2},
	)

	decoded, err := DecodeV1(update)
	if err != nil {
		t.Fatalf("DecodeV1() unexpected error: %v", err)
	}

	if len(decoded.Structs) != 3 {
		t.Fatalf("len(Structs) = %d, want 3", len(decoded.Structs))
	}

	item, ok := decoded.Structs[0].(*ytypes.Item)
	if !ok {
		t.Fatalf("Structs[0] type = %T, want *ytypes.Item", decoded.Structs[0])
	}
	if item.ID() != (ytypes.ID{Client: 1, Clock: 0}) || item.Length() != 5 {
		t.Fatalf("item = id=%+v len=%d, want client=1 clock=0 len=5", item.ID(), item.Length())
	}

	content, ok := item.Content.(ParsedContent)
	if !ok || content.ContentRef() != itemContentString || content.Length() != 5 {
		t.Fatalf("item content = %#v, want string content len=5", item.Content)
	}

	if decoded.Structs[1].Kind() != ytypes.KindGC || decoded.Structs[1].ID().Clock != 5 {
		t.Fatalf("Structs[1] = %#v, want GC at clock 5", decoded.Structs[1])
	}
	if decoded.Structs[2].Kind() != ytypes.KindSkip || decoded.Structs[2].ID().Clock != 7 {
		t.Fatalf("Structs[2] = %#v, want Skip at clock 7", decoded.Structs[2])
	}

	if !decoded.DeleteSet.Has(ytypes.ID{Client: 1, Clock: 5}) {
		t.Fatalf("DeleteSet should contain client=1 clock=5")
	}
}

func TestLazyReaderV1CanFilterSkips(t *testing.T) {
	t.Parallel()

	update := buildUpdate(
		clientBlock{
			client: 9,
			clock:  0,
			structs: []structEncoding{
				skip(2),
				gc(1),
			},
		},
	)

	reader, err := NewLazyReaderV1(update, true)
	if err != nil {
		t.Fatalf("NewLazyReaderV1() unexpected error: %v", err)
	}

	if reader.Current() == nil || reader.Current().Kind() != ytypes.KindGC {
		t.Fatalf("Current() = %#v, want first non-skip GC", reader.Current())
	}

	if err := reader.Next(); err != nil {
		t.Fatalf("Next() unexpected error: %v", err)
	}
	if reader.Current() != nil {
		t.Fatalf("Current() after end = %#v, want nil", reader.Current())
	}

	ds, err := reader.ReadDeleteSet()
	if err != nil {
		t.Fatalf("ReadDeleteSet() unexpected error: %v", err)
	}
	if ds == nil || !ds.IsEmpty() {
		t.Fatalf("DeleteSet = %#v, want empty set", ds)
	}
}

func TestDecodeV1ParsesAdditionalContentRefs(t *testing.T) {
	t.Parallel()

	docOpts := appendAnyObject(nil, map[string][]byte{
		"autoload": appendAnyBool(nil, true),
		"name":     appendAnyString(nil, "subdoc"),
	})

	update := buildUpdate(
		clientBlock{
			client: 3,
			clock:  0,
			structs: []structEncoding{
				itemDeleted(rootParent("doc"), 3),
				itemType(rootParent("doc"), typeRefYXmlElement, "p"),
				itemDoc(rootParent("doc"), "guid-1", docOpts),
				itemString(rootParent("doc"), "🙂"),
			},
		},
	)

	decoded, err := DecodeV1(update)
	if err != nil {
		t.Fatalf("DecodeV1() unexpected error: %v", err)
	}

	if len(decoded.Structs) != 4 {
		t.Fatalf("len(Structs) = %d, want 4", len(decoded.Structs))
	}

	deleted := decoded.Structs[0].(*ytypes.Item).Content.(ParsedContent)
	if deleted.ContentRef() != itemContentDeleted || deleted.Length() != 3 || deleted.IsCountable() {
		t.Fatalf("deleted content = %#v, want ref=deleted len=3 countable=false", deleted)
	}

	contentType := decoded.Structs[1].(*ytypes.Item).Content.(ParsedContent)
	if contentType.ContentRef() != itemContentType || contentType.EmbeddedType() != typeRefYXmlElement || contentType.TypeName != "p" {
		t.Fatalf("type content = %#v, want xml element named p", contentType)
	}

	doc := decoded.Structs[2].(*ytypes.Item).Content.(ParsedContent)
	if doc.ContentRef() != itemContentDoc || doc.TypeName != "guid-1" {
		t.Fatalf("doc content = %#v, want doc guid guid-1", doc)
	}

	str := decoded.Structs[3].(*ytypes.Item).Content.(ParsedContent)
	if str.ContentRef() != itemContentString || str.Length() != 2 {
		t.Fatalf("string content = %#v, want utf16 length 2", str)
	}
}

func TestEncodeStateVectorFromUpdateV1(t *testing.T) {
	t.Parallel()

	update := buildUpdate(
		clientBlock{
			client: 1,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "ab"),
				gc(1),
			},
		},
		clientBlock{
			client: 2,
			clock:  1,
			structs: []structEncoding{
				itemString(rootParent("doc"), "z"),
			},
		},
	)

	stateVector, err := EncodeStateVectorFromUpdateV1(update)
	if err != nil {
		t.Fatalf("EncodeStateVectorFromUpdateV1() unexpected error: %v", err)
	}

	got, err := decodeStateVector(stateVector)
	if err != nil {
		t.Fatalf("decodeStateVector() unexpected error: %v", err)
	}

	if len(got) != 1 || got[1] != 3 {
		t.Fatalf("state vector = %v, want map[1:3]", got)
	}
}

func TestReadDeleteSetBeforeStructsEndFails(t *testing.T) {
	t.Parallel()

	update := buildUpdate(clientBlock{
		client: 1,
		clock:  0,
		structs: []structEncoding{
			gc(1),
		},
	})

	reader, err := NewLazyReaderV1(update, false)
	if err != nil {
		t.Fatalf("NewLazyReaderV1() unexpected error: %v", err)
	}

	if _, err := reader.ReadDeleteSet(); !errors.Is(err, ErrDeleteSetBeforeStructsEnd) {
		t.Fatalf("ReadDeleteSet() error = %v, want ErrDeleteSetBeforeStructsEnd", err)
	}
}
