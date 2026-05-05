package yupdate

import (
	"fmt"

	"github.com/drksbr/yjs-crdt-golang-server/internal/varint"
)

func (d *decoderV1) readVarIntRaw(op string) ([]byte, error) {
	raw := make([]byte, 0, 6)
	for i := 0; i < 6; i++ {
		b, err := d.reader.ReadByte()
		if err != nil {
			return nil, wrapError(op, d.offset(), err)
		}
		raw = append(raw, b)
		if b < 0x80 {
			return raw, nil
		}
	}
	return nil, wrapError(op, d.offset(), fmt.Errorf("varint excedeu 6 bytes"))
}

func (d *decoderV1) readAnyRaw(op string) ([]byte, error) {
	start := d.offset()
	tag, err := d.reader.ReadByte()
	if err != nil {
		return nil, wrapError(op+".tag", start, err)
	}

	return d.readAnyRawWithTag(op, start, tag)
}

func (d *decoderV1) readAnyRawWithTag(op string, start int, tag byte) ([]byte, error) {
	if !isLib0AnyTag(tag) {
		return nil, wrapError(op, start, fmt.Errorf("tag any desconhecida: %d", tag))
	}

	raw := []byte{tag}
	switch tag {
	case 127, 126, 121, 120:
		return raw, nil
	case 125:
		value, err := d.readVarIntRaw(op + ".varint")
		if err != nil {
			return nil, err
		}
		return append(raw, value...), nil
	case 124:
		value, err := d.reader.ReadN(4)
		if err != nil {
			return nil, wrapError(op+".float32", d.offset(), err)
		}
		return append(raw, value...), nil
	case 123, 122:
		value, err := d.reader.ReadN(8)
		if err != nil {
			return nil, wrapError(op+".float64", d.offset(), err)
		}
		return append(raw, value...), nil
	case 119:
		value, err := d.readString(op + ".string")
		if err != nil {
			return nil, err
		}
		return appendVarStringV1(raw, value), nil
	case 118:
		length, err := d.readVarUint(op + ".object.len")
		if err != nil {
			return nil, err
		}
		raw = appendVarUintV1(raw, length)
		for i := uint32(0); i < length; i++ {
			key, err := d.readString(op + ".object.key")
			if err != nil {
				return nil, err
			}
			raw = appendVarStringV1(raw, key)
			value, err := d.readAnyRaw(op + ".object.value")
			if err != nil {
				return nil, err
			}
			raw = append(raw, value...)
		}
		return raw, nil
	case 117:
		length, err := d.readVarUint(op + ".array.len")
		if err != nil {
			return nil, err
		}
		raw = appendVarUintV1(raw, length)
		for i := uint32(0); i < length; i++ {
			value, err := d.readAnyRaw(op + ".array.value")
			if err != nil {
				return nil, err
			}
			raw = append(raw, value...)
		}
		return raw, nil
	case 116:
		value, err := d.readBuf(op + ".buf")
		if err != nil {
			return nil, err
		}
		raw = appendVarUintV1(raw, uint32(len(value)))
		return append(raw, value...), nil
	default:
		return nil, wrapError(op, start, fmt.Errorf("tag any desconhecida: %d", tag))
	}
}

// readJSONRawCompat lê valores serializados via writeJSON do Yjs V1:
// - representação oficial V1: varString(JSON.stringify(value))
// - representação legada deste projeto: lib0 any raw
//
// Para manter compatibilidade com payloads antigos, aceitamos ambas.
func (d *decoderV1) readJSONRawCompat(op string) ([]byte, error) {
	start := d.offset()
	first, err := d.reader.ReadByte()
	if err != nil {
		return nil, wrapError(op+".tag", start, err)
	}

	if isLib0AnyTag(first) {
		return d.readAnyRawWithTag(op+".any", start, first)
	}
	return d.readVarStringRawWithFirst(op+".json", start, first)
}

func (d *decoderV1) readVarStringRawWithFirst(op string, start int, first byte) ([]byte, error) {
	rawLen := []byte{first}
	value := uint32(first & 0x7f)
	shift := uint(7)
	i := 0
	last := first

	for last&0x80 != 0 {
		i++
		if i >= 5 {
			return nil, wrapError(op+".len", start, varint.ErrOverflow)
		}
		next, err := d.reader.ReadByte()
		if err != nil {
			return nil, wrapError(op+".len", start, varint.ErrUnexpectedEOF)
		}
		rawLen = append(rawLen, next)
		last = next

		if i == 4 {
			if next&0x80 != 0 || next > 0x0f {
				return nil, wrapError(op+".len", start, varint.ErrOverflow)
			}
		}
		value |= uint32(next&0x7f) << shift
		shift += 7
	}

	if encodedVarUintLen(value) != len(rawLen) {
		return nil, wrapError(op+".len", start, varint.ErrNonCanonical)
	}

	payload, err := d.reader.ReadN(int(value))
	if err != nil {
		return nil, wrapError(op, d.offset(), err)
	}
	raw := append([]byte{}, rawLen...)
	raw = append(raw, payload...)
	return raw, nil
}

func isLib0AnyTag(tag byte) bool {
	return tag >= 116 && tag <= 127
}

func encodedVarUintLen(value uint32) int {
	n := 1
	for value >= 0x80 {
		value >>= 7
		n++
	}
	return n
}
