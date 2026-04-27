package yupdate

import (
	"bytes"
	"errors"
	"testing"

	"yjs-go-bridge/internal/varint"
	"yjs-go-bridge/internal/yidset"
)

func TestCreateContentIDsFromUpdateV1(t *testing.T) {
	t.Parallel()

	update := buildUpdate(
		clientBlock{
			client: 1,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "ab"),
				gc(1),
				skip(2),
				itemDeleted(rootParent("doc"), 2),
			},
		},
		clientBlock{
			client: 4,
			clock:  7,
			structs: []structEncoding{
				gc(2),
			},
		},
		deleteRange{client: 1, clock: 9, length: 1},
		deleteRange{client: 4, clock: 3, length: 2},
	)

	contentIDs, err := CreateContentIDsFromUpdateV1(update)
	if err != nil {
		t.Fatalf("CreateContentIDsFromUpdateV1() unexpected error: %v", err)
	}

	assertIDSetRanges(t, contentIDs.Inserts, map[uint32][][2]uint32{
		1: {{0, 3}, {5, 2}},
		4: {{7, 2}},
	})
	assertIDSetRanges(t, contentIDs.Deletes, map[uint32][][2]uint32{
		1: {{9, 1}},
		4: {{3, 2}},
	})
}

func TestReadUpdateToContentIDsV1OnlySkipsProducesEmptyInserts(t *testing.T) {
	t.Parallel()

	update := buildUpdate(
		clientBlock{
			client: 9,
			clock:  0,
			structs: []structEncoding{
				skip(2),
				skip(3),
			},
		},
		deleteRange{client: 9, clock: 1, length: 1},
	)

	contentIDs, err := ReadUpdateToContentIDsV1(update)
	if err != nil {
		t.Fatalf("ReadUpdateToContentIDsV1() unexpected error: %v", err)
	}

	if !contentIDs.Inserts.IsEmpty() {
		t.Fatalf("Inserts should be empty, got %#v", contentIDs.Inserts)
	}
	assertIDSetRanges(t, contentIDs.Deletes, map[uint32][][2]uint32{
		9: {{1, 1}},
	})
}

func TestMergeContentIDsTreatsNilAsEmptyAndIsDeterministic(t *testing.T) {
	t.Parallel()

	left := NewContentIDs()
	_ = left.Inserts.Add(3, 10, 2)
	_ = left.Inserts.Add(1, 3, 2)
	_ = left.Inserts.Add(3, 12, 2)
	_ = left.Deletes.Add(2, 5, 1)
	_ = left.Deletes.Add(1, 7, 1)

	right := NewContentIDs()
	_ = right.Inserts.Add(3, 11, 3)
	_ = right.Inserts.Add(2, 4, 1)
	_ = right.Deletes.Add(1, 8, 1)
	_ = right.Deletes.Add(2, 9, 2)

	merged := MergeContentIDs(nil, left, right)
	if merged.Inserts.Ranges(3)[0].Clock != 10 || merged.Inserts.Ranges(3)[0].Length != 4 {
		t.Fatalf("Inserts(3) = %v, want [{10 4}]", merged.Inserts.Ranges(3))
	}
	assertIDSetRanges(t, merged.Inserts, map[uint32][][2]uint32{
		1: {{3, 2}},
		2: {{4, 1}},
		3: {{10, 4}},
	})
	assertIDSetRanges(t, merged.Deletes, map[uint32][][2]uint32{
		1: {{7, 2}},
		2: {{5, 1}, {9, 2}},
	})
}

func TestIntersectContentIDsIsPerFieldAndDeterministic(t *testing.T) {
	t.Parallel()

	left := NewContentIDs()
	_ = left.Inserts.Add(1, 0, 5)
	_ = left.Inserts.Add(2, 10, 2)
	_ = left.Deletes.Add(1, 0, 3)

	right := NewContentIDs()
	_ = right.Inserts.Add(1, 3, 4)
	_ = right.Inserts.Add(2, 11, 1)
	_ = right.Deletes.Add(1, 2, 5)

	got := IntersectContentIDs(left, right)
	assertIDSetRanges(t, got.Inserts, map[uint32][][2]uint32{
		1: {{3, 2}},
		2: {{11, 1}},
	})
	assertIDSetRanges(t, got.Deletes, map[uint32][][2]uint32{
		1: {{2, 1}},
	})
}

