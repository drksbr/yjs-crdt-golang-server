package yprotocol

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/yjsbridge"
)

func TestPublicV2SyncOutputHelpersEncodeV2Payloads(t *testing.T) {
	t.Parallel()

	left := buildGCOnlyUpdate(11, 2)
	right := buildGCOnlyUpdate(12, 1)
	mergedV1, err := yjsbridge.MergeUpdates(left, right)
	if err != nil {
		t.Fatalf("MergeUpdates() unexpected error: %v", err)
	}

	cases := []struct {
		name     string
		encode   func() ([]byte, error)
		wantType SyncMessageType
		wantV1   []byte
	}{
		{
			name: "sync step2 from V1 updates emits V2",
			encode: func() ([]byte, error) {
				return EncodeSyncStep2FromUpdatesV2(left, right)
			},
			wantType: SyncMessageTypeStep2,
			wantV1:   mergedV1,
		},
		{
			name: "sync update from V1 update emits V2",
			encode: func() ([]byte, error) {
				return EncodeSyncUpdateV2(left)
			},
			wantType: SyncMessageTypeUpdate,
			wantV1:   left,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			encoded, err := tc.encode()
			if err != nil {
				t.Fatalf("%s unexpected error: %v", tc.name, err)
			}
			decoded, err := DecodeSyncMessage(encoded)
			if err != nil {
				t.Fatalf("DecodeSyncMessage() unexpected error: %v", err)
			}
			if decoded.Type != tc.wantType {
				t.Fatalf("decoded.Type = %v, want %v", decoded.Type, tc.wantType)
			}
			assertV2PayloadEquivalentToV1(t, decoded.Payload, tc.wantV1)
		})
	}
}

func TestPublicV2ProtocolSyncOutputHelpersEncodeDecodableEnvelopes(t *testing.T) {
	t.Parallel()

	left := buildGCOnlyUpdate(21, 2)
	right := buildGCOnlyUpdate(22, 1)
	mergedV1, err := yjsbridge.MergeUpdates(left, right)
	if err != nil {
		t.Fatalf("MergeUpdates() unexpected error: %v", err)
	}

	cases := []struct {
		name     string
		encode   func() ([]byte, error)
		wantType SyncMessageType
		wantV1   []byte
	}{
		{
			name: "protocol sync step2 from V1 updates emits V2",
			encode: func() ([]byte, error) {
				return EncodeProtocolSyncStep2FromUpdatesV2(left, right)
			},
			wantType: SyncMessageTypeStep2,
			wantV1:   mergedV1,
		},
		{
			name: "protocol sync update from V1 update emits V2",
			encode: func() ([]byte, error) {
				return EncodeProtocolSyncUpdateV2(left)
			},
			wantType: SyncMessageTypeUpdate,
			wantV1:   left,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			encoded, err := tc.encode()
			if err != nil {
				t.Fatalf("%s unexpected error: %v", tc.name, err)
			}
			envelope, err := DecodeProtocolMessage(encoded)
			if err != nil {
				t.Fatalf("DecodeProtocolMessage() unexpected error: %v", err)
			}
			if envelope.Protocol != ProtocolTypeSync || envelope.Sync == nil {
				t.Fatalf("DecodeProtocolMessage() = %#v, want sync envelope", envelope)
			}
			if envelope.Sync.Type != tc.wantType {
				t.Fatalf("envelope.Sync.Type = %v, want %v", envelope.Sync.Type, tc.wantType)
			}
			assertV2PayloadEquivalentToV1(t, envelope.Sync.Payload, tc.wantV1)
		})
	}
}

func TestPublicV2SyncOutputHelpersAcceptV2Payloads(t *testing.T) {
	t.Parallel()

	v1 := buildGCOnlyUpdate(31, 2)
	v2, err := yjsbridge.ConvertUpdateToV2(v1)
	if err != nil {
		t.Fatalf("ConvertUpdateToV2() unexpected error: %v", err)
	}

	cases := []struct {
		name   string
		encode func() ([]byte, error)
		decode func([]byte) (*SyncMessage, error)
	}{
		{
			name: "sync step2 preserves V2-equivalent payload",
			encode: func() ([]byte, error) {
				return EncodeSyncStep2FromUpdatesV2(v2)
			},
			decode: DecodeSyncMessage,
		},
		{
			name: "sync update preserves V2-equivalent payload",
			encode: func() ([]byte, error) {
				return EncodeSyncUpdateV2(v2)
			},
			decode: DecodeSyncMessage,
		},
		{
			name: "protocol sync step2 preserves V2-equivalent payload",
			encode: func() ([]byte, error) {
				return EncodeProtocolSyncStep2FromUpdatesV2(v2)
			},
			decode: DecodeProtocolSyncMessage,
		},
		{
			name: "protocol sync update preserves V2-equivalent payload",
			encode: func() ([]byte, error) {
				return EncodeProtocolSyncUpdateV2(v2)
			},
			decode: DecodeProtocolSyncMessage,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			encoded, err := tc.encode()
			if err != nil {
				t.Fatalf("%s unexpected error: %v", tc.name, err)
			}
			decoded, err := tc.decode(encoded)
			if err != nil {
				t.Fatalf("decode unexpected error: %v", err)
			}
			assertV2PayloadEquivalentToV1(t, decoded.Payload, v1)
		})
	}
}

func TestPublicV2SyncStep2ContextCancellation(t *testing.T) {
	t.Parallel()

	update := buildGCOnlyUpdate(41, 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cases := []struct {
		name   string
		encode func(context.Context, ...[]byte) ([]byte, error)
	}{
		{
			name:   "sync step2 V2",
			encode: EncodeSyncStep2FromUpdatesV2Context,
		},
		{
			name:   "protocol sync step2 V2",
			encode: EncodeProtocolSyncStep2FromUpdatesV2Context,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := tc.encode(ctx, update)
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("%s error = %v, want %v", tc.name, err, context.Canceled)
			}
		})
	}
}

func TestPublicV2SyncUpdateHelpersReturnConversionErrors(t *testing.T) {
	t.Parallel()

	invalidUpdate := []byte{0xff}
	cases := []struct {
		name   string
		encode func([]byte) ([]byte, error)
	}{
		{
			name:   "sync update V2",
			encode: EncodeSyncUpdateV2,
		},
		{
			name:   "protocol sync update V2",
			encode: EncodeProtocolSyncUpdateV2,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			encoded, err := tc.encode(invalidUpdate)
			if err == nil {
				t.Fatalf("%s error = nil, want conversion error", tc.name)
			}
			if encoded != nil {
				t.Fatalf("%s encoded = %x, want nil on error", tc.name, encoded)
			}
		})
	}
}

func assertV2PayloadEquivalentToV1(t *testing.T, gotV2, wantV1 []byte) {
	t.Helper()

	format, err := yjsbridge.FormatFromUpdate(gotV2)
	if err != nil {
		t.Fatalf("FormatFromUpdate() unexpected error: %v", err)
	}
	if format != yjsbridge.UpdateFormatV2 {
		t.Fatalf("FormatFromUpdate() = %s, want %s", format, yjsbridge.UpdateFormatV2)
	}
	gotV1, err := yjsbridge.ConvertUpdateToV1(gotV2)
	if err != nil {
		t.Fatalf("ConvertUpdateToV1() unexpected error: %v", err)
	}
	if !bytes.Equal(gotV1, wantV1) {
		t.Fatalf("ConvertUpdateToV1() = %x, want %x", gotV1, wantV1)
	}
}
