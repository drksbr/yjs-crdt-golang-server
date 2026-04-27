package yhttp

import (
	"sync"

	"github.com/coder/websocket"

	"yjs-go-bridge/pkg/storage"
)

type roomRegistry struct {
	mu    sync.RWMutex
	rooms map[storage.DocumentKey]map[string]*peerSocket
}

type peerSocket struct {
	conn    *websocket.Conn
	writeMu sync.Mutex
}

func newRoomRegistry() roomRegistry {
	return roomRegistry{
		rooms: make(map[storage.DocumentKey]map[string]*peerSocket),
	}
}

func (r *roomRegistry) add(key storage.DocumentKey, connectionID string, conn *websocket.Conn) *peerSocket {
	r.mu.Lock()
	defer r.mu.Unlock()

	room := r.rooms[key]
	if room == nil {
		room = make(map[string]*peerSocket)
		r.rooms[key] = room
	}

	peer := &peerSocket{conn: conn}
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

func (r *roomRegistry) peersExcept(key storage.DocumentKey, excludeConnectionID string) []*peerSocket {
	r.mu.RLock()
	defer r.mu.RUnlock()

	room := r.rooms[key]
	if len(room) == 0 {
		return nil
	}

	out := make([]*peerSocket, 0, len(room))
	for connectionID, peer := range room {
		if connectionID == excludeConnectionID {
			continue
		}
		out = append(out, peer)
	}
	return out
}
