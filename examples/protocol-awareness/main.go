package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sort"

	"yjs-go-bridge/pkg/yawareness"
	"yjs-go-bridge/pkg/yprotocol"
)

func main() {
	localClientID := uint32(42)

	manager := yawareness.NewStateManager(localClientID)

	if err := manager.SetLocalState(json.RawMessage(`{"name":"server","status":"present"}`)); err != nil {
		log.Fatalf("set local state: %v", err)
	}

	remoteUpdate := &yawareness.Update{
		Clients: []yawareness.ClientState{
			{
				ClientID: 7,
				Clock:    3,
				State:    json.RawMessage(`{"name":"alice","color":"blue","cursor":12}`),
			},
			{
				ClientID: 13,
				Clock:    1,
				State:    json.RawMessage(`{"name":"bob","focus":"sidebar"}`),
			},
		},
	}

	remoteAwarenessMsg, err := yawareness.EncodeProtocolUpdate(remoteUpdate)
	if err != nil {
		log.Fatalf("encode remote awareness: %v", err)
	}
	decodedRemote, err := yawareness.DecodeProtocolUpdate(remoteAwarenessMsg)
	if err != nil {
		log.Fatalf("decode remote awareness: %v", err)
	}
	manager.Apply(decodedRemote)

	fmt.Println("runtime: estado atual do awareness")
	snapshot := manager.Snapshot()
	for _, client := range snapshot.Clients {
		fmt.Printf("  client=%d clock=%d state=%s\n", client.ClientID, client.Clock, string(client.State))
	}

	clientIDs := snapshotClientIDs(snapshot)
	queryResponse := manager.UpdateForClients(clientIDs)
	queryResponsePayload, err := yawareness.EncodeProtocolUpdate(queryResponse)
	if err != nil {
		log.Fatalf("encode query response awareness: %v", err)
	}

	if _, err := yawareness.DecodeProtocolUpdate(queryResponsePayload); err != nil {
		log.Fatalf("decode query response awareness: %v", err)
	}

	renewed, err := manager.RenewLocalIfDue(yawareness.OutdatedTimeout / 2)
	if err != nil {
		log.Fatalf("renew local: %v", err)
	}
	if renewed {
		fmt.Println("runtime: local state foi renovado por heartbeat")
	}

	stale := manager.ExpireStale(yawareness.OutdatedTimeout)
	if len(stale) > 0 {
		fmt.Printf("runtime: removidos como stale: %v\n", stale)
	}

	syncStep1 := yprotocol.EncodeProtocolSyncStep1([]byte{0x00})
	syncUpdate := yprotocol.EncodeProtocolSyncUpdate([]byte{0xde, 0xad, 0xbe, 0xef})
	auth := yprotocol.EncodeProtocolAuthPermissionDenied("invalid token")

	queryMessage := yprotocol.EncodeProtocolQueryAwareness()
	if _, err := yprotocol.DecodeProtocolQueryAwareness(queryMessage); err != nil {
		log.Fatalf("decode query-awareness: %v", err)
	}

	stream := concatMessages(
		syncStep1,
		syncUpdate,
		auth,
		queryMessage,
		remoteAwarenessMsg,
		queryResponsePayload,
	)

	fmt.Printf("stream multiplexado: %s\n", hex.EncodeToString(stream))

	messages, err := yprotocol.ReadProtocolMessagesFromStream(context.Background(), bytes.NewReader(stream))
	if err != nil {
		log.Fatalf("read protocol stream: %v", err)
	}

	fmt.Println("decodificando protocolo multiplexado:")
	for idx, msg := range messages {
		fmt.Printf("[%d] protocol=%s\n", idx+1, msg.Protocol)
		switch {
		case msg.Sync != nil:
			fmt.Printf("    sync: %s payload=%d bytes\n", msg.Sync.Type, len(msg.Sync.Payload))
		case msg.Auth != nil:
			fmt.Printf("    auth: %s reason=%q\n", msg.Auth.Type, msg.Auth.Reason)
		case msg.QueryAwareness != nil:
			fmt.Println("    query-awareness: solicitação de snapshot")
		case msg.Awareness != nil:
			fmt.Printf("    awareness: %d clientes\n", len(msg.Awareness.Clients))
			for _, client := range msg.Awareness.Clients {
				fmt.Printf("      - client=%d clock=%d state=%s\n", client.ClientID, client.Clock, string(client.State))
			}
		default:
			fmt.Println("    mensagem desconhecida")
		}
	}
}

func snapshotClientIDs(snapshot *yawareness.Update) []uint32 {
	clientIDs := make([]uint32, 0, len(snapshot.Clients))
	for _, client := range snapshot.Clients {
		clientIDs = append(clientIDs, client.ClientID)
	}
	sort.Slice(clientIDs, func(i, j int) bool {
		return clientIDs[i] < clientIDs[j]
	})
	return clientIDs
}

func concatMessages(messages ...[]byte) []byte {
	stream := make([]byte, 0)
	for _, message := range messages {
		stream = append(stream, message...)
	}
	return stream
}
