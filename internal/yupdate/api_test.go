package yupdate

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/drksbr/yjs-crdt-golang-server/internal/varint"
)

func TestDecodeUpdateDispatchesV1(t *testing.T) {
	t.Parallel()

	update := buildUpdate(
		clientBlock{
			client: 3,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "hello"),
			},
		},
	)

	got, err := DecodeUpdate(update)
	if err != nil {
		t.Fatalf("DecodeUpdate() unexpected error: %v", err)
	}

	encoded, err := EncodeUpdate(got)
	if err != nil {
		t.Fatalf("EncodeUpdate() unexpected error: %v", err)
	}

	if !bytes.Equal(encoded, update) {
		t.Fatalf("DecodeUpdate() decode+encode roundtrip = %v, want %v", encoded, update)
	}
}

func TestMergeUpdatesDispatchesV1(t *testing.T) {
	t.Parallel()

	left := buildUpdate(
		clientBlock{
			client: 4,
			clock:  0,
			structs: []structEncoding{
				gc(1),
			},
		},
	)
	right := buildUpdate(
		clientBlock{
			client: 4,
			clock:  1,
			structs: []structEncoding{
				itemString(rootParent("doc"), "x"),
			},
		},
	)

	got, err := MergeUpdates(left, right)
	if err != nil {
		t.Fatalf("MergeUpdates() unexpected error: %v", err)
	}

	expected, err := MergeUpdatesV1(left, right)
	if err != nil {
		t.Fatalf("MergeUpdatesV1() unexpected error: %v", err)
	}

	if !bytes.Equal(got, expected) {
		t.Fatalf("MergeUpdates() = %v, want %v", got, expected)
	}
}

func TestMergeUpdatesContextDispatchesV1(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	left := buildUpdate(
		clientBlock{
			client: 4,
			clock:  0,
			structs: []structEncoding{
				gc(1),
			},
		},
	)
	right := buildUpdate(
		clientBlock{
			client: 4,
			clock:  1,
			structs: []structEncoding{
				itemString(rootParent("doc"), "x"),
			},
		},
	)

	got, err := MergeUpdatesContext(ctx, left, right)
	if err != nil {
		t.Fatalf("MergeUpdatesContext() unexpected error: %v", err)
	}

	expected, err := MergeUpdatesV1Context(ctx, left, right)
	if err != nil {
		t.Fatalf("MergeUpdatesV1Context() unexpected error: %v", err)
	}

	if !bytes.Equal(got, expected) {
		t.Fatalf("MergeUpdatesContext() = %v, want %v", got, expected)
	}
}

func TestMergeUpdatesAPIValidation(t *testing.T) {
	t.Parallel()

	v1Left := buildUpdate(
		clientBlock{
			client: 4,
			clock:  0,
			structs: []structEncoding{
				gc(1),
			},
		},
	)
	v1Right := buildUpdate(
		clientBlock{
			client: 4,
			clock:  1,
			structs: []structEncoding{
				itemString(rootParent("doc"), "x"),
			},
		},
	)
	v2Update := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	tests := []struct {
		name        string
		updates     [][]byte
		wantErr     error
		assertValue func(*testing.T, []byte)
	}{
		{
			name:    "dispatch_v1_updates",
			updates: [][]byte{v1Left, v1Right},
			assertValue: func(t *testing.T, got []byte) {
				expected, err := MergeUpdatesV1(v1Left, v1Right)
				if err != nil {
					t.Fatalf("MergeUpdatesV1() unexpected error: %v", err)
				}
				if !bytes.Equal(got, expected) {
					t.Fatalf("MergeUpdates() = %v, want %v", got, expected)
				}
			},
		},
		{
			name:    "empty_argument_list",
			updates: nil,
			assertValue: func(t *testing.T, got []byte) {
				expected := encodeEmptyUpdateV1()
				if !bytes.Equal(got, expected) {
					t.Fatalf("MergeUpdates() = %v, want %v", got, expected)
				}
			},
		},
		{
			name:    "all_empty_payloads_are_noop",
			updates: [][]byte{nil, []byte{}},
			assertValue: func(t *testing.T, got []byte) {
				expected := encodeEmptyUpdateV1()
				if !bytes.Equal(got, expected) {
					t.Fatalf("MergeUpdates() = %v, want %v", got, expected)
				}
			},
		},
		{
			name:    "mixed_v1_and_v2",
			updates: [][]byte{v1Left, v2Update},
			wantErr: ErrMismatchedUpdateFormats,
		},
		{
			name:        "empty_payload_rejected",
			updates:     [][]byte{v1Left, nil},
			wantErr:     ErrUnknownUpdateFormat,
			assertValue: nil,
		},
		{
			name:    "malformed_payload_rejected",
			updates: [][]byte{v1Left, []byte{0x80}},
			wantErr: varint.ErrUnexpectedEOF,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := MergeUpdates(tt.updates...)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("MergeUpdates() error = nil, want %v", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("MergeUpdates() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("MergeUpdates() unexpected error: %v", err)
			}

			if tt.assertValue != nil {
				tt.assertValue(t, got)
			}
		})
	}
}

