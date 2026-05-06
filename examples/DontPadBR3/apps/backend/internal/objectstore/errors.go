package objectstore

import "github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/common"

func errPayloadTooLarge() error {
	return common.ErrPayloadTooLarge
}
