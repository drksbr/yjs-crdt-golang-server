package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage/memory"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yprotocol"
)

func main() {
	ctx := context.Background()
	key := storage.DocumentKey{
		Namespace:  "team-a",
		DocumentID: "provider-memory-demo",
	}

	store := memory.New()
	provider := yprotocol.NewProvider(yprotocol.ProviderConfig{Store: store})

	author, err := provider.Open(ctx, key, "conn-author", 101)
	if err != nil {
		log.Fatalf("abrindo author: %v", err)
	}

	seedUpdate := mustDecodeHex("01020100040103646f630161030103646f6302112200")
	updateDispatch, err := author.HandleEncodedMessages(yprotocol.EncodeProtocolSyncUpdate(seedUpdate))
	if err != nil {
		log.Fatalf("publicando update inicial: %v", err)
	}

	authorPresence, err := yprotocol.EncodeProtocolAwarenessUpdate(&yprotocol.AwarenessMessage{
		Clients: []yprotocol.AwarenessClient{{
			ClientID: author.ClientID(),
			Clock:    1,
			State:    json.RawMessage(`{"name":"author","cursor":3}`),
		}},
	})
	if err != nil {
		log.Fatalf("codificando awareness local: %v", err)
	}

	awarenessDispatch, err := author.HandleEncodedMessages(authorPresence)
	if err != nil {
		log.Fatalf("publicando awareness local: %v", err)
	}

	lateJoiner, err := provider.Open(ctx, key, "conn-reader", 202)
	if err != nil {
		log.Fatalf("abrindo late joiner: %v", err)
	}

	hello, err := yprotocol.EncodeProtocolEnvelopes(
		&yprotocol.ProtocolMessage{
			Protocol: yprotocol.ProtocolTypeSync,
			Sync: &yprotocol.SyncMessage{
				Type:    yprotocol.SyncMessageTypeStep1,
				Payload: []byte{0x00},
			},
		},
		&yprotocol.ProtocolMessage{
			Protocol:       yprotocol.ProtocolTypeQueryAwareness,
			QueryAwareness: &yprotocol.QueryAwarenessMessage{},
		},
	)
	if err != nil {
		log.Fatalf("codificando hello do late joiner: %v", err)
	}

	lateJoinerReply, err := lateJoiner.HandleEncodedMessages(hello)
	if err != nil {
		log.Fatalf("processando hello do late joiner: %v", err)
	}
	lateJoinerMessages, err := yprotocol.DecodeProtocolMessages(lateJoinerReply.Direct)
	if err != nil {
		log.Fatalf("decodificando reply direto do late joiner: %v", err)
	}

	record, err := author.Persist(nil)
	if err != nil {
		log.Fatalf("persistindo snapshot em memoria: %v", err)
	}

	if _, err := lateJoiner.Close(); err != nil {
		log.Fatalf("fechando late joiner: %v", err)
	}
	if _, err := author.Close(); err != nil {
		log.Fatalf("fechando author: %v", err)
	}

	reopenedProvider := yprotocol.NewProvider(yprotocol.ProviderConfig{Store: store})
	reopened, err := reopenedProvider.Open(ctx, key, "conn-reopen", 303)
	if err != nil {
		log.Fatalf("reabrindo provider: %v", err)
	}

	restoreProbe, err := yprotocol.EncodeProtocolEnvelopes(
		&yprotocol.ProtocolMessage{
			Protocol: yprotocol.ProtocolTypeSync,
			Sync: &yprotocol.SyncMessage{
				Type:    yprotocol.SyncMessageTypeStep1,
				Payload: []byte{0x00},
			},
		},
		&yprotocol.ProtocolMessage{
			Protocol:       yprotocol.ProtocolTypeQueryAwareness,
			QueryAwareness: &yprotocol.QueryAwarenessMessage{},
		},
	)
	if err != nil {
		log.Fatalf("codificando probe de restore: %v", err)
	}

	restoreReply, err := reopened.HandleEncodedMessages(restoreProbe)
	if err != nil {
		log.Fatalf("processando probe de restore: %v", err)
	}
	restoreMessages, err := yprotocol.DecodeProtocolMessages(restoreReply.Direct)
	if err != nil {
		log.Fatalf("decodificando restore direto: %v", err)
	}

	fmt.Println("publicacao inicial no provider local")
	fmt.Printf("  sync broadcast: %d bytes\n", len(updateDispatch.Broadcast))
	fmt.Printf("  awareness broadcast: %d bytes\n", len(awarenessDispatch.Broadcast))
	fmt.Println()

	fmt.Println("late joiner -> sync step1 + query-awareness")
	for _, line := range formatMessages(lateJoinerMessages) {
		fmt.Printf("  %s\n", line)
	}
	fmt.Println()

	fmt.Println("persistencia explicita")
	fmt.Printf("  stored_at: %s\n", record.StoredAt.UTC())
	fmt.Printf("  persisted update bytes: %d\n", len(record.Snapshot.UpdateV1))
	fmt.Println()

	fmt.Println("restore via memory store em um novo provider")
	for _, line := range formatMessages(restoreMessages) {
		fmt.Printf("  %s\n", line)
	}
}

func mustDecodeHex(value string) []byte {
	decoded, err := hex.DecodeString(value)
	if err != nil {
		log.Fatalf("decodificando hex %q: %v", value, err)
	}
	return decoded
}

func formatMessages(messages []*yprotocol.ProtocolMessage) []string {
	lines := make([]string, 0, len(messages))
	for _, message := range messages {
		switch message.Protocol {
		case yprotocol.ProtocolTypeSync:
			lines = append(lines, fmt.Sprintf("sync.%s payload=%s", syncTypeLabel(message.Sync.Type), hex.EncodeToString(message.Sync.Payload)))
		case yprotocol.ProtocolTypeAwareness:
			lines = append(lines, fmt.Sprintf("awareness snapshot=%s", formatAwareness(message.Awareness)))
		case yprotocol.ProtocolTypeAuth:
			lines = append(lines, fmt.Sprintf("auth.%d reason=%q", message.Auth.Type, message.Auth.Reason))
		case yprotocol.ProtocolTypeQueryAwareness:
			lines = append(lines, "query-awareness")
		}
	}
	return lines
}

func syncTypeLabel(typ yprotocol.SyncMessageType) string {
	switch typ {
	case yprotocol.SyncMessageTypeStep1:
		return "step1"
	case yprotocol.SyncMessageTypeStep2:
		return "step2"
	case yprotocol.SyncMessageTypeUpdate:
		return "update"
	default:
		return fmt.Sprintf("unknown(%d)", typ)
	}
}

func formatAwareness(update *yprotocol.AwarenessMessage) string {
	if update == nil || len(update.Clients) == 0 {
		return "[]"
	}

	clientIDs := make([]int, 0, len(update.Clients))
	for _, client := range update.Clients {
		clientIDs = append(clientIDs, int(client.ClientID))
	}
	sort.Ints(clientIDs)

	parts := make([]string, 0, len(clientIDs))
	for _, clientID := range clientIDs {
		for _, client := range update.Clients {
			if client.ClientID != uint32(clientID) {
				continue
			}
			parts = append(parts, fmt.Sprintf("{client:%d clock:%d state:%s}", client.ClientID, client.Clock, string(client.State)))
			break
		}
	}

	return "[" + strings.Join(parts, ", ") + "]"
}
