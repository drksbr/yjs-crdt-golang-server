package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/yjsbridge"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yprotocol"
)

func main() {
	const (
		serverClientID = uint32(1)
		clientClientID = uint32(7)
	)

	server := yprotocol.NewSession(serverClientID)
	client := yprotocol.NewSession(clientClientID)

	serverSeed := mustDecodeHex("01020100040103646f630161030103646f6302112200")
	if err := server.LoadUpdate(serverSeed); err != nil {
		log.Fatalf("load server update: %v", err)
	}

	if err := server.Awareness().SetLocalState(json.RawMessage(`{"name":"server","status":"ready"}`)); err != nil {
		log.Fatalf("set server awareness: %v", err)
	}

	clientStateVector, err := yjsbridge.EncodeStateVectorFromUpdate(client.UpdateV1())
	if err != nil {
		log.Fatalf("encode client state vector: %v", err)
	}

	clientHello, err := yprotocol.EncodeProtocolEnvelopes(
		&yprotocol.ProtocolMessage{
			Protocol: yprotocol.ProtocolTypeSync,
			Sync: &yprotocol.SyncMessage{
				Type:    yprotocol.SyncMessageTypeStep1,
				Payload: clientStateVector,
			},
		},
		&yprotocol.ProtocolMessage{
			Protocol:       yprotocol.ProtocolTypeQueryAwareness,
			QueryAwareness: &yprotocol.QueryAwarenessMessage{},
		},
	)
	if err != nil {
		log.Fatalf("encode client hello: %v", err)
	}

	serverReply, err := server.HandleEncodedMessages(clientHello)
	if err != nil {
		log.Fatalf("server handle hello: %v", err)
	}
	if len(serverReply) == 0 {
		log.Fatal("server returned no sync/query-awareness response")
	}

	clientReply, err := client.HandleEncodedMessages(serverReply)
	if err != nil {
		log.Fatalf("client handle server reply: %v", err)
	}

	fmt.Println("sync step1 -> step2")
	fmt.Printf("  server state vector: %s\n", formatStateVector(mustStateVector(server.UpdateV1())))
	fmt.Printf("  client state vector: %s\n", formatStateVector(mustStateVector(client.UpdateV1())))
	fmt.Printf("  converged: %v\n", bytes.Equal(server.UpdateV1(), client.UpdateV1()))
	fmt.Printf("  follow-up reply after step2: %d bytes\n", len(clientReply))
	fmt.Println()

	fmt.Println("query-awareness -> awareness snapshot")
	fmt.Printf("  client awareness snapshot: %s\n", formatAwarenessSnapshot(client.Awareness().Snapshot()))
	fmt.Println()

	if err := client.Awareness().SetLocalState(json.RawMessage(`{"name":"client","cursor":{"anchor":12,"head":12}}`)); err != nil {
		log.Fatalf("set client awareness: %v", err)
	}

	clientPresence := client.Awareness().UpdateForClients([]uint32{clientClientID})
	clientPresenceEnvelope, err := yprotocol.EncodeProtocolEnvelope(&yprotocol.ProtocolMessage{
		Protocol:  yprotocol.ProtocolTypeAwareness,
		Awareness: clientPresence,
	})
	if err != nil {
		log.Fatalf("encode client awareness envelope: %v", err)
	}

	serverBroadcast, err := server.HandleEncodedMessages(clientPresenceEnvelope)
	if err != nil {
		log.Fatalf("server handle client awareness: %v", err)
	}

	fmt.Println("aplicacao de awareness inbound")
	fmt.Printf("  server awareness snapshot: %s\n", formatAwarenessSnapshot(server.Awareness().Snapshot()))
	fmt.Printf("  outbound after inbound awareness: %d bytes\n", len(serverBroadcast))
}

func mustDecodeHex(value string) []byte {
	decoded, err := hex.DecodeString(value)
	if err != nil {
		log.Fatalf("decode hex %q: %v", value, err)
	}
	return decoded
}

func mustStateVector(update []byte) map[uint32]uint32 {
	stateVector, err := yjsbridge.StateVectorFromUpdate(update)
	if err != nil {
		log.Fatalf("state vector from update: %v", err)
	}
	return stateVector
}

func formatStateVector(state map[uint32]uint32) string {
	if len(state) == 0 {
		return "{}"
	}

	clients := make([]int, 0, len(state))
	for clientID := range state {
		clients = append(clients, int(clientID))
	}
	sort.Ints(clients)

	parts := make([]string, 0, len(clients))
	for _, clientID := range clients {
		parts = append(parts, fmt.Sprintf("%d:%d", clientID, state[uint32(clientID)]))
	}

	return "{" + strings.Join(parts, ", ") + "}"
}

func formatAwarenessSnapshot(snapshot *yprotocol.AwarenessMessage) string {
	if snapshot == nil || len(snapshot.Clients) == 0 {
		return "[]"
	}

	parts := make([]string, 0, len(snapshot.Clients))
	for _, client := range snapshot.Clients {
		parts = append(parts, fmt.Sprintf("{client:%d clock:%d state:%s}", client.ClientID, client.Clock, string(client.State)))
	}

	return "[" + strings.Join(parts, ", ") + "]"
}
