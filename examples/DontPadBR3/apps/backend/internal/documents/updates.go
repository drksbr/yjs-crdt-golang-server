package documents

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yprotocol"
)

func (s *Service) applyUpdateAndPersist(ctx context.Context, documentID string, update []byte) error {
	key := storage.DocumentKey{
		Namespace:  s.namespace,
		DocumentID: documentID,
	}
	connectionID := fmt.Sprintf("flush-%d", time.Now().UnixNano())
	clientID := s.nextClientID.Add(1)
	if clientID == 0 {
		clientID = uint32(time.Now().UnixNano())
	}

	conn, err := s.provider.Open(ctx, key, connectionID, clientID)
	if err != nil {
		return err
	}
	defer func() {
		_, _ = conn.Close()
	}()

	stream := yprotocol.EncodeProtocolSyncUpdate(update)
	if _, err := conn.HandleEncodedMessagesContext(ctx, stream); err != nil {
		return err
	}
	_, err = conn.Persist(ctx)
	if err != nil && !errors.Is(err, yprotocol.ErrPersistenceDisabled) {
		return err
	}
	return nil
}