func TestFormatFromUpdatePublicAPI(t *testing.T) {
	t.Parallel()

	v1Update := buildUpdate(
		clientBlock{
			client: 3,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "z"),
			},
		},
	)
	v2Update := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	tests := []struct {
		name    string
		input   []byte
		want    UpdateFormat
		wantErr error
	}{
		{
			name:  "v1",
			input: v1Update,
			want:  UpdateFormatV1,
		},
		{
			name:  "v2",
			input: v2Update,
			want:  UpdateFormatV2,
		},
		{
			name:    "empty_payload",
			input:   nil,
			want:    UpdateFormatUnknown,
			wantErr: ErrUnknownUpdateFormat,
		},
		{
			name:    "zero_prefix_v1",
			input:   []byte{0, 0},
			want:    UpdateFormatV1,
			wantErr: nil,
		},
		{
			name:    "broken_varint",
			input:   []byte{0x80},
			want:    UpdateFormatUnknown,
			wantErr: varint.ErrUnexpectedEOF,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := FormatFromUpdate(tt.input)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("FormatFromUpdate() error = %v, want %v", err, tt.wantErr)
				}
				if got != tt.want {
					t.Fatalf("FormatFromUpdate() = %s, want %s", got, tt.want)
				}
				return
			}

			if err != nil {
				t.Fatalf("FormatFromUpdate() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("FormatFromUpdate() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestValidateUpdateDecodesDetectedPayload(t *testing.T) {
	t.Parallel()

	v1 := buildUpdate(
		clientBlock{
			client: 9,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "valid"),
			},
		},
	)
	v2 := mustDecodeHex(t, "000002a50100000104060374686901020101000001010000")
	minimalDetectedV2 := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	tests := []struct {
		name    string
		update  []byte
		wantErr error
	}{
		{name: "valid_v1", update: v1},
		{name: "valid_v2", update: v2},
		{name: "detected_but_malformed_v2", update: minimalDetectedV2, wantErr: varint.ErrUnexpectedEOF},
		{name: "unknown_empty", update: nil, wantErr: ErrUnknownUpdateFormat},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateUpdate(tt.update)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("ValidateUpdate() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateUpdate() unexpected error: %v", err)
			}
		})
	}
}

func TestValidateUpdatesContextPreservesIndexAndCancellation(t *testing.T) {
	t.Parallel()

	valid := buildUpdate(
		clientBlock{
			client: 10,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "valid"),
			},
		},
	)
	malformedV2 := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	validV2 := mustDecodeHex(t, "000002a50100000104060374686901020101000001010000")

	if err := ValidateUpdates(valid, validV2); err != nil {
		t.Fatalf("ValidateUpdates(v1, v2) unexpected error: %v", err)
	}

	err := ValidateUpdates(valid, malformedV2)
	if err == nil {
		t.Fatal("ValidateUpdates() error = nil, want malformed update")
	}
	if !strings.Contains(err.Error(), "update[1]") {
		t.Fatalf("ValidateUpdates() error = %v, want update index 1", err)
	}
	if !errors.Is(err, varint.ErrUnexpectedEOF) {
		t.Fatalf("ValidateUpdates() error = %v, want %v", err, varint.ErrUnexpectedEOF)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := ValidateUpdatesContext(ctx, valid); !errors.Is(err, context.Canceled) {
		t.Fatalf("ValidateUpdatesContext(cancelled) error = %v, want %v", err, context.Canceled)
	}
}

