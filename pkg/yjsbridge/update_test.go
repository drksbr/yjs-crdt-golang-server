package yjsbridge

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"yjs-go-bridge/internal/yupdate"
)

func TestPublicUpdateAPIStableV1Paths(t *testing.T) {
	t.Parallel()

	emptyUpdate := NewPersistedSnapshot().UpdateV1

	format, err := FormatFromUpdate(emptyUpdate)
	if err != nil {
		t.Fatalf("FormatFromUpdate() unexpected error: %v", err)
	}
	if format != UpdateFormatV1 {
		t.Fatalf("FormatFromUpdate() = %s, want %s", format, UpdateFormatV1)
	}

	aggregatedFormat, err := FormatFromUpdates(emptyUpdate, emptyUpdate)
	if err != nil {
		t.Fatalf("FormatFromUpdates() unexpected error: %v", err)
	}
	if aggregatedFormat != UpdateFormatV1 {
		t.Fatalf("FormatFromUpdates() = %s, want %s", aggregatedFormat, UpdateFormatV1)
	}

	converted, err := ConvertUpdateToV1(emptyUpdate)
	if err != nil {
		t.Fatalf("ConvertUpdateToV1() unexpected error: %v", err)
	}
	if !bytes.Equal(converted, emptyUpdate) {
		t.Fatalf("ConvertUpdateToV1() = %v, want %v", converted, emptyUpdate)
	}

	merged, err := MergeUpdates(emptyUpdate, emptyUpdate)
	if err != nil {
		t.Fatalf("MergeUpdates() unexpected error: %v", err)
	}
	if !bytes.Equal(merged, emptyUpdate) {
		t.Fatalf("MergeUpdates() = %v, want %v", merged, emptyUpdate)
	}

	noopMerged, err := MergeUpdates(nil, []byte{})
	if err != nil {
		t.Fatalf("MergeUpdates(all-empty) unexpected error: %v", err)
	}
	if !bytes.Equal(noopMerged, emptyUpdate) {
		t.Fatalf("MergeUpdates(all-empty) = %v, want %v", noopMerged, emptyUpdate)
	}

	stateVector, err := StateVectorFromUpdate(emptyUpdate)
	if err != nil {
		t.Fatalf("StateVectorFromUpdate() unexpected error: %v", err)
	}
	if len(stateVector) != 0 {
		t.Fatalf("StateVectorFromUpdate() = %v, want empty", stateVector)
	}

	encodedStateVector, err := EncodeStateVectorFromUpdates(nil, emptyUpdate, []byte{})
	if err != nil {
		t.Fatalf("EncodeStateVectorFromUpdates() unexpected error: %v", err)
	}
	decodedStateVector, err := DecodeStateVector(encodedStateVector)
	if err != nil {
		t.Fatalf("DecodeStateVector() unexpected error: %v", err)
	}
	if len(decodedStateVector) != 0 {
		t.Fatalf("DecodeStateVector() = %v, want empty", decodedStateVector)
	}

	snapshot, err := SnapshotFromUpdates(nil, emptyUpdate, []byte{})
	if err != nil {
		t.Fatalf("SnapshotFromUpdates() unexpected error: %v", err)
	}
	if !snapshot.IsEmpty() {
		t.Fatalf("SnapshotFromUpdates() should be empty, got %#v", snapshot)
	}

	diff, err := DiffUpdate(emptyUpdate, encodedStateVector)
	if err != nil {
		t.Fatalf("DiffUpdate() unexpected error: %v", err)
	}
	if !bytes.Equal(diff, emptyUpdate) {
		t.Fatalf("DiffUpdate() = %v, want %v", diff, emptyUpdate)
	}

	contentIDs, err := CreateContentIDsFromUpdate(emptyUpdate)
	if err != nil {
		t.Fatalf("CreateContentIDsFromUpdate() unexpected error: %v", err)
	}
	if !contentIDs.IsEmpty() {
		t.Fatalf("CreateContentIDsFromUpdate() should be empty, got %#v", contentIDs)
	}

	intersected, err := IntersectUpdateWithContentIDs(emptyUpdate, contentIDs)
	if err != nil {
		t.Fatalf("IntersectUpdateWithContentIDs() unexpected error: %v", err)
	}
	if !bytes.Equal(intersected, emptyUpdate) {
		t.Fatalf("IntersectUpdateWithContentIDs() = %v, want %v", intersected, emptyUpdate)
	}
}

