package yjsbridge

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

func TestPublicSnapshotAPIStableV1Paths(t *testing.T) {
	t.Parallel()

	emptyStored, err := EncodePersistedSnapshotV1(nil)
	if err != nil {
		t.Fatalf("EncodePersistedSnapshotV1(nil) unexpected error: %v", err)
	}

	tests := []struct {
		name     string
		snapshot *PersistedSnapshot
	}{
		{
			name:     "nil_snapshot",
			snapshot: nil,
		},
		{
			name:     "new_snapshot",
			snapshot: NewPersistedSnapshot(),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			encoded, err := EncodePersistedSnapshotV1(tt.snapshot)
			if err != nil {
				t.Fatalf("EncodePersistedSnapshotV1() unexpected error: %v", err)
			}
			if !bytes.Equal(encoded, emptyStored) {
				t.Fatalf("EncodePersistedSnapshotV1() = %v, want %v", encoded, emptyStored)
			}

			got, err := DecodePersistedSnapshotV1(encoded)
			if err != nil {
				t.Fatalf("DecodePersistedSnapshotV1() unexpected error: %v", err)
			}
			if got == nil {
				t.Fatal("DecodePersistedSnapshotV1() returned nil, want non-nil snapshot")
			}
			if !got.IsEmpty() {
				t.Fatalf("snapshot should be empty after restore, got: %#v", got)
			}
			if !bytes.Equal(got.UpdateV1, emptyStored) {
				t.Fatalf("got.UpdateV1 = %v, want %v", got.UpdateV1, emptyStored)
			}
		})
	}

	t.Run("restore_round_trip_from_update", func(t *testing.T) {
		t.Parallel()

		got, err := PersistedSnapshotFromUpdate(emptyStored)
		if err != nil {
			t.Fatalf("PersistedSnapshotFromUpdate() unexpected error: %v", err)
		}
		restored, err := DecodePersistedSnapshotV1(got.UpdateV1)
		if err != nil {
			t.Fatalf("DecodePersistedSnapshotV1() unexpected error: %v", err)
		}
		if !bytes.Equal(restored.UpdateV1, emptyStored) {
			t.Fatalf("restored.UpdateV1 = %v, want %v", restored.UpdateV1, emptyStored)
		}
	})
}

func TestPublicSnapshotAPIV1UnsupportedAndContextAware(t *testing.T) {
	t.Parallel()

	samplePayload, err := EncodePersistedSnapshotV1(nil)
	if err != nil {
		t.Fatalf("EncodePersistedSnapshotV1(nil) unexpected error: %v", err)
	}

	t.Run("rejects_v2_payload", func(t *testing.T) {
		t.Parallel()

		_, err := DecodePersistedSnapshotV1([]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
		if !errors.Is(err, ErrUnsupportedUpdateFormatV2) {
			t.Fatalf("DecodePersistedSnapshotV1() error = %v, want %v", err, ErrUnsupportedUpdateFormatV2)
		}
	})

	t.Run("from_updates_context_cancelled", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := PersistedSnapshotFromUpdatesContext(ctx, samplePayload)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("PersistedSnapshotFromUpdatesContext() error = %v, want %v", err, context.Canceled)
		}
	})

	t.Run("decode_context_cancelled", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := DecodePersistedSnapshotV1Context(ctx, samplePayload)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("DecodePersistedSnapshotV1Context() error = %v, want %v", err, context.Canceled)
		}
	})
}

func TestPublicSnapshotAPINilContextFallsBackToBackground(t *testing.T) {
	t.Parallel()

	samplePayload, err := EncodePersistedSnapshotV1(nil)
	if err != nil {
		t.Fatalf("EncodePersistedSnapshotV1(nil) unexpected error: %v", err)
	}

	t.Run("from_updates_context_nil", func(t *testing.T) {
		t.Parallel()

		got, err := PersistedSnapshotFromUpdatesContext(nil, samplePayload)
		if err != nil {
			t.Fatalf("PersistedSnapshotFromUpdatesContext(nil) unexpected error: %v", err)
		}
		if got == nil {
			t.Fatal("PersistedSnapshotFromUpdatesContext(nil) returned nil, want non-nil")
		}
		if !bytes.Equal(got.UpdateV1, samplePayload) {
			t.Fatalf("PersistedSnapshotFromUpdatesContext(nil).UpdateV1 = %v, want %v", got.UpdateV1, samplePayload)
		}
	})

	t.Run("decode_context_nil", func(t *testing.T) {
		t.Parallel()

		got, err := DecodePersistedSnapshotV1Context(nil, samplePayload)
		if err != nil {
			t.Fatalf("DecodePersistedSnapshotV1Context(nil) unexpected error: %v", err)
		}
		if got == nil {
			t.Fatal("DecodePersistedSnapshotV1Context(nil) returned nil, want non-nil")
		}
		if !bytes.Equal(got.UpdateV1, samplePayload) {
			t.Fatalf("DecodePersistedSnapshotV1Context(nil).UpdateV1 = %v, want %v", got.UpdateV1, samplePayload)
		}
	})
}

func TestPublicSnapshotAPISupportsNilAndEmptyPayloadAndConstructors(t *testing.T) {
	t.Parallel()

	empty := NewPersistedSnapshot()
	canonical, err := EncodePersistedSnapshotV1(empty)
	if err != nil {
		t.Fatalf("EncodePersistedSnapshotV1() unexpected error: %v", err)
	}

	tests := []struct {
		name    string
		payload []byte
	}{
		{name: "nil_payload", payload: nil},
		{name: "empty_payload", payload: []byte{}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := DecodePersistedSnapshotV1(tt.payload)
			if err != nil {
				t.Fatalf("DecodePersistedSnapshotV1() unexpected error: %v", err)
			}
			if !bytes.Equal(got.UpdateV1, canonical) {
				t.Fatalf("DecodePersistedSnapshotV1().UpdateV1 = %v, want %v", got.UpdateV1, canonical)
			}
			if !got.IsEmpty() {
				t.Fatalf("snapshot should be empty, got %#v", got)
			}
		})
	}

	constructed := NewSnapshot()
	if constructed == nil {
		t.Fatal("NewSnapshot() = nil, want non-nil")
	}
	if !constructed.IsEmpty() {
		t.Fatalf("NewSnapshot() should be empty, got %#v", constructed)
	}

	persisted := NewPersistedSnapshot()
	if persisted == nil {
		t.Fatal("NewPersistedSnapshot() = nil, want non-nil")
	}
	if !persisted.IsEmpty() {
		t.Fatalf("NewPersistedSnapshot() should be empty, got %#v", persisted)
	}
}