func TestUpdateDispatchErrors(t *testing.T) {
	t.Parallel()

	if _, err := DecodeUpdate(nil); !errors.Is(err, ErrUnknownUpdateFormat) {
		t.Fatalf("DecodeUpdate(nil) error = %v, want %v", err, ErrUnknownUpdateFormat)
	}

	if _, err := DecodeUpdate([]byte{0x80}); !errors.Is(err, varint.ErrUnexpectedEOF) {
		t.Fatalf("DecodeUpdate(v1 header broken) error = %v, want %v", err, varint.ErrUnexpectedEOF)
	}

	if _, err := DecodeUpdate(append(buildUpdate(), 0x00)); !errors.Is(err, ErrTrailingBytes) {
		t.Fatalf("DecodeUpdate(v1 with trailing) error = %v, want %v", err, ErrTrailingBytes)
	}

	if _, err := DiffUpdate([]byte{0x00, 0x01}, nil); !errors.Is(err, varint.ErrUnexpectedEOF) {
		t.Fatalf("DiffUpdate(v2-ambiguous candidate) error = %v, want %v", err, varint.ErrUnexpectedEOF)
	}
}

func TestSingleUpdateFormatAwareAPIsDispatchV1(t *testing.T) {
	t.Parallel()

	update := buildUpdate(
		clientBlock{
			client: 7,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "hi"),
				itemAny(rootParent("doc"), appendAnyString(nil, "x")),
			},
		},
		clientBlock{
			client: 9,
			clock:  0,
			structs: []structEncoding{
				itemBinary(rootParent("doc"), []byte{0x01, 0x02}),
			},
		},
		deleteRange{
			client: 7,
			clock:  4,
			length: 1,
		},
	)

	gotStateVector, err := StateVectorFromUpdate(update)
	if err != nil {
		t.Fatalf("StateVectorFromUpdate() unexpected error: %v", err)
	}
	if len(gotStateVector) != 2 || gotStateVector[7] != 3 || gotStateVector[9] != 1 {
		t.Fatalf("StateVectorFromUpdate() = %#v, want map[7:3 9:1]", gotStateVector)
	}

	gotEncodedStateVector, err := EncodeStateVectorFromUpdate(update)
	if err != nil {
		t.Fatalf("EncodeStateVectorFromUpdate() unexpected error: %v", err)
	}
	wantEncodedStateVector, err := EncodeStateVectorFromUpdateV1(update)
	if err != nil {
		t.Fatalf("EncodeStateVectorFromUpdateV1() unexpected error: %v", err)
	}
	if !bytes.Equal(gotEncodedStateVector, wantEncodedStateVector) {
		t.Fatalf("EncodeStateVectorFromUpdate() = %v, want %v", gotEncodedStateVector, wantEncodedStateVector)
	}

	gotContentIDs, err := CreateContentIDsFromUpdate(update)
	if err != nil {
		t.Fatalf("CreateContentIDsFromUpdate() unexpected error: %v", err)
	}
	wantContentIDs, err := CreateContentIDsFromUpdateV1(update)
	if err != nil {
		t.Fatalf("CreateContentIDsFromUpdateV1() unexpected error: %v", err)
	}
	if !IsSubsetContentIDs(gotContentIDs, wantContentIDs) || !IsSubsetContentIDs(wantContentIDs, gotContentIDs) {
		t.Fatalf("CreateContentIDsFromUpdate() = %#v, want %#v", gotContentIDs, wantContentIDs)
	}

	filter := NewContentIDs()
	_ = filter.Inserts.Add(7, 2, 1)
	_ = filter.Deletes.Add(7, 4, 1)

	gotIntersection, err := IntersectUpdateWithContentIDs(update, filter)
	if err != nil {
		t.Fatalf("IntersectUpdateWithContentIDs() unexpected error: %v", err)
	}
	wantIntersection, err := IntersectUpdateWithContentIDsV1(update, filter)
	if err != nil {
		t.Fatalf("IntersectUpdateWithContentIDsV1() unexpected error: %v", err)
	}
	if !bytes.Equal(gotIntersection, wantIntersection) {
		t.Fatalf("IntersectUpdateWithContentIDs() = %v, want %v", gotIntersection, wantIntersection)
	}
}