func TestDiffContentIDsRemovesRangesIndependentlyPerField(t *testing.T) {
	t.Parallel()
	// [5,5+3) removendo [6,6+1) mantém apenas os clocks 5 e 7 (2 faixas de 1).

	base := NewContentIDs()
	_ = base.Inserts.Add(1, 0, 5)
	_ = base.Inserts.Add(2, 10, 3)
	_ = base.Deletes.Add(1, 5, 3)

	remove := NewContentIDs()
	_ = remove.Inserts.Add(1, 2, 2)
	_ = remove.Inserts.Add(3, 1, 1)
	_ = remove.Deletes.Add(1, 6, 1)

	got := DiffContentIDs(base, remove)
	assertIDSetRanges(t, got.Inserts, map[uint32][][2]uint32{
		1: {{0, 2}, {4, 1}},
		2: {{10, 3}},
	})
	assertIDSetRanges(t, got.Deletes, map[uint32][][2]uint32{
		1: {{5, 1}, {7, 1}},
	})
}

func TestDiffContentIDsReturnsExpectedSuffixForLongerRange(t *testing.T) {
	t.Parallel()
	// [5,5+4) removendo [6,6+1) deve manter o sufixo [7,7+2).

	base := NewContentIDs()
	_ = base.Deletes.Add(1, 5, 4)

	remove := NewContentIDs()
	_ = remove.Deletes.Add(1, 6, 1)

	got := DiffContentIDs(base, remove)
	assertIDSetRanges(t, got.Deletes, map[uint32][][2]uint32{
		1: {{5, 1}, {7, 2}},
	})
}

func TestIsSubsetContentIDsChecksBothFieldsIndependently(t *testing.T) {
	t.Parallel()

	subject := NewContentIDs()
	_ = subject.Inserts.Add(1, 0, 2)
	_ = subject.Deletes.Add(2, 5, 1)

	container := NewContentIDs()
	_ = container.Inserts.Add(1, 0, 3)
	_ = container.Deletes.Add(2, 4, 1)
	_ = container.Deletes.Add(2, 5, 1)

	if !IsSubsetContentIDs(subject, container) {
		t.Fatal("IsSubsetContentIDs() = false, want true")
	}

	subset := subject.Clone()
	_ = subset.Deletes.Add(2, 6, 1)
	if IsSubsetContentIDs(subset, container) {
		t.Fatal("IsSubsetContentIDs() = true, want false when deletes are missing")
	}
}

func TestContentIDsFromUpdatesAggregatesV1PayloadsAndNoopsNilOrEmpty(t *testing.T) {
	t.Parallel()

	left := buildUpdate(
		clientBlock{
			client: 1,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "ab"),
				gc(1),
			},
		},
		deleteRange{client: 1, clock: 3, length: 1},
	)

	right := buildUpdate(
		clientBlock{
			client: 2,
			clock:  4,
			structs: []structEncoding{
				itemDeleted(rootParent("doc"), 2),
			},
		},
		deleteRange{client: 2, clock: 5, length: 1},
	)

	contentIDs, err := ContentIDsFromUpdates(nil, nil, left, []byte{}, right)
	if err != nil {
		t.Fatalf("ContentIDsFromUpdates() unexpected error: %v", err)
	}

	assertIDSetRanges(t, contentIDs.Inserts, map[uint32][][2]uint32{
		1: {{0, 3}},
		2: {{4, 2}},
	})
	assertIDSetRanges(t, contentIDs.Deletes, map[uint32][][2]uint32{
		1: {{3, 1}},
		2: {{5, 1}},
	})
}

func TestEncodeDecodeContentIDsRoundTripAndDeterministicOrder(t *testing.T) {
	t.Parallel()

	c := NewContentIDs()
	_ = c.Inserts.Add(2, 10, 2)
	_ = c.Inserts.Add(1, 5, 3)
	_ = c.Inserts.Add(2, 20, 2)
	_ = c.Deletes.Add(3, 4, 1)
	_ = c.Deletes.Add(3, 9, 1)

	got, err := EncodeContentIDs(c)
	if err != nil {
		t.Fatalf("EncodeContentIDs() unexpected error: %v", err)
	}
	want := []byte{
		2,    // inserts clients
		1, 1, // client 1, 1 range
		5, 3, // clock/length
		2, 2, // client 2, 2 ranges
		10, 2,
		20, 2,
		1,    // deletes clients
		3, 2, // client 3, 2 ranges
		4, 1,
		9, 1,
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("EncodeContentIDs() = %v, want %v", got, want)
	}

	decoded, err := DecodeContentIDs(got)
	if err != nil {
		t.Fatalf("DecodeContentIDs() unexpected error: %v", err)
	}
	if !decoded.Inserts.IsEmpty() && !mergedEqual(decoded.Inserts, c.Inserts) {
		t.Fatalf("decoded.Inserts = %v, want %v", decoded.Inserts, c.Inserts)
	}
	if !decoded.Deletes.IsEmpty() && !mergedEqual(decoded.Deletes, c.Deletes) {
		t.Fatalf("decoded.Deletes = %v, want %v", decoded.Deletes, c.Deletes)
	}
}

