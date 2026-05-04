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

		var nilCtx context.Context
		got, err := PersistedSnapshotFromUpdatesContext(nilCtx, samplePayload)
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

		var nilCtx context.Context
		got, err := DecodePersistedSnapshotV1Context(nilCtx, samplePayload)
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

func TestPublicSnapshotAPIV2OptInCodec(t *testing.T) {
	t.Parallel()

	v2 := mustDecodeHexPayload(t, "000002a50100000104060374686901020101000001010000")
	v1 := mustDecodeHexPayload(t, "010165000401017402686900")

	restored, err := DecodePersistedSnapshotV2(v2)
	if err != nil {
		t.Fatalf("DecodePersistedSnapshotV2() unexpected error: %v", err)
	}
	if !bytes.Equal(restored.UpdateV1, v1) {
		t.Fatalf("DecodePersistedSnapshotV2().UpdateV1 = %x, want %x", restored.UpdateV1, v1)
	}
	if !bytes.Equal(restored.UpdateV2, v2) {
		t.Fatalf("DecodePersistedSnapshotV2().UpdateV2 = %x, want canonical V2 %x", restored.UpdateV2, v2)
	}

	encoded, err := EncodePersistedSnapshotV2(restored)
	if err != nil {
		t.Fatalf("EncodePersistedSnapshotV2() unexpected error: %v", err)
	}
	format, err := FormatFromUpdate(encoded)
	if err != nil {
		t.Fatalf("FormatFromUpdate(encoded) unexpected error: %v", err)
	}
	if format != UpdateFormatV2 {
		t.Fatalf("FormatFromUpdate(encoded) = %s, want %s", format, UpdateFormatV2)
	}
	converted, err := ConvertUpdateToV1(encoded)
	if err != nil {
		t.Fatalf("ConvertUpdateToV1(encoded) unexpected error: %v", err)
	}
	if !bytes.Equal(converted, v1) {
		t.Fatalf("ConvertUpdateToV1(encoded) = %x, want %x", converted, v1)
	}

	if _, err := DecodePersistedSnapshotV1(encoded); !errors.Is(err, ErrUnsupportedUpdateFormatV2) {
		t.Fatalf("DecodePersistedSnapshotV1(v2) error = %v, want %v", err, ErrUnsupportedUpdateFormatV2)
	}
}

func TestPublicSnapshotAPIV2ContextAndEmptyPayload(t *testing.T) {
	t.Parallel()

	empty, err := DecodePersistedSnapshotV2(nil)
	if err != nil {
		t.Fatalf("DecodePersistedSnapshotV2(nil) unexpected error: %v", err)
	}
	if !empty.IsEmpty() {
		t.Fatalf("DecodePersistedSnapshotV2(nil).IsEmpty() = false, want true")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = DecodePersistedSnapshotV2Context(ctx, mustDecodeHexPayload(t, "000002a50100000104060374686901020101000001010000"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("DecodePersistedSnapshotV2Context(cancelled) error = %v, want %v", err, context.Canceled)
	}
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
	if format, err := FormatFromUpdate(persisted.UpdateV2); err != nil || format != UpdateFormatV2 {
		t.Fatalf("NewPersistedSnapshot().UpdateV2 format = %s, %v; want %s", format, err, UpdateFormatV2)
	}
}
