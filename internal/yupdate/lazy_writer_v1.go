package yupdate

import (
	"bytes"

	"yjs-go-bridge/internal/ytypes"
)

type lazyWriterFragmentV1 struct {
	written uint32
	payload []byte
}

type lazyWriterV1 struct {
	currClient uint32
	written    uint32
	current    []byte
	fragments  []lazyWriterFragmentV1
	lastClient uint32
	hasLast    bool
}

func newLazyWriterV1() *lazyWriterV1 {
	return &lazyWriterV1{}
}

func (w *lazyWriterV1) write(current ytypes.Struct, startOffset, endTrim uint32) error {
	if w.written > 0 && w.currClient != current.ID().Client {
		w.flush()
	}
	if w.written == 0 {
		if w.hasLast && current.ID().Client >= w.lastClient {
			return ErrInvalidClientOrder
		}
		w.currClient = current.ID().Client
		w.current = appendVarUintV1(w.current[:0], w.currClient)
		w.current = appendVarUintV1(w.current, current.ID().Clock+startOffset)
	}

	var err error
	w.current, err = appendStructRangeV1(w.current, current, startOffset, endTrim)
	if err != nil {
		return err
	}
	w.written++
	return nil
}

func (w *lazyWriterV1) finish(dst []byte) ([]byte, error) {
	w.flush()

	dst = appendVarUintV1(dst, uint32(len(w.fragments)))
	for _, fragment := range w.fragments {
		dst = appendVarUintV1(dst, fragment.written)
		dst = append(dst, fragment.payload...)
	}
	return dst, nil
}

func (w *lazyWriterV1) flush() {
	if w.written == 0 {
		return
	}
	w.fragments = append(w.fragments, lazyWriterFragmentV1{
		written: w.written,
		payload: bytes.Clone(w.current),
	})
	w.lastClient = w.currClient
	w.hasLast = true
	w.current = w.current[:0]
	w.written = 0
}

func appendStructRangeV1(dst []byte, current ytypes.Struct, startOffset, endTrim uint32) ([]byte, error) {
	if startOffset == 0 && endTrim == 0 {
		return appendStructV1(dst, current)
	}

	sliced, err := sliceStructWindowV1(current, startOffset, endTrim)
	if err != nil {
		return nil, err
	}
	return appendStructV1(dst, sliced)
}
