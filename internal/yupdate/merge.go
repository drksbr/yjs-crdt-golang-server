package yupdate

import (
	"context"
	"fmt"
	"slices"

	"yjs-go-bridge/internal/ytypes"
)

type blockSetV1 struct {
	clients map[uint32][]ytypes.Struct
}

type decodedMergeUpdateV1 struct {
	blockSet  *blockSetV1
	deleteSet *ytypes.DeleteSet
}

// MergeUpdatesV1Context consolida múltiplos updates V1 em um único update,
// respeitando cancelamento do contexto durante a etapa de agregação.
func MergeUpdatesV1Context(ctx context.Context, updates ...[]byte) ([]byte, error) {
	merged, err := aggregatePayloadsInParallel(ctx, updates, 0, decodeMergeUpdateV1, mergeDecodedUpdatesV1)
	if err != nil {
		return nil, err
	}

	out, err := encodeStructGroupsV1(merged.blockSet.clients)
	if err != nil {
		return nil, err
	}
	return AppendDeleteSetBlockV1(out, merged.deleteSet), nil
}

// MergeUpdatesV1 consolida múltiplos updates V1 em um único update.
func MergeUpdatesV1(updates ...[]byte) ([]byte, error) {
	return MergeUpdatesV1Context(context.Background(), updates...)
}

func newBlockSetV1(structs []ytypes.Struct) *blockSetV1 {
	return &blockSetV1{clients: groupStructsByClient(structs)}
}

func decodeMergeUpdateV1(_ context.Context, _ int, update []byte) (decodedMergeUpdateV1, error) {
	decoded, err := DecodeV1(update)
	if err != nil {
		return decodedMergeUpdateV1{}, err
	}

	return decodedMergeUpdateV1{
		blockSet:  newBlockSetV1(decoded.Structs),
		deleteSet: decoded.DeleteSet,
	}, nil
}

func mergeDecodedUpdatesV1(_ context.Context, entries []decodedMergeUpdateV1) (decodedMergeUpdateV1, error) {
	merged := decodedMergeUpdateV1{
		blockSet:  &blockSetV1{clients: make(map[uint32][]ytypes.Struct)},
		deleteSet: ytypes.NewDeleteSet(),
	}
	for _, entry := range entries {
		if entry.blockSet != nil {
			if err := merged.blockSet.insertInto(entry.blockSet); err != nil {
				return decodedMergeUpdateV1{}, err
			}
		}
		if entry.deleteSet != nil {
			if err := merged.deleteSet.Merge(entry.deleteSet); err != nil {
				return decodedMergeUpdateV1{}, err
			}
		}
	}
	return merged, nil
}

func (b *blockSetV1) insertInto(other *blockSetV1) error {
	for client, newRefs := range other.clients {
		currentRefs, ok := b.clients[client]
		if !ok {
			b.clients[client] = slices.Clone(newRefs)
			continue
		}

		localIsLeft := currentRefs[0].ID().Clock < newRefs[0].ID().Clock
		left := currentRefs
		right := newRefs
		if !localIsLeft {
			left, right = right, left
		}

		lastLeft := left[len(left)-1]
		firstRight := right[0]
		gap := int64(firstRight.ID().Clock) - int64(lastLeft.EndClock())
		if gap >= 0 {
			merged, err := mergeDisjointStructLists(client, left, right, uint32(gap))
			if err != nil {
				return err
			}
			b.clients[client] = merged
			continue
		}

		merged, err := mergeOverlappingStructLists(client, left, right)
		if err != nil {
			return err
		}
		b.clients[client] = merged
	}
	return nil
}

func mergeDisjointStructLists(client uint32, left, right []ytypes.Struct, gap uint32) ([]ytypes.Struct, error) {
	if len(left) == 0 {
		return slices.Clone(right), nil
	}
	if len(right) == 0 {
		return slices.Clone(left), nil
	}

	merged := make([]ytypes.Struct, 0, len(left)+len(right)+1)
	merged = append(merged, left...)
	if gap > 0 {
		skip, err := ytypes.NewSkip(ytypes.ID{Client: client, Clock: left[len(left)-1].EndClock()}, gap)
		if err != nil {
			return nil, err
		}
		merged = append(merged, skip)
	}
	merged = append(merged, right...)
	return merged, nil
}