func TestSingleUpdateFormatAwareAPIsRejectUnknownFormats(t *testing.T) {
	t.Parallel()

	filter := NewContentIDs()

	tests := []struct {
		name    string
		call    func([]byte) error
		input   []byte
		wantErr error
	}{
		{
			name: "StateVectorFromUpdate_empty",
			call: func(update []byte) error {
				_, err := StateVectorFromUpdate(update)
				return err
			},
			input:   nil,
			wantErr: ErrUnknownUpdateFormat,
		},
		{
			name: "EncodeStateVectorFromUpdate_empty",
			call: func(update []byte) error {
				_, err := EncodeStateVectorFromUpdate(update)
				return err
			},
			input:   nil,
			wantErr: ErrUnknownUpdateFormat,
		},
		{
			name: "CreateContentIDsFromUpdate_empty",
			call: func(update []byte) error {
				_, err := CreateContentIDsFromUpdate(update)
				return err
			},
			input:   nil,
			wantErr: ErrUnknownUpdateFormat,
		},
		{
			name: "IntersectUpdateWithContentIDs_empty",
			call: func(update []byte) error {
				_, err := IntersectUpdateWithContentIDs(update, filter)
				return err
			},
			input:   nil,
			wantErr: ErrUnknownUpdateFormat,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.call(tt.input)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("%s error = %v, want %v", tt.name, err, tt.wantErr)
			}
		})
	}
}

func TestSingleUpdateFormatAwareAPIsDeriveSupportedV2ThroughV1Conversion(t *testing.T) {
	t.Parallel()

	v2Update := mustDecodeHex(t, "000003e5010102000400050400810084080474686c6f41000201010001020103000165010101")
	v1Update, err := ConvertUpdateToV1(v2Update)
	if err != nil {
		t.Fatalf("ConvertUpdateToV1() unexpected error: %v", err)
	}

	gotStateVector, err := StateVectorFromUpdate(v2Update)
	if err != nil {
		t.Fatalf("StateVectorFromUpdate(v2) unexpected error: %v", err)
	}
	wantStateVector, err := StateVectorFromUpdate(v1Update)
	if err != nil {
		t.Fatalf("StateVectorFromUpdate(v1) unexpected error: %v", err)
	}
	if !equalStateVectors(gotStateVector, wantStateVector) {
		t.Fatalf("StateVectorFromUpdate(v2) = %#v, want %#v", gotStateVector, wantStateVector)
	}

	gotEncodedStateVector, err := EncodeStateVectorFromUpdate(v2Update)
	if err != nil {
		t.Fatalf("EncodeStateVectorFromUpdate(v2) unexpected error: %v", err)
	}
	wantEncodedStateVector, err := EncodeStateVectorFromUpdate(v1Update)
	if err != nil {
		t.Fatalf("EncodeStateVectorFromUpdate(v1) unexpected error: %v", err)
	}
	if !bytes.Equal(gotEncodedStateVector, wantEncodedStateVector) {
		t.Fatalf("EncodeStateVectorFromUpdate(v2) = %x, want %x", gotEncodedStateVector, wantEncodedStateVector)
	}

	gotContentIDs, err := CreateContentIDsFromUpdate(v2Update)
	if err != nil {
		t.Fatalf("CreateContentIDsFromUpdate(v2) unexpected error: %v", err)
	}
	wantContentIDs, err := CreateContentIDsFromUpdate(v1Update)
	if err != nil {
		t.Fatalf("CreateContentIDsFromUpdate(v1) unexpected error: %v", err)
	}
	if !IsSubsetContentIDs(gotContentIDs, wantContentIDs) || !IsSubsetContentIDs(wantContentIDs, gotContentIDs) {
		t.Fatalf("CreateContentIDsFromUpdate(v2) = %#v, want %#v", gotContentIDs, wantContentIDs)
	}

	stateVector := encodeStateVectorEntry(101, 2)
	gotDiff, err := DiffUpdate(v2Update, stateVector)
	if err != nil {
		t.Fatalf("DiffUpdate(v2) unexpected error: %v", err)
	}
	wantDiff, err := DiffUpdate(v1Update, stateVector)
	if err != nil {
		t.Fatalf("DiffUpdate(v1) unexpected error: %v", err)
	}
	if !bytes.Equal(gotDiff, wantDiff) {
		t.Fatalf("DiffUpdate(v2) = %x, want %x", gotDiff, wantDiff)
	}

	gotIntersection, err := IntersectUpdateWithContentIDs(v2Update, wantContentIDs)
	if err != nil {
		t.Fatalf("IntersectUpdateWithContentIDs(v2) unexpected error: %v", err)
	}
	wantIntersection, err := IntersectUpdateWithContentIDs(v1Update, wantContentIDs)
	if err != nil {
		t.Fatalf("IntersectUpdateWithContentIDs(v1) unexpected error: %v", err)
	}
	if !bytes.Equal(gotIntersection, wantIntersection) {
		t.Fatalf("IntersectUpdateWithContentIDs(v2) = %x, want %x", gotIntersection, wantIntersection)
	}
}

