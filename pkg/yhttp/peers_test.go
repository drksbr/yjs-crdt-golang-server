package yhttp

import (
	"bytes"
	"context"
	"encoding/hex"
	"sync"
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
		tt := tt
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

func TestProtocolPayloadToOwnerMessagesForFormatUsesNegotiatedV2(t *testing.T) {
	t.Parallel()

	key := storage.DocumentKey{Namespace: "tests", DocumentID: "remote-owner-v2-native"}
	updateV1 := buildGCOnlyUpdate(84, 2)
	updateV2, err := yjsbridge.ConvertUpdateToV2(updateV1)
	if err != nil {
		t.Fatalf("ConvertUpdateToV2() unexpected error: %v", err)
	}

	t.Run("sync step1", func(t *testing.T) {
		t.Parallel()

		stateVector := []byte{0x00}
		messages, err := protocolPayloadToOwnerMessagesForFormat(
			key,
			"conn-owner-v2",
			48,
			yprotocol.EncodeProtocolSyncStep1(stateVector),
			yjsbridge.UpdateFormatV2,
		)
		if err != nil {
			t.Fatalf("protocolPayloadToOwnerMessagesForFormat(step1 V2) unexpected error: %v", err)
		}
		requestV2, ok := singleYHTTPMessage(t, messages).(*ynodeproto.DocumentSyncRequestV2)
		if !ok {
			t.Fatalf("messages[0] = %T, want *ynodeproto.DocumentSyncRequestV2", messages[0])
		}
		if !bytes.Equal(requestV2.StateVector, stateVector) {
			t.Fatalf("StateVector = %x, want %x", requestV2.StateVector, stateVector)
		}
	})

	t.Run("sync update", func(t *testing.T) {
		t.Parallel()

		messages, err := protocolPayloadToOwnerMessagesForFormat(
			key,
			"conn-owner-v2",
			48,
			yprotocol.EncodeProtocolSyncUpdate(updateV2),
			yjsbridge.UpdateFormatV2,
		)
		if err != nil {
			t.Fatalf("protocolPayloadToOwnerMessagesForFormat(update V2) unexpected error: %v", err)
		}
		update, ok := singleYHTTPMessage(t, messages).(*ynodeproto.DocumentUpdateV2FromEdge)
		if !ok {
			t.Fatalf("messages[0] = %T, want *ynodeproto.DocumentUpdateV2FromEdge", messages[0])
		}
		if !bytes.Equal(update.UpdateV2, updateV2) {
			t.Fatalf("UpdateV2 = %x, want exact client V2 bytes %x", update.UpdateV2, updateV2)
		}
		assertYHTTPV2EquivalentToV1(t, "owner update", update.UpdateV2, updateV1)
	})
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

func TestProtocolPayloadToRemoteMessagesForFormatUsesNegotiatedV2(t *testing.T) {
	t.Parallel()

	key := storage.DocumentKey{Namespace: "tests", DocumentID: "remote-peer-v2-egress"}
	updateV1 := buildGCOnlyUpdate(81, 2)

	updateMessages, err := protocolPayloadToRemoteMessagesForFormat(
		key,
		"conn-v2",
		46,
		yprotocol.EncodeProtocolSyncUpdate(updateV1),
		yjsbridge.UpdateFormatV2,
	)
	if err != nil {
		t.Fatalf("protocolPayloadToRemoteMessagesForFormat(update V2) unexpected error: %v", err)
	}
	updateV2, ok := singleYHTTPMessage(t, updateMessages).(*ynodeproto.DocumentUpdateV2)
	if !ok {
		t.Fatalf("messages[0] = %T, want *ynodeproto.DocumentUpdateV2", updateMessages[0])
	}
	assertYHTTPV2EquivalentToV1(t, "remote update", updateV2.UpdateV2, updateV1)

	step2Messages, err := protocolPayloadToRemoteMessagesForFormat(
		key,
		"conn-v2",
		46,
		yprotocol.EncodeProtocolSyncStep2(updateV1),
		yjsbridge.UpdateFormatV2,
	)
	if err != nil {
		t.Fatalf("protocolPayloadToRemoteMessagesForFormat(step2 V2) unexpected error: %v", err)
	}
	responseV2, ok := singleYHTTPMessage(t, step2Messages).(*ynodeproto.DocumentSyncResponseV2)
	if !ok {
		t.Fatalf("messages[0] = %T, want *ynodeproto.DocumentSyncResponseV2", step2Messages[0])
	}
	assertYHTTPV2EquivalentToV1(t, "remote sync response", responseV2.UpdateV2, updateV1)
}

func TestRemoteMessageToProtocolPayloadAcceptsNegotiatedV2Messages(t *testing.T) {
	t.Parallel()

	key := storage.DocumentKey{Namespace: "tests", DocumentID: "remote-message-v2"}
	updateV1 := buildGCOnlyUpdate(82, 2)
	updateV2, err := yjsbridge.ConvertUpdateToV2(updateV1)
	if err != nil {
		t.Fatalf("ConvertUpdateToV2() unexpected error: %v", err)
	}

	for _, tt := range []struct {
		name     string
		message  ynodeproto.Message
		wantType yprotocol.SyncMessageType
	}{
		{
			name: "sync response v2",
			message: &ynodeproto.DocumentSyncResponseV2{
				DocumentKey:  key,
				ConnectionID: "conn-v2",
				Epoch:        47,
				UpdateV2:     updateV2,
			},
			wantType: yprotocol.SyncMessageTypeStep2,
		},
		{
			name: "sync update v2",
			message: &ynodeproto.DocumentUpdateV2{
				DocumentKey:  key,
				ConnectionID: "conn-v2",
				Epoch:        47,
				UpdateV2:     updateV2,
			},
			wantType: yprotocol.SyncMessageTypeUpdate,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			payload, closeMessage, err := remoteMessageToProtocolPayload(tt.message)
			if err != nil {
				t.Fatalf("remoteMessageToProtocolPayload() unexpected error: %v", err)
			}
			if closeMessage != nil {
				t.Fatalf("closeMessage = %#v, want nil", closeMessage)
			}
			messages, err := yprotocol.DecodeProtocolMessages(payload)
			if err != nil {
				t.Fatalf("DecodeProtocolMessages() unexpected error: %v", err)
			}
			if len(messages) != 1 || messages[0].Sync == nil {
				t.Fatalf("messages = %#v, want one sync message", messages)
			}
			if messages[0].Sync.Type != tt.wantType {
				t.Fatalf("sync type = %v, want %v", messages[0].Sync.Type, tt.wantType)
			}
			if !bytes.Equal(messages[0].Sync.Payload, updateV2) {
				t.Fatalf("sync payload = %x, want exact negotiated V2 bytes %x", messages[0].Sync.Payload, updateV2)
			}
			assertYHTTPV2EquivalentToV1(t, tt.name, messages[0].Sync.Payload, updateV1)
		})
	}
}

func TestProtocolPayloadForSyncOutputFormatPreservesExistingV2Bytes(t *testing.T) {
	t.Parallel()

	updateV1 := buildGCOnlyUpdate(83, 2)
	updateV2, err := yjsbridge.ConvertUpdateToV2(updateV1)
	if err != nil {
		t.Fatalf("ConvertUpdateToV2() unexpected error: %v", err)
	}
	payload := yprotocol.EncodeProtocolSyncUpdate(updateV2)

	converted, err := protocolPayloadForSyncOutputFormat(payload, yjsbridge.UpdateFormatV2)
	if err != nil {
		t.Fatalf("protocolPayloadForSyncOutputFormat(V2) unexpected error: %v", err)
	}
	messages, err := yprotocol.DecodeProtocolMessages(converted)
	if err != nil {
		t.Fatalf("DecodeProtocolMessages(converted) unexpected error: %v", err)
	}
	if len(messages) != 1 || messages[0].Sync == nil {
		t.Fatalf("messages = %#v, want one sync message", messages)
	}
	if !bytes.Equal(messages[0].Sync.Payload, updateV2) {
		t.Fatalf("sync payload = %x, want existing V2 bytes %x", messages[0].Sync.Payload, updateV2)
	}
	if bytes.Equal(messages[0].Sync.Payload, updateV1) {
		t.Fatalf("sync payload normalized back to V1 bytes: %x", messages[0].Sync.Payload)
	}
}

func TestProtocolPayloadForSyncOutputFormatConvertsSyncOnly(t *testing.T) {
	t.Parallel()

	updateV1 := buildGCOnlyUpdate(71, 2)
	payload, err := yprotocol.EncodeProtocolEnvelopes(
		&yprotocol.ProtocolMessage{
			Protocol: yprotocol.ProtocolTypeSync,
			Sync: &yprotocol.SyncMessage{
				Type:    yprotocol.SyncMessageTypeUpdate,
				Payload: updateV1,
			},
		},
		&yprotocol.ProtocolMessage{
			Protocol:       yprotocol.ProtocolTypeQueryAwareness,
			QueryAwareness: &yprotocol.QueryAwarenessMessage{},
		},
	)
	if err != nil {
		t.Fatalf("EncodeProtocolEnvelopes() unexpected error: %v", err)
	}

	converted, err := protocolPayloadForSyncOutputFormat(payload, yjsbridge.UpdateFormatV2)
	if err != nil {
		t.Fatalf("protocolPayloadForSyncOutputFormat(V2) unexpected error: %v", err)
	}
	messages, err := yprotocol.DecodeProtocolMessages(converted)
	if err != nil {
		t.Fatalf("DecodeProtocolMessages(converted) unexpected error: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("len(messages) = %d, want 2", len(messages))
	}
	if messages[0].Sync == nil {
		t.Fatalf("messages[0] = %#v, want sync", messages[0])
	}
	convertedBack, err := yjsbridge.ConvertUpdateToV1(messages[0].Sync.Payload)
	if err != nil {
		t.Fatalf("ConvertUpdateToV1(converted sync) unexpected error: %v", err)
	}
	if !bytes.Equal(convertedBack, updateV1) {
		t.Fatalf("converted sync V1 = %x, want %x", convertedBack, updateV1)
	}
	if messages[1].QueryAwareness == nil {
		t.Fatalf("messages[1] = %#v, want query awareness preserved", messages[1])
	}
}

func TestSwitchableRemoteStreamPeerSwitchTargetIsIdempotent(t *testing.T) {
	t.Parallel()

	peer := newSwitchableRemoteStreamPeer(storage.DocumentKey{Namespace: "tests", DocumentID: "switch-target"}, "conn-a")
	first := &recordingForwardDeliveryTarget{}
	second := &recordingForwardDeliveryTarget{}

	var wg sync.WaitGroup
	wg.Add(1)
	errCh := make(chan error, 1)
	go func() {
		defer wg.Done()
		errCh <- peer.deliver(context.Background(), []byte("payload"))
	}()

	peer.switchTarget(first)
	peer.switchTarget(second)
	wg.Wait()

	if err := <-errCh; err != nil {
		t.Fatalf("peer.deliver() unexpected error: %v", err)
	}
	if first.deliveries()+second.deliveries() != 1 {
		t.Fatalf("deliveries = first:%d second:%d, want exactly one", first.deliveries(), second.deliveries())
	}

	peer.clearSession()
	peer.switchTarget(second)
	if err := peer.deliver(context.Background(), []byte("again")); err != nil {
		t.Fatalf("peer.deliver(after clear) unexpected error: %v", err)
	}
	if second.deliveries() == 0 {
		t.Fatal("second target did not receive delivery after clear/switch")
	}
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

type recordingForwardDeliveryTarget struct {
	mu    sync.Mutex
	count int
}

func (t *recordingForwardDeliveryTarget) deliver(context.Context, []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.count++
	return nil
}

func (t *recordingForwardDeliveryTarget) deliveries() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.count
}
