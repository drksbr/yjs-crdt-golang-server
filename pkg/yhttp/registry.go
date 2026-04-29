package yhttp

import (
	"context"
	"sync"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
)

type roomPeer interface {
	deliver(ctx context.Context, payload []byte) error
	close(reason string) error
}

type roomRegistry struct {
	mu    sync.RWMutex
	rooms map[storage.DocumentKey]map[string]roomPeer
}

func newRoomRegistry() roomRegistry {
	return roomRegistry{
		rooms: make(map[storage.DocumentKey]map[string]roomPeer),
	}
}

func (r *roomRegistry) add(key storage.DocumentKey, connectionID string, peer roomPeer) roomPeer {
	r.mu.Lock()
	defer r.mu.Unlock()

	room := r.rooms[key]
	if room == nil {
		room = make(map[string]roomPeer)
		r.rooms[key] = room
	}

	room[connectionID] = peer
	return peer
}

func (r *roomRegistry) remove(key storage.DocumentKey, connectionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	room := r.rooms[key]
	if room == nil {
		return
	}

	delete(room, connectionID)
	if len(room) == 0 {
		delete(r.rooms, key)
	}
}

func (r *roomRegistry) peersExcept(key storage.DocumentKey, excludeConnectionID string) []roomPeer {
	r.mu.RLock()
	defer r.mu.RUnlock()

	room := r.rooms[key]
	if len(room) == 0 {
		return nil
	}

	out := make([]roomPeer, 0, len(room))
	for connectionID, peer := range room {
		if connectionID == excludeConnectionID {
			continue
		}
		out = append(out, peer)
	}
	return out
}
