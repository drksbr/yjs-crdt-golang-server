package yjsbridge

import (
	"bytes"
	"errors"
	"testing"
)

func TestPublicContentIDsRoundTripAndSetOps(t *testing.T) {
	t.Parallel()

	left := NewContentIDs()
	if err := left.AddInsert(1, 0, 3); err != nil {
		t.Fatalf("AddInsert() unexpected error: %v", err)
	}
	if err := left.AddDelete(2, 4, 1); err != nil {
		t.Fatalf("AddDelete() unexpected error: %v", err)
	}

	right := NewContentIDs()
	if err := right.AddInsert(1, 2, 2); err != nil {
		t.Fatalf("AddInsert() unexpected error: %v", err)
	}
	if err := right.AddDelete(2, 4, 1); err != nil {
		t.Fatalf("AddDelete() unexpected error: %v", err)
	}

	merged := MergeContentIDs(left, right)
	if got := merged.InsertRanges(); len(got) != 1 || got[0] != (IDRange{Client: 1, Clock: 0, Length: 4}) {
		t.Fatalf("MergeContentIDs() inserts = %#v, want merged range", got)
	}

	intersection := IntersectContentIDs(left, right)
	if got := intersection.InsertRanges(); len(got) != 1 || got[0] != (IDRange{Client: 1, Clock: 2, Length: 1}) {
		t.Fatalf("IntersectContentIDs() inserts = %#v, want overlap", got)
	}

	diff := DiffContentIDs(left, right)
	if got := diff.InsertRanges(); len(got) != 1 || got[0] != (IDRange{Client: 1, Clock: 0, Length: 2}) {
		t.Fatalf("DiffContentIDs() inserts = %#v, want prefix range", got)
	}
	if !IsSubsetContentIDs(intersection, merged) {
		t.Fatalf("IsSubsetContentIDs() = false, want true")
	}

	payload, err := EncodeContentIDs(merged)
	if err != nil {
		t.Fatalf("EncodeContentIDs() unexpected error: %v", err)
	}
	decoded, err := DecodeContentIDs(payload)
	if err != nil {
		t.Fatalf("DecodeContentIDs() unexpected error: %v", err)
	}
	if !bytes.Equal(mustEncodeContentIDs(t, decoded), payload) {
		t.Fatalf("Decode/Encode content ids roundtrip mismatch")
	}
}

func TestPublicContentIDsContracts(t *testing.T) {
	t.Parallel()

	var nilContentIDs *ContentIDs

	if _, err := DecodeContentIDs(nil); !errors.Is(err, ErrInvalidContentIDsPayload) {
		t.Fatalf("DecodeContentIDs(nil) error = %v, want %v", err, ErrInvalidContentIDsPayload)
	}
	if err := nilContentIDs.AddInsert(1, 0, 1); !errors.Is(err, ErrNilContentIDs) {
		t.Fatalf("AddInsert(nil) error = %v, want %v", err, ErrNilContentIDs)
	}
	if err := nilContentIDs.AddDelete(1, 0, 1); !errors.Is(err, ErrNilContentIDs) {
		t.Fatalf("AddDelete(nil) error = %v, want %v", err, ErrNilContentIDs)
	}

	contentIDs := NewContentIDs()
	if err := contentIDs.AddInsert(1, 0, 0); !errors.Is(err, ErrInvalidRangeLength) {
		t.Fatalf("AddInsert() error = %v, want %v", err, ErrInvalidRangeLength)
	}
	if err := contentIDs.AddDelete(1, ^uint32(0), 2); !errors.Is(err, ErrRangeOverflow) {
		t.Fatalf("AddDelete() error = %v, want %v", err, ErrRangeOverflow)
	}
}

func mustEncodeContentIDs(t *testing.T, contentIDs *ContentIDs) []byte {
	t.Helper()

	payload, err := EncodeContentIDs(contentIDs)
	if err != nil {
		t.Fatalf("EncodeContentIDs() unexpected error: %v", err)
	}
	return payload
}
