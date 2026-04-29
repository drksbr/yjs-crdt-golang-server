package yhttp

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yjsbridge"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/ynodeproto"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yprotocol"
)

func TestProtocolPayloadToOwnerMessagesNormalizesV2SyncPayloads(t *testing.T) {
	t.Parallel()

	key := storage.DocumentKey{Namespace: "tests", DocumentID: "remote-owner-v2-normalization"}
	v2Update := mustDecodeYHTTPHex(t, "000002a50100000104060374686901020101000001010000")
	v1Update, err := yjsbridge.ConvertUpdateToV1(v2Update)
	if err != nil {
		t.Fatalf("ConvertUpdateToV1(v2) unexpected error: %v", err)
	}

	for _, tt := range []struct {
		name    string
		payload []byte
	}{
		{name: "sync update", payload: yprotocol.EncodeProtocolSyncUpdate(v2Update)},
		{name: "sync step2", payload: yprotocol.EncodeProtocolSyncStep2(v2Update)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			messages, err := protocolPayloadToOwnerMessages(key, "conn-a", 44, tt.payload)
			if err != nil {
				t.Fatalf("protocolPayloadToOwnerMessages() unexpected error: %v", err)
			}
			if len(messages) != 1 {
				t.Fatalf("len(messages) = %d, want 1", len(messages))
			}
			update, ok := messages[0].(*ynodeproto.DocumentUpdate)
			if !ok {
				t.Fatalf("messages[0] = %T, want *ynodeproto.DocumentUpdate", messages[0])
			}
			if !bytes.Equal(update.UpdateV1, v1Update) {
				t.Fatalf("UpdateV1 = %x, want canonical V1 %x", update.UpdateV1, v1Update)
			}
			if bytes.Equal(update.UpdateV1, v2Update) {
				t.Fatalf("UpdateV1 preserved V2 bytes: %x", update.UpdateV1)
			}
		})
	}
}

func TestProtocolPayloadToRemoteMessagesNormalizesV2SyncPayloads(t *testing.T) {
	t.Parallel()

	key := storage.DocumentKey{Namespace: "tests", DocumentID: "remote-peer-v2-normalization"}
	v2Update := mustDecodeYHTTPHex(t, "000002a50100000104060374686901020101000001010000")
	v1Update, err := yjsbridge.ConvertUpdateToV1(v2Update)
	if err != nil {
		t.Fatalf("ConvertUpdateToV1(v2) unexpected error: %v", err)
	}

	t.Run("sync update", func(t *testing.T) {
		t.Parallel()

		messages, err := protocolPayloadToRemoteMessages(key, "conn-a", 45, yprotocol.EncodeProtocolSyncUpdate(v2Update))
		if err != nil {
			t.Fatalf("protocolPayloadToRemoteMessages(update) unexpected error: %v", err)
		}
		update, ok := singleYHTTPMessage(t, messages).(*ynodeproto.DocumentUpdate)
		if !ok {
			t.Fatalf("messages[0] = %T, want *ynodeproto.DocumentUpdate", messages[0])
		}
		if !bytes.Equal(update.UpdateV1, v1Update) {
			t.Fatalf("UpdateV1 = %x, want canonical V1 %x", update.UpdateV1, v1Update)
		}
		if bytes.Equal(update.UpdateV1, v2Update) {
			t.Fatalf("UpdateV1 preserved V2 bytes: %x", update.UpdateV1)
		}
	})

	t.Run("sync step2", func(t *testing.T) {
		t.Parallel()

		messages, err := protocolPayloadToRemoteMessages(key, "conn-a", 45, yprotocol.EncodeProtocolSyncStep2(v2Update))
		if err != nil {
			t.Fatalf("protocolPayloadToRemoteMessages(step2) unexpected error: %v", err)
		}
		response, ok := singleYHTTPMessage(t, messages).(*ynodeproto.DocumentSyncResponse)
		if !ok {
			t.Fatalf("messages[0] = %T, want *ynodeproto.DocumentSyncResponse", messages[0])
		}
		if !bytes.Equal(response.UpdateV1, v1Update) {
			t.Fatalf("UpdateV1 = %x, want canonical V1 %x", response.UpdateV1, v1Update)
		}
		if bytes.Equal(response.UpdateV1, v2Update) {
			t.Fatalf("UpdateV1 preserved V2 bytes: %x", response.UpdateV1)
		}
	})
}

func singleYHTTPMessage(t *testing.T, messages []ynodeproto.Message) ynodeproto.Message {
	t.Helper()

	if len(messages) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(messages))
	}
	return messages[0]
}

func mustDecodeYHTTPHex(t *testing.T, value string) []byte {
	t.Helper()

	decoded, err := hex.DecodeString(value)
	if err != nil {
		t.Fatalf("hex.DecodeString(%q) unexpected error: %v", value, err)
	}
	return decoded
}
