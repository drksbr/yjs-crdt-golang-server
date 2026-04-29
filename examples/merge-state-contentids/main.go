package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/yjsbridge"
)

func main() {
	ctx := context.Background()

	left, err := mustDecodeHex("01020100040103646f630161030103646f6302112200")
	if err != nil {
		log.Fatalf("decodificando update left: %v", err)
	}

	right, err := mustDecodeHex("01020300040103646f630162030103646f63013300")
	if err != nil {
		log.Fatalf("decodificando update right: %v", err)
	}

	merged, err := yjsbridge.MergeUpdates(left, right)
	if err != nil {
		log.Fatalf("mergendo updates: %v", err)
	}

	fmt.Println("update left:", hex.EncodeToString(left))
	fmt.Println("update right:", hex.EncodeToString(right))
	fmt.Println("update merged:", hex.EncodeToString(merged))

	stateVector, err := yjsbridge.StateVectorFromUpdate(merged)
	if err != nil {
		log.Fatalf("extraindo state vector do merged: %v", err)
	}
	fmt.Println("state vector (merged):", formatStateVector(stateVector))

	aggVector, err := yjsbridge.StateVectorFromUpdates(left, right)
	if err != nil {
		log.Fatalf("extraindo state vector agregado left+right: %v", err)
	}
	fmt.Println("state vector agregado left+right:", formatStateVector(aggVector))

	leftEncodedVector, err := yjsbridge.EncodeStateVectorFromUpdate(left)
	if err != nil {
		log.Fatalf("serializando state vector do left: %v", err)
	}
	missingForLeft, err := yjsbridge.DiffUpdate(merged, leftEncodedVector)
	if err != nil {
		log.Fatalf("extraindo diff do merged contra left: %v", err)
	}
	replayedFromLeft, err := yjsbridge.MergeUpdates(left, missingForLeft)
	if err != nil {
		log.Fatalf("reaplicando diff sobre left: %v", err)
	}
	fmt.Println("delta faltante para left:", hex.EncodeToString(missingForLeft))
	fmt.Println("left + delta reconverge para merged:", bytes.Equal(replayedFromLeft, merged))

	encodedVector, err := yjsbridge.EncodeStateVectorFromUpdates(left, right)
	if err != nil {
		log.Fatalf("serializando state vector agregado: %v", err)
	}
	decodedVector, err := yjsbridge.DecodeStateVector(encodedVector)
	if err != nil {
		log.Fatalf("decodificando state vector agregado: %v", err)
	}
	fmt.Println("state vector agregado serializado:", hex.EncodeToString(encodedVector))
	fmt.Println("state vector agregado decodificado:", formatStateVector(decodedVector))

	leftIDs, err := yjsbridge.CreateContentIDsFromUpdate(left)
	if err != nil {
		log.Fatalf("extraindo content ids do left: %v", err)
	}
	rightIDs, err := yjsbridge.CreateContentIDsFromUpdate(right)
	if err != nil {
		log.Fatalf("extraindo content ids do right: %v", err)
	}
	mergedIDs, err := yjsbridge.ContentIDsFromUpdates(left, right)
	if err != nil {
		log.Fatalf("extraindo content ids agregados: %v", err)
	}

	leftOnlyIDs := yjsbridge.DiffContentIDs(mergedIDs, rightIDs)
	intersected, err := yjsbridge.IntersectUpdateWithContentIDsContext(ctx, merged, leftOnlyIDs)
	if err != nil {
		log.Fatalf("intersectando update com ids: %v", err)
	}
	rightOnlyIDs := yjsbridge.DiffContentIDs(mergedIDs, leftIDs)
	rightOnly, err := yjsbridge.IntersectUpdateWithContentIDsContext(ctx, merged, rightOnlyIDs)
	if err != nil {
		log.Fatalf("intersectando update right-only com ids: %v", err)
	}

	intersectStateVector, err := yjsbridge.StateVectorFromUpdate(intersected)
	if err != nil {
		log.Fatalf("extraindo state vector da interseção: %v", err)
	}
	rightOnlyStateVector, err := yjsbridge.StateVectorFromUpdate(rightOnly)
	if err != nil {
		log.Fatalf("extraindo state vector da interseção right-only: %v", err)
	}

	fmt.Printf("content ids left: %s\n", formatContentIDsSummary(leftIDs))
	fmt.Printf("content ids right: %s\n", formatContentIDsSummary(rightIDs))
	fmt.Printf("content ids merged: %s\n", formatContentIDsSummary(mergedIDs))

	encodedContentIDs, err := yjsbridge.EncodeContentIDs(mergedIDs)
	if err != nil {
		log.Fatalf("serializando content ids merged: %v", err)
	}
	fmt.Println("content ids merged serializado:", hex.EncodeToString(encodedContentIDs))

	reencoded, err := yjsbridge.DecodeContentIDs(encodedContentIDs)
	if err != nil {
		log.Fatalf("decodificando content ids merged: %v", err)
	}
	fmt.Printf("content ids merged roundtrip: %s\n", formatContentIDsSummary(reencoded))
	fmt.Println("update apenas do left via intersect:", hex.EncodeToString(intersected))
	fmt.Println("update apenas do right via intersect:", hex.EncodeToString(rightOnly))
	fmt.Println("state vector do trecho left-intersected:", formatStateVector(intersectStateVector))
	fmt.Println("state vector do trecho right-intersected:", formatStateVector(rightOnlyStateVector))
}

func mustDecodeHex(payload string) ([]byte, error) {
	return hex.DecodeString(payload)
}

func formatStateVector(state map[uint32]uint32) string {
	clients := make([]int, 0, len(state))
	for client := range state {
		clients = append(clients, int(client))
	}
	sort.Ints(clients)

	parts := make([]string, 0, len(state))
	for _, client := range clients {
		parts = append(parts, fmt.Sprintf("%d:%d", client, state[uint32(client)]))
	}

	return "{" + strings.Join(parts, ", ") + "}"
}

func formatContentIDsSummary(ids *yjsbridge.ContentIDs) string {
	if ids == nil || ids.IsEmpty() {
		return "{inserts:[], deletes:[]}"
	}

	return fmt.Sprintf("{inserts:%s, deletes:%s}",
		formatContentRanges(ids.InsertRanges()),
		formatContentRanges(ids.DeleteRanges()),
	)
}

func formatContentRanges(values []yjsbridge.IDRange) string {
	if len(values) == 0 {
		return "[]"
	}

	parts := make([]string, 0, len(values))
	for _, entry := range values {
		parts = append(parts, fmt.Sprintf("{Client:%d Clock:%d Length:%d}", entry.Client, entry.Clock, entry.Length))
	}
	return "[" + strings.Join(parts, " ") + "]"
}
