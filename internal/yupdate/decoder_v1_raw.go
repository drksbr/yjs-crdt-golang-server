package yupdate

import "fmt"

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