func TestDecodeContentIDsRejectsMalformedPayload(t *testing.T) {
	t.Parallel()

	var (
		emptyPayload = []byte{}
		truncated    = []byte{1, 2}
		nonCanonical = []byte{1, 0x80, 0x00}
		invalidZero  = []byte{0, 1, 2, 1, 1, 0}
	)

	cases := []struct {
		name string
		data []byte
		want error
	}{
		{name: "empty payload", data: emptyPayload, want: varint.ErrUnexpectedEOF},
		{name: "truncated payload", data: truncated, want: varint.ErrUnexpectedEOF},
		{name: "non canonical varuint", data: nonCanonical, want: varint.ErrNonCanonical},
		{name: "invalid zero length", data: invalidZero, want: yidset.ErrInvalidLength},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := DecodeContentIDs(tc.data)
			if got != nil {
				t.Fatalf("DecodeContentIDs() ids = %v, want nil on error", got)
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("DecodeContentIDs() error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestDecodeContentIDsRejectsTrailingBytes(t *testing.T) {
	t.Parallel()

	base, err := EncodeContentIDs(NewContentIDs())
	if err != nil {
		t.Fatalf("EncodeContentIDs() unexpected error: %v", err)
	}

	_, err = DecodeContentIDs(append(base, 0x01))
	if !errors.Is(err, ErrContentIDsTrailingBytes) {
		t.Fatalf("DecodeContentIDs() error = %v, want %v", err, ErrContentIDsTrailingBytes)
	}
}

func TestCreateContentIDsFromUpdateV1EmptyUpdate(t *testing.T) {
	t.Parallel()

	contentIDs, err := CreateContentIDsFromUpdateV1(buildUpdate())
	if err != nil {
		t.Fatalf("CreateContentIDsFromUpdateV1() unexpected error: %v", err)
	}

	if !contentIDs.IsEmpty() {
		t.Fatalf("ContentIDs should be empty, got %#v", contentIDs)
	}
}

func mergedEqual(left, right *yidset.IdSet) bool {
	leftClients := left.Clients()
	rightClients := right.Clients()
	if len(leftClients) != len(rightClients) {
		return false
	}

	for i := range leftClients {
		if leftClients[i] != rightClients[i] {
			return false
		}
		leftRanges := left.Ranges(leftClients[i])
		rightRanges := right.Ranges(leftClients[i])
		if len(leftRanges) != len(rightRanges) {
			return false
		}
		for j := range leftRanges {
			if leftRanges[j] != rightRanges[j] {
				return false
			}
		}
	}

	return true
}

func assertIDSetRanges(t *testing.T, set *yidset.IdSet, want map[uint32][][2]uint32) {
	t.Helper()

	gotClients := set.Clients()
	if len(gotClients) != len(want) {
		t.Fatalf("len(Clients) = %d, want %d", len(gotClients), len(want))
	}

	for _, client := range gotClients {
		gotRanges := set.Ranges(client)
		wantRanges, ok := want[client]
		if !ok {
			t.Fatalf("unexpected client %d", client)
		}
		if len(gotRanges) != len(wantRanges) {
			t.Fatalf("len(Ranges(%d)) = %d, want %d", client, len(gotRanges), len(wantRanges))
		}
		for i, got := range gotRanges {
			if got.Clock != wantRanges[i][0] || got.Length != wantRanges[i][1] {
				t.Fatalf("Ranges(%d)[%d] = {%d %d}, want {%d %d}", client, i, got.Clock, got.Length, wantRanges[i][0], wantRanges[i][1])
			}
		}
	}
}
