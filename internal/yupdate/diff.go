package yupdate

import (
	"context"

	"yjs-go-bridge/internal/ytypes"
)

// DiffUpdateV1 retorna a parte de `update` que ainda não está coberta pelo state vector.
// O delete set é preservado integralmente, seguindo a semântica do Yjs.
func DiffUpdateV1(update, stateVector []byte) ([]byte, error) {
	return DiffUpdateV1Context(context.Background(), update, stateVector)
}

// DiffUpdateV1Context retorna a parte de `update` que ainda não está coberta
// pelo state vector, respeitando cancelamento do contexto.
func DiffUpdateV1Context(ctx context.Context, update, stateVector []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	state, err := DecodeStateVectorV1(stateVector)
	if err != nil {
		return nil, err
	}

	reader, err := NewLazyReaderV1(update, false)
	if err != nil {
		return nil, err
	}

	writer := newLazyWriterV1()
	for current := reader.Current(); current != nil; {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		client := current.ID().Client
		svClock := state[client]

		if current.Kind() == ytypes.KindSkip {
			if err := reader.Next(); err != nil {
				return nil, err
			}
			current = reader.Current()
			continue
		}

		if current.EndClock() > svClock {
			if current.ID().Clock < svClock {
				if err := writer.write(current, svClock-current.ID().Clock, 0); err != nil {
					return nil, err
				}
			} else {
				if err := writer.write(current, 0, 0); err != nil {
					return nil, err
				}
			}

			if err := reader.Next(); err != nil {
				return nil, err
			}
			current = reader.Current()
			for current != nil && current.ID().Client == client {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				if err := writer.write(current, 0, 0); err != nil {
					return nil, err
				}
				if err := reader.Next(); err != nil {
					return nil, err
				}
				current = reader.Current()
			}
			continue
		}

		for current != nil && current.ID().Client == client && current.EndClock() <= svClock {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if err := reader.Next(); err != nil {
				return nil, err
			}
			current = reader.Current()
		}
	}

	deleteSet, err := reader.ReadDeleteSet()
	if err != nil {
		return nil, err
	}

	out, err := writer.finish(nil)
	if err != nil {
		return nil, err
	}
	return AppendDeleteSetBlockV1(out, deleteSet), nil
}