func equalStateVectors(a, b map[uint32]uint32) bool {
	if len(a) != len(b) {
		return false
	}
	for client, clock := range a {
		if b[client] != clock {
			return false
		}
	}
	return true
}

func TestMergeUpdatesContextRejectsUnknownFormats(t *testing.T) {
	t.Parallel()

	v1 := buildUpdate(
		clientBlock{
			client: 1,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "x"),
			},
		},
	)

	tests := []struct {
		name    string
		updates [][]byte
		wantErr error
	}{
		{
			name:    "mixed",
			updates: [][]byte{v1, mustDecodeHex(t, "000002a50100000104060374686901020101000001010000")},
			wantErr: ErrMismatchedUpdateFormats,
		},
		{
			name:    "empty_payload",
			updates: [][]byte{v1, nil},
			wantErr: ErrUnknownUpdateFormat,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := MergeUpdatesContext(context.Background(), tt.updates...)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("MergeUpdatesContext() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestDiffAndIntersectContextDispatchV1(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	update := buildUpdate(
		clientBlock{
			client: 8,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "abcd"),
			},
		},
		deleteRange{
			client: 8,
			clock:  3,
			length: 1,
		},
	)
	stateVector := encodeStateVectorEntry(8, 2)
	filter := NewContentIDs()
	_ = filter.Inserts.Add(8, 1, 2)
	_ = filter.Deletes.Add(8, 3, 1)

	gotDiff, err := DiffUpdateContext(ctx, update, stateVector)
	if err != nil {
		t.Fatalf("DiffUpdateContext() unexpected error: %v", err)
	}
	wantDiff, err := DiffUpdateV1Context(ctx, update, stateVector)
	if err != nil {
		t.Fatalf("DiffUpdateV1Context() unexpected error: %v", err)
	}
	if !bytes.Equal(gotDiff, wantDiff) {
		t.Fatalf("DiffUpdateContext() = %v, want %v", gotDiff, wantDiff)
	}

	gotIntersect, err := IntersectUpdateWithContentIDsContext(ctx, update, filter)
	if err != nil {
		t.Fatalf("IntersectUpdateWithContentIDsContext() unexpected error: %v", err)
	}
	wantIntersect, err := IntersectUpdateWithContentIDsV1Context(ctx, update, filter)
	if err != nil {
		t.Fatalf("IntersectUpdateWithContentIDsV1Context() unexpected error: %v", err)
	}
	if !bytes.Equal(gotIntersect, wantIntersect) {
		t.Fatalf("IntersectUpdateWithContentIDsContext() = %v, want %v", gotIntersect, wantIntersect)
	}
}

func TestDiffAndIntersectContextRejectUnknownFormats(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	filter := NewContentIDs()

	tests := []struct {
		name    string
		call    func([]byte) error
		input   []byte
		wantErr error
	}{
		{
			name: "DiffUpdateContext_empty",
			call: func(update []byte) error {
				_, err := DiffUpdateContext(ctx, update, nil)
				return err
			},
			input:   nil,
			wantErr: ErrUnknownUpdateFormat,
		},
		{
			name: "IntersectUpdateWithContentIDsContext_empty",
			call: func(update []byte) error {
				_, err := IntersectUpdateWithContentIDsContext(ctx, update, filter)
				return err
			},
			input:   nil,
			wantErr: ErrUnknownUpdateFormat,
		},
		{
			name: "DiffUpdateContext_malformed",
			call: func(update []byte) error {
				_, err := DiffUpdateContext(ctx, update, nil)
				return err
			},
			input:   []byte{0x80},
			wantErr: varint.ErrUnexpectedEOF,
		},
		{
			name: "IntersectUpdateWithContentIDsContext_malformed",
			call: func(update []byte) error {
				_, err := IntersectUpdateWithContentIDsContext(ctx, update, filter)
				return err
			},
			input:   []byte{0x80},
			wantErr: varint.ErrUnexpectedEOF,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.call(tt.input)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("%s error = %v, want %v", tt.name, err, tt.wantErr)
			}
		})
	}
}
