package yupdate

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestPersistedSnapshotV1RestoreMatchesDirectConstructor(t *testing.T) {
	t.Parallel()

	validUpdate := buildUpdate(
		clientBlock{
			client: 18,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "persisted"),
			},
		},
	)

	tests := []struct {
		name  string
		input []byte
	}{
		{
			name:  "v1_payload",
			input: validUpdate,
		},
		{
			name:  "empty_v1_payload",
			input: encodeEmptyUpdateV1(),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			want, err := PersistedSnapshotFromUpdate(tt.input)
			if err != nil {
				t.Fatalf("PersistedSnapshotFromUpdate() unexpected error: %v", err)
			}

			got, err := DecodePersistedSnapshotV1(tt.input)
			if err != nil {
				t.Fatalf("DecodePersistedSnapshotV1() unexpected error: %v", err)
			}

			if !bytes.Equal(got.UpdateV1, want.UpdateV1) {
				t.Fatalf("DecodePersistedSnapshotV1().UpdateV1 = %v, want %v", got.UpdateV1, want.UpdateV1)
			}

			assertPersistedSnapshotMatchesSnapshot(t, got, want.Snapshot)
		})
	}
}

func TestPersistedSnapshotV1RestoreFromEmptyPayload(t *testing.T) {
	t.Parallel()

	want := NewPersistedSnapshot()

	tests := []struct {
		name    string
		payload []byte
	}{
		{
			name:    "nil_payload",
			payload: nil,
		},
		{
			name:    "empty_payload",
			payload: []byte{},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := DecodePersistedSnapshotV1(tt.payload)
			if err != nil {
				t.Fatalf("DecodePersistedSnapshotV1() unexpected error: %v", err)
			}

			if !bytes.Equal(got.UpdateV1, want.UpdateV1) {
				t.Fatalf("got.UpdateV1 = %v, want %v", got.UpdateV1, want.UpdateV1)
			}
			if got.Snapshot == nil {
				t.Fatalf("got.Snapshot = nil, want non-nil")
			}
			assertPersistedSnapshotMatchesSnapshot(t, got, want.Snapshot)
		})
	}
}

func TestPersistedSnapshotNilCloneContract(t *testing.T) {
	t.Parallel()

	got := (*PersistedSnapshot)(nil).Clone()
	if got == nil {
		t.Fatalf("(*PersistedSnapshot)(nil).Clone() = nil, want non-nil snapshot")
	}
	if !got.IsEmpty() {
		t.Fatalf("(*PersistedSnapshot)(nil).Clone().IsEmpty() = false, want true")
	}
	if !bytes.Equal(got.UpdateV1, encodeEmptyUpdateV1()) {
		t.Fatalf("(*PersistedSnapshot)(nil).Clone().UpdateV1 = %v, want %v", got.UpdateV1, encodeEmptyUpdateV1())
	}
}

func TestPersistedSnapshotV1RestoreContextCanceled(t *testing.T) {
	t.Parallel()

	update := buildUpdate(
		clientBlock{
			client: 19,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "context"),
			},
		},
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := DecodePersistedSnapshotV1Context(ctx, update)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("DecodePersistedSnapshotV1Context() error = %v, want %v", err, context.Canceled)
	}
}

func TestPersistedSnapshotV1RestoreV2UnsupportedErrorContract(t *testing.T) {
	t.Parallel()

	v2 := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	t.Run("single_payload_no_format_detection_keeps_error_unindexed", func(t *testing.T) {
		t.Parallel()

		_, err := DecodePersistedSnapshotV1(v2)
		if err == nil {
			t.Fatalf("DecodePersistedSnapshotV1() error = nil, want %v", ErrUnsupportedUpdateFormatV2)
		}
		if !errors.Is(err, ErrUnsupportedUpdateFormatV2) {
			t.Fatalf("DecodePersistedSnapshotV1() error = %v, want %v", err, ErrUnsupportedUpdateFormatV2)
		}
		if strings.Contains(err.Error(), "update[") {
			t.Fatalf("DecodePersistedSnapshotV1() error = %v, want no update index context", err)
		}
	})

	t.Run("format_detection_variant_returns_indexed_unsupported_error", func(t *testing.T) {
		t.Parallel()

		_, err := PersistedSnapshotFromUpdatesContext(context.Background(), nil, []byte{}, v2)
		if err == nil {
			t.Fatalf("PersistedSnapshotFromUpdatesContext() error = nil, want %v", ErrUnsupportedUpdateFormatV2)
		}
		if !errors.Is(err, ErrUnsupportedUpdateFormatV2) {
			t.Fatalf("PersistedSnapshotFromUpdatesContext() error = %v, want %v", err, ErrUnsupportedUpdateFormatV2)
		}
		if !strings.Contains(err.Error(), "update[2]") {
			t.Fatalf("PersistedSnapshotFromUpdatesContext() error = %v, want contains %q", err, "update[2]")
		}
	})
}
