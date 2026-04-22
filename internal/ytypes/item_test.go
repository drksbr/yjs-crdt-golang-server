package ytypes

import (
	"errors"
	"testing"
)

type stubContent struct {
	length    uint32
	countable bool
}

func (c stubContent) Length() uint32 {
	return c.length
}

func (c stubContent) IsCountable() bool {
	return c.countable
}

func TestNewItemDerivesLengthAndCountableBit(t *testing.T) {
	t.Parallel()

	item, err := NewItem(
		ID{Client: 7, Clock: 100},
		stubContent{length: 4, countable: false},
		ItemOptions{
			Flags: ItemFlagDeleted | ItemFlagMarker | ItemFlagCountable,
		},
	)
	if err != nil {
		t.Fatalf("NewItem() unexpected error: %v", err)
	}

	if item.Kind() != KindItem {
		t.Fatalf("Kind() = %v, want KindItem", item.Kind())
	}
	if item.Length() != 4 {
		t.Fatalf("Length() = %d, want 4", item.Length())
	}
	if item.Countable() {
		t.Fatalf("Countable() = true, want false")
	}
	if !item.Deleted() || !item.Marker() {
		t.Fatalf("Deleted()/Marker() did not preserve the requested flags")
	}
}

func TestNewItemCopiesIDsAndSupportsFlagMutation(t *testing.T) {
	t.Parallel()

	origin := ID{Client: 1, Clock: 2}
	rightOrigin := ID{Client: 3, Clock: 4}
	redone := ID{Client: 5, Clock: 6}

	item, err := NewItem(
		ID{Client: 9, Clock: 10},
		stubContent{length: 2, countable: true},
		ItemOptions{
			Origin:      &origin,
			RightOrigin: &rightOrigin,
			Parent:      mustParentRoot(t, "doc"),
			ParentSub:   "title",
			Redone:      &redone,
			Flags:       ItemFlagKeep,
		},
	)
	if err != nil {
		t.Fatalf("NewItem() unexpected error: %v", err)
	}

	origin.Clock = 999
	rightOrigin.Clock = 999
	redone.Clock = 999

	if item.Origin == nil || item.Origin.Clock != 2 {
		t.Fatalf("Origin = %+v, want copied clock 2", item.Origin)
	}
	if item.RightOrigin == nil || item.RightOrigin.Clock != 4 {
		t.Fatalf("RightOrigin = %+v, want copied clock 4", item.RightOrigin)
	}
	if item.Redone == nil || item.Redone.Clock != 6 {
		t.Fatalf("Redone = %+v, want copied clock 6", item.Redone)
	}
	if item.Parent.Kind() != ParentRoot || item.Parent.Root() != "doc" || item.ParentSub != "title" {
		t.Fatalf("Parent = %+v / ParentSub = %q, want root=doc sub=title", item.Parent, item.ParentSub)
	}
	if !item.Keep() || !item.Countable() {
		t.Fatalf("Keep()/Countable() should both be true")
	}

	item.SetKeep(false)
	item.SetMarker(true)
	item.SetDeleted(false)
	item.MarkDeleted()

	if item.Keep() {
		t.Fatalf("Keep() = true, want false")
	}
	if !item.Marker() || !item.Deleted() {
		t.Fatalf("Marker()/Deleted() should both be true after mutation")
	}
}

func TestNewItemRejectsNilOrEmptyContent(t *testing.T) {
	t.Parallel()

	_, err := NewItem(ID{Client: 1, Clock: 1}, nil, ItemOptions{})
	if !errors.Is(err, ErrNilContent) {
		t.Fatalf("NewItem(nil) error = %v, want ErrNilContent", err)
	}

	_, err = NewItem(ID{Client: 1, Clock: 1}, stubContent{length: 0, countable: true}, ItemOptions{})
	if !errors.Is(err, ErrInvalidLength) {
		t.Fatalf("NewItem(zero length) error = %v, want ErrInvalidLength", err)
	}
}

func TestParentRefHelpers(t *testing.T) {
	t.Parallel()

	empty := ParentRef{}
	if !empty.IsZero() {
		t.Fatalf("zero ParentRef should be empty")
	}

	parentID := NewParentID(ID{Client: 11, Clock: 12})
	gotID, ok := parentID.ID()
	if parentID.Kind() != ParentID || !ok || gotID != (ID{Client: 11, Clock: 12}) {
		t.Fatalf("NewParentID() = %+v / id=%+v / ok=%v, want parent id 11/12", parentID, gotID, ok)
	}
}

func TestItemHeaderRoundTrip(t *testing.T) {
	t.Parallel()

	info, err := (ItemHeader{
		ContentRef:     7,
		HasOrigin:      true,
		HasRightOrigin: true,
		HasParentSub:   true,
	}).Encode()
	if err != nil {
		t.Fatalf("ItemHeader.Encode() unexpected error: %v", err)
	}

	got := DecodeItemHeader(info)
	if got.ContentRef != 7 || !got.HasOrigin || !got.HasRightOrigin || !got.HasParentSub {
		t.Fatalf("DecodeItemHeader() = %+v, want all fields preserved", got)
	}
}

func TestItemHeaderRejectsInvalidContentRef(t *testing.T) {
	t.Parallel()

	_, err := (ItemHeader{ContentRef: 32}).Encode()
	if !errors.Is(err, errInvalidContentRef) {
		t.Fatalf("ItemHeader.Encode() error = %v, want errInvalidContentRef", err)
	}
}

func mustParentRoot(t *testing.T, root string) ParentRef {
	t.Helper()

	parent, err := NewParentRoot(root)
	if err != nil {
		t.Fatalf("NewParentRoot(%q) unexpected error: %v", root, err)
	}

	return parent
}