func TestPublicUpdateAPIRejectsUnsupportedAndRespectsContext(t *testing.T) {
	t.Parallel()

	emptyUpdate := NewPersistedSnapshot().UpdateV1
	v2Update := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	tests := []struct {
		name string
		run  func(context.Context) error
		want error
	}{
		{
			name: "merge_v2",
			run: func(context.Context) error {
				_, err := MergeUpdates(v2Update)
				return err
			},
			want: ErrUnsupportedUpdateFormatV2,
		},
		{
			name: "state_vector_v2",
			run: func(context.Context) error {
				_, err := StateVectorFromUpdate(v2Update)
				return err
			},
			want: ErrUnsupportedUpdateFormatV2,
		},
		{
			name: "content_ids_v2",
			run: func(context.Context) error {
				_, err := CreateContentIDsFromUpdate(v2Update)
				return err
			},
			want: ErrUnsupportedUpdateFormatV2,
		},
		{
			name: "snapshot_v2",
			run: func(context.Context) error {
				_, err := SnapshotFromUpdate(v2Update)
				return err
			},
			want: ErrUnsupportedUpdateFormatV2,
		},
		{
			name: "merge_context_cancelled",
			run: func(ctx context.Context) error {
				_, err := MergeUpdatesContext(ctx, emptyUpdate)
				return err
			},
			want: context.Canceled,
		},
		{
			name: "state_vector_context_cancelled",
			run: func(ctx context.Context) error {
				_, err := StateVectorFromUpdatesContext(ctx, emptyUpdate)
				return err
			},
			want: context.Canceled,
		},
		{
			name: "content_ids_context_cancelled",
			run: func(ctx context.Context) error {
				_, err := ContentIDsFromUpdatesContext(ctx, emptyUpdate)
				return err
			},
			want: context.Canceled,
		},
		{
			name: "intersect_context_cancelled",
			run: func(ctx context.Context) error {
				_, err := IntersectUpdateWithContentIDsContext(ctx, emptyUpdate, NewContentIDs())
				return err
			},
			want: context.Canceled,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			if errors.Is(tt.want, context.Canceled) {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(context.Background())
				cancel()
			}

			if err := tt.run(ctx); !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestPublicUpdateAPIMatchesInternalContracts(t *testing.T) {
	t.Parallel()

	emptyUpdate := NewPersistedSnapshot().UpdateV1

	gotSV, err := StateVectorFromUpdates(emptyUpdate)
	if err != nil {
		t.Fatalf("StateVectorFromUpdates() unexpected error: %v", err)
	}
	wantSV, err := yupdate.StateVectorFromUpdates(emptyUpdate)
	if err != nil {
		t.Fatalf("internal StateVectorFromUpdates() unexpected error: %v", err)
	}
	if len(gotSV) != len(wantSV) {
		t.Fatalf("StateVectorFromUpdates() len = %d, want %d", len(gotSV), len(wantSV))
	}

	gotIDs, err := ContentIDsFromUpdates(emptyUpdate)
	if err != nil {
		t.Fatalf("ContentIDsFromUpdates() unexpected error: %v", err)
	}
	wantIDs, err := yupdate.ContentIDsFromUpdates(emptyUpdate)
	if err != nil {
		t.Fatalf("internal ContentIDsFromUpdates() unexpected error: %v", err)
	}
	gotPayload, err := EncodeContentIDs(gotIDs)
	if err != nil {
		t.Fatalf("EncodeContentIDs() unexpected error: %v", err)
	}
	wantPayload, err := yupdate.EncodeContentIDs(wantIDs)
	if err != nil {
		t.Fatalf("internal EncodeContentIDs() unexpected error: %v", err)
	}
	if !bytes.Equal(gotPayload, wantPayload) {
		t.Fatalf("ContentIDsFromUpdates() payload = %v, want %v", gotPayload, wantPayload)
	}
}
