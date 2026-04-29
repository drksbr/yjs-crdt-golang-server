package yupdate

import (
	"fmt"
	"unicode/utf16"

	ybinary "github.com/drksbr/yjs-crdt-golang-server/internal/binary"
	"github.com/drksbr/yjs-crdt-golang-server/internal/varint"
	"github.com/drksbr/yjs-crdt-golang-server/internal/ytypes"
)

type decoderV1 struct {
	reader *ybinary.Reader
}

func newDecoderV1(data []byte) *decoderV1 {
	return &decoderV1{reader: ybinary.NewReader(data)}
}

func (d *decoderV1) offset() int {
	return d.reader.Offset()
}

func (d *decoderV1) remaining() int {
	return d.reader.Remaining()
}

func (d *decoderV1) readVarUint(op string) (uint32, error) {
	value, _, err := varint.Read(d.reader)
	if err != nil {
		return 0, wrapError(op, d.offset(), err)
	}
	return value, nil
}

func (d *decoderV1) readInfo() (byte, error) {
	value, err := d.reader.ReadByte()
	if err != nil {
		return 0, wrapError("ReadInfo", d.offset(), err)
	}
	return value, nil
}

func (d *decoderV1) readID(op string) (ytypes.ID, error) {
	id, _, err := ytypes.ReadID(d.reader)
	if err != nil {
		return ytypes.ID{}, wrapError(op, d.offset(), err)
	}
	return id, nil
}

func (d *decoderV1) readString(op string) (string, error) {
	length, err := d.readVarUint(op + ".len")
	if err != nil {
		return "", err
	}

	data, err := d.reader.ReadN(int(length))
	if err != nil {
		return "", wrapError(op, d.offset(), err)
	}
	return string(data), nil
}

func (d *decoderV1) readBuf(op string) ([]byte, error) {
	length, err := d.readVarUint(op + ".len")
	if err != nil {
		return nil, err
	}

	buf, err := d.reader.ReadN(int(length))
	if err != nil {
		return nil, wrapError(op, d.offset(), err)
	}
	copied := make([]byte, len(buf))
	copy(copied, buf)
	return copied, nil
}

func (d *decoderV1) readParentInfo() (bool, error) {
	value, err := d.readVarUint("ReadParentInfo")
	if err != nil {
		return false, err
	}
	switch value {
	case 0:
		return false, nil
	case 1:
		return true, nil
	default:
		return false, wrapError("ReadParentInfo.value", d.offset(), fmt.Errorf("parent info invalido: %d", value))
	}
}

func (d *decoderV1) skipJSON(op string) error {
	_, err := d.readString(op)
	return err
}

func (d *decoderV1) skipAny(op string) error {
	tag, err := d.readInfo()
	if err != nil {
		return wrapError(op+".tag", d.offset(), err)
	}

	switch tag {
	case 127, 126, 121, 120:
		return nil
	case 125:
		return d.skipVarInt(op + ".varint")
	case 124:
		_, err := d.reader.ReadN(4)
		return wrapError(op+".float32", d.offset(), err)
	case 123, 122:
		_, err := d.reader.ReadN(8)
		return wrapError(op+".float64", d.offset(), err)
	case 119:
		_, err := d.readString(op + ".string")
		return err
	case 118:
		length, err := d.readVarUint(op + ".object.len")
		if err != nil {
			return err
		}
		for i := uint32(0); i < length; i++ {
			if _, err := d.readString(op + ".object.key"); err != nil {
				return err
			}
			if err := d.skipAny(op + ".object.value"); err != nil {
				return err
			}
		}
		return nil
	case 117:
		length, err := d.readVarUint(op + ".array.len")
		if err != nil {
			return err
		}
		for i := uint32(0); i < length; i++ {
			if err := d.skipAny(op + ".array.value"); err != nil {
				return err
			}
		}
		return nil
	case 116:
		_, err := d.readBuf(op + ".buf")
		return err
	default:
		return wrapError(op, d.offset(), fmt.Errorf("tag any desconhecida: %d", tag))
	}
}

func (d *decoderV1) skipVarInt(op string) error {
	for i := 0; i < 6; i++ {
		b, err := d.reader.ReadByte()
		if err != nil {
			return wrapError(op, d.offset(), err)
		}
		if b < 0x80 {
			return nil
		}
	}
	return wrapError(op, d.offset(), fmt.Errorf("varint excedeu 6 bytes"))
}

func utf16Length(s string) uint32 {
	return uint32(len(utf16.Encode([]rune(s))))
}
