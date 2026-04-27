package yhttp

import "github.com/coder/websocket"

func newWebsocketRoomPeer(conn *websocket.Conn) roomPeer {
	return &websocketPeer{conn: conn}
}
