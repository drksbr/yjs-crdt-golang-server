package yupdate

import (
	"yjs-go-bridge/internal/yidset"
	"yjs-go-bridge/internal/ytypes"
)

// IntersectUpdateWithContentIDsV1 filtra um update V1 mantendo apenas o conteúdo
// mencionado pelo padrão de content ids.
func IntersectUpdateWithContentIDsV1(update []byte, contentIDs *ContentIDs) ([]byte, error) {
	if contentIDs == nil {
		contentIDs = NewContentIDs()
	}

	reader, err := NewLazyReaderV1(update, true)
	if err != nil {
		return nil, err
	}

	writer := newLazyWriterV1()
	for current := reader.Current(); current != nil; {
		client := current.ID().Client
		nextClock := current.ID().Clock
		firstWrite := false
		ranges := contentIDs.Inserts.Ranges(client)
		rangeIdx := 0

		for current != nil && current.ID().Client == client {
			currentClock := current.ID().Clock
			currentEndClock := current.EndClock()

			for rangeIdx < len(ranges) && ranges[rangeIdx].End() <= uint64(currentClock) {
				rangeIdx++
			}

			for rangeIdx < len(ranges) {
				currentRange := ranges[rangeIdx]
				if uint64(currentRange.Clock) >= uint64(currentEndClock) {
					break
				}

				segmentStart := uint64(currentRange.Clock)
				if segmentStart < uint64(currentClock) {
					segmentStart = uint64(currentClock)
				}
				segmentEnd := currentRange.End()
				if segmentEnd > uint64(currentEndClock) {
					segmentEnd = uint64(currentEndClock)
				}
				if segmentStart >= uint64(currentEndClock) || segmentStart >= segmentEnd {
					break
				}

				if firstWrite && segmentStart > uint64(nextClock) {
					skipLen := uint32(segmentStart - uint64(nextClock))
					skip, err := ytypes.NewSkip(ytypes.ID{Client: client, Clock: nextClock}, skipLen)
					if err != nil {
						return nil, err
					}
					if err := writer.write(skip, 0, 0); err != nil {
						return nil, err
					}
				}

				startOffset := uint32(segmentStart - uint64(currentClock))
				endTrim := uint32(uint64(currentEndClock) - segmentEnd)
				if err := writer.write(current, uint32(startOffset), endTrim); err != nil {
					return nil, err
				}
				nextClock = uint32(segmentEnd)
				firstWrite = true

				if currentRange.End() <= uint64(currentEndClock) {
					rangeIdx++
				}
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

	filteredDeleteSet := intersectDeleteSetWithIDs(deleteSet, contentIDs.Deletes)
	out, err := writer.finish(nil)
	if err != nil {
		return nil, err
	}
	return AppendDeleteSetBlockV1(out, filteredDeleteSet), nil
}

func intersectDeleteSetWithIDs(ds *ytypes.DeleteSet, ids *yidset.IdSet) *ytypes.DeleteSet {
	result := ytypes.NewDeleteSet()
	if ds == nil || ids == nil {
		return result
	}

	deleteIDs := yidset.New()
	for _, client := range ds.Clients() {
		for _, r := range ds.Ranges(client) {
			_ = deleteIDs.Add(client, r.Clock, r.Length)
		}
	}

	intersection := yidset.IntersectSets(deleteIDs, ids)
	for _, client := range intersection.Clients() {
		for _, r := range intersection.Ranges(client) {
			_ = result.Add(client, r.Clock, r.Length)
		}
	}
	return result
}
