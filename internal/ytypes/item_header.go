package ytypes

import "errors"

const (
	// ItemContentRefMask preserva os 5 bits baixos do header wire do Item.
	ItemContentRefMask byte = 0x1f
	ItemHasParentSub   byte = 0x20
	ItemHasRightOrigin byte = 0x40
	ItemHasOrigin      byte = 0x80
)

var errInvalidContentRef = errors.New("ytypes: content ref invalido")

// ItemHeader descreve o byte de header do Item no formato de update do Yjs.
// Ele é diferente das flags internas mantidas em Item.Info.
type ItemHeader struct {
	ContentRef     byte
	HasOrigin      bool
	HasRightOrigin bool
	HasParentSub   bool
}

// Encode serializa o header no mesmo layout bit a bit usado pelo Yjs.
func (h ItemHeader) Encode() (byte, error) {
	if h.ContentRef > ItemContentRefMask {
		return 0, errInvalidContentRef
	}

	info := h.ContentRef & ItemContentRefMask
	if h.HasOrigin {
		info |= ItemHasOrigin
	}
	if h.HasRightOrigin {
		info |= ItemHasRightOrigin
	}
	if h.HasParentSub {
		info |= ItemHasParentSub
	}
	return info, nil
}

// DecodeItemHeader interpreta o byte bruto vindo do update.
func DecodeItemHeader(info byte) ItemHeader {
	return ItemHeader{
		ContentRef:     info & ItemContentRefMask,
		HasOrigin:      info&ItemHasOrigin != 0,
		HasRightOrigin: info&ItemHasRightOrigin != 0,
		HasParentSub:   info&ItemHasParentSub != 0,
	}
}