func mergeOverlappingStructLists(client uint32, left, right []ytypes.Struct) ([]ytypes.Struct, error) {
	if len(left) == 0 {
		return slices.Clone(right), nil
	}
	if len(right) == 0 {
		return slices.Clone(left), nil
	}

	result := make([]ytypes.Struct, 0, len(left)+len(right))
	nextExpected := left[0].ID().Clock
	li, ri := 0, 0
	var lblock, rblock ytypes.Struct
	if len(left) > 0 {
		lblock = left[0]
	}
	if len(right) > 0 {
		rblock = right[0]
	}

	addToResult := func(block ytypes.Struct) {
		result = append(result, block)
		nextExpected = block.EndClock()
	}

	apply := func(block ytypes.Struct, refs []ytypes.Struct, index *int) (ytypes.Struct, error) {
		for block != nil && (isSkip(block) || block.EndClock() <= nextExpected) {
			*index++
			if *index >= len(refs) {
				return nil, nil
			}
			block = refs[*index]
		}
		if block != nil && block.ID().Clock < nextExpected && block.EndClock() > nextExpected {
			diff := nextExpected - block.ID().Clock
			sliced, err := sliceStructV1(block, diff)
			if err != nil {
				return nil, err
			}
			block = sliced
		}
		for block != nil && block.ID().Clock == nextExpected && !isSkip(block) {
			addToResult(block)
			*index++
			if *index >= len(refs) {
				return nil, nil
			}
			block = refs[*index]
		}
		return block, nil
	}

	for li < len(left) && ri < len(right) {
		var err error
		lblock, err = apply(lblock, left, &li)
		if err != nil {
			return nil, err
		}
		rblock, err = apply(rblock, right, &ri)
		if err != nil {
			return nil, err
		}
		if lblock == nil || rblock == nil {
			break
		}
		minNextClock := minClock(lblock.ID().Clock, rblock.ID().Clock)
		if minNextClock > nextExpected {
			skip, err := ytypes.NewSkip(ytypes.ID{Client: client, Clock: nextExpected}, minNextClock-nextExpected)
			if err != nil {
				return nil, err
			}
			addToResult(skip)
		}
	}

	for li < len(left) {
		var err error
		lblock, err = apply(lblock, left, &li)
		if err != nil {
			return nil, err
		}
		if lblock != nil {
			if lblock.ID().Clock > nextExpected {
				skip, err := ytypes.NewSkip(ytypes.ID{Client: client, Clock: nextExpected}, lblock.ID().Clock-nextExpected)
				if err != nil {
					return nil, err
				}
				addToResult(skip)
			}
		}
	}

	for ri < len(right) {
		var err error
		rblock, err = apply(rblock, right, &ri)
		if err != nil {
			return nil, err
		}
		if rblock != nil {
			if rblock.ID().Clock > nextExpected {
				skip, err := ytypes.NewSkip(ytypes.ID{Client: client, Clock: nextExpected}, rblock.ID().Clock-nextExpected)
				if err != nil {
					return nil, err
				}
				addToResult(skip)
			}
		}
	}

	return result, nil
}

func sliceStructV1(current ytypes.Struct, diff uint32) (ytypes.Struct, error) {
	return sliceStructWindowV1(current, diff, 0)
}

func sliceStructWindowV1(current ytypes.Struct, startOffset, endTrim uint32) (ytypes.Struct, error) {
	length := current.Length()
	if uint64(startOffset)+uint64(endTrim) >= uint64(length) {
		return nil, ErrInvalidSliceOffset
	}
	if startOffset == 0 && endTrim == 0 {
		return current, nil
	}
	newLength := length - startOffset - endTrim
	if newLength == 0 {
		return nil, ErrInvalidSliceOffset
	}

	id, err := current.ID().Offset(startOffset)
	if err != nil {
		return nil, err
	}

	switch value := current.(type) {
	case ytypes.GC:
		return ytypes.NewGC(id, newLength)
	case ytypes.Skip:
		return ytypes.NewSkip(id, newLength)
	case *ytypes.Item:
		content, ok := value.Content.(ParsedContent)
		if !ok {
			return nil, fmt.Errorf("%w: %T", ErrUnsupportedContentSlice, value.Content)
		}
		nextContent, err := content.SliceWindow(startOffset, endTrim)
		if err != nil {
			return nil, err
		}
		opts := ytypes.ItemOptions{
			Origin:      originForSlice(value, startOffset),
			RightOrigin: value.RightOrigin,
			Parent:      value.Parent,
			ParentSub:   value.ParentSub,
			Redone:      value.Redone,
			Flags:       value.Info,
		}
		return ytypes.NewItem(id, nextContent, opts)
	default:
		return nil, fmt.Errorf("yupdate: struct nao suportada para slice: %T", current)
	}
}

func originForSlice(item *ytypes.Item, diff uint32) *ytypes.ID {
	if diff == 0 {
		return item.Origin
	}
	origin, err := item.ID().Offset(diff - 1)
	if err != nil {
		return nil
	}
	return &origin
}

func isSkip(current ytypes.Struct) bool {
	return current.Kind() == ytypes.KindSkip
}

func minClock(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}
