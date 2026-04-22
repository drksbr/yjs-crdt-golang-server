package yupdate

import (
	"bytes"
	"errors"
	"testing"

	"yjs-go-bridge/internal/varint"
	"yjs-go-bridge/internal/ytypes"
)

func TestAppendDeleteSetBlockV1WritesDeterministicLayout(t *testing.T) {
	t.Parallel()

	ds := ytypes.NewDeleteSet()
	mustAddDeleteRange(t, ds, 2, 5, 1)
	mustAddDeleteRange(t, ds, 2, 9, 2)
	mustAddDeleteRange(t, ds, 9, 1, 1)

	got := EncodeDeleteSetBlockV1(ds)
	want := []byte{
		2,          // clientes
		9, 1, 1, 1, // client=9, 1 range: [1,1]
		2, 2, 5, 1, 9, 2, // client=2, 2 ranges: [5,1] e [9,2]
	}

	if !bytes.Equal(got, want) {
		t.Fatalf("EncodeDeleteSetBlockV1() = %v, want %v", got, want)
	}
}

func TestDecodeDeleteSetBlockV1RoundTripAndConsumesOnlyBlock(t *testing.T) {
	t.Parallel()

	ds := ytypes.NewDeleteSet()
	mustAddDeleteRange(t, ds, 10, 3, 4)
	mustAddDeleteRange(t, ds, 4, 8, 2)
	mustAddDeleteRange(t, ds, 4, 20, 1)

	payload := append(EncodeDeleteSetBlockV1(ds), 0xaa, 0xbb)

	got, consumed, err := DecodeDeleteSetBlockV1(payload)
	if err != nil {
		t.Fatalf("DecodeDeleteSetBlockV1() unexpected error: %v", err)
	}
	if consumed != len(payload)-2 {
		t.Fatalf("consumed = %d, want %d", consumed, len(payload)-2)
	}

	assertDeleteSetRanges(t, got, map[uint32][]ytypes.DeleteRange{
		4: {
			{Clock: 8, Length: 2},
			{Clock: 20, Length: 1},
		},
		10: {
			{Clock: 3, Length: 4},
		},
	})
}

func TestDecodeDeleteSetBlockV1RejectsMalformedData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data []byte
		want error
	}{
		{
			name: "unexpected eof",
			data: []byte{1, 9, 1, 7},
			want: varint.ErrUnexpectedEOF,
		},
		{
			name: "non canonical varuint",
			data: []byte{1, 0x80, 0x00},
			want: varint.ErrNonCanonical,
		},
		{
			name: "invalid zero length",
			data: []byte{1, 9, 1, 7, 0},
			want: ytypes.ErrInvalidLength,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, _, err := DecodeDeleteSetBlockV1(tc.data)
			if got != nil {
				t.Fatalf("DecodeDeleteSetBlockV1() ds = %v, want nil on error", got)
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("DecodeDeleteSetBlockV1() error = %v, want %v", err, tc.want)
			}
		})
	}
}

func mustAddDeleteRange(t *testing.T, ds *ytypes.DeleteSet, client, clock, length uint32) {
	t.Helper()

	if err := ds.Add(client, clock, length); err != nil {
		t.Fatalf("Add(%d, %d, %d) unexpected error: %v", client, clock, length, err)
	}
}

func assertDeleteSetRanges(t *testing.T, ds *ytypes.DeleteSet, want map[uint32][]ytypes.DeleteRange) {
	t.Helper()

	gotClients := ds.Clients()
	if len(gotClients) != len(want) {
		t.Fatalf("Clients() len = %d, want %d", len(gotClients), len(want))
	}

	for client, wantRanges := range want {
		gotRanges := ds.Ranges(client)
		if len(gotRanges) != len(wantRanges) {
			t.Fatalf("Ranges(%d) len = %d, want %d", client, len(gotRanges), len(wantRanges))
		}
		for i := range wantRanges {
			if gotRanges[i] != wantRanges[i] {
				t.Fatalf("Ranges(%d)[%d] = %+v, want %+v", client, i, gotRanges[i], wantRanges[i])
			}
		}
	}
}
