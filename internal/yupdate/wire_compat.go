package yupdate

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"

	ybinary "github.com/drksbr/yjs-crdt-golang-server/internal/binary"
	"github.com/drksbr/yjs-crdt-golang-server/internal/varint"
	"github.com/drksbr/yjs-crdt-golang-server/internal/ytypes"
)

// ConvertUpdateToV1YjsWire converte qualquer update suportado para V1 no wire
// compatível com clientes Yjs (ContentEmbed/ContentFormat em JSON-string no V1).
func ConvertUpdateToV1YjsWire(update []byte) ([]byte, error) {
	decoded, err := DecodeUpdate(update)
	if err != nil {
		return nil, err
	}

	if err := rewriteEmbedAndFormatToV1JSONWire(decoded); err != nil {
		return nil, err
	}

	return EncodeV1(decoded)
}

// ConvertUpdateToV2YjsWire converte qualquer update suportado para V2 no wire
// compatível com clientes Yjs (ContentEmbed/ContentFormat em lib0-any no V2).
func ConvertUpdateToV2YjsWire(update []byte) ([]byte, error) {
	decoded, err := DecodeUpdate(update)
	if err != nil {
		return nil, err
	}

	if err := rewriteEmbedAndFormatToV2AnyWire(decoded); err != nil {
		return nil, err
	}

	return EncodeV2(decoded)
}

func rewriteEmbedAndFormatToV2AnyWire(decoded *DecodedUpdate) error {
	if decoded == nil {
		return nil
	}

	for _, current := range decoded.Structs {
		item, ok := current.(*ytypes.Item)
		if !ok {
			continue
		}

		content, ok := item.Content.(ParsedContent)
		if !ok {
			continue
		}

		switch content.Ref {
		case itemContentEmbed:
			anyRaw, err := anyOrJSONRawToAnyRaw(content.Raw)
			if err != nil {
				return err
			}
			content.Raw = anyRaw
			item.Content = content
		case itemContentFormat:
			keyRaw, valueRaw, err := splitV1StringPrefix(content.Raw)
			if err != nil {
				return err
			}
			anyRaw, err := anyOrJSONRawToAnyRaw(valueRaw)
			if err != nil {
				return err
			}
			content.Raw = append(append([]byte{}, keyRaw...), anyRaw...)
			item.Content = content
		}
	}
	return nil
}

func rewriteEmbedAndFormatToV1JSONWire(decoded *DecodedUpdate) error {
	if decoded == nil {
		return nil
	}

	for _, current := range decoded.Structs {
		item, ok := current.(*ytypes.Item)
		if !ok {
			continue
		}

		content, ok := item.Content.(ParsedContent)
		if !ok {
			continue
		}

		switch content.Ref {
		case itemContentEmbed:
			jsonRaw, err := anyOrJSONRawToV1JSONRaw(content.Raw)
			if err != nil {
				return err
			}
			content.Raw = jsonRaw
			item.Content = content
		case itemContentFormat:
			keyRaw, valueRaw, err := splitV1StringPrefix(content.Raw)
			if err != nil {
				return err
			}
			jsonRaw, err := anyOrJSONRawToV1JSONRaw(valueRaw)
			if err != nil {
				return err
			}
			content.Raw = append(append([]byte{}, keyRaw...), jsonRaw...)
			item.Content = content
		}
	}
	return nil
}

func anyOrJSONRawToAnyRaw(raw []byte) ([]byte, error) {
	if _, err := decodeLib0AnyRawExact(raw); err == nil {
		return raw, nil
	}

	value, err := decodeV1JSONRaw(raw)
	if err != nil {
		return nil, err
	}
	return encodeLib0AnyRaw(value)
}

func anyOrJSONRawToV1JSONRaw(raw []byte) ([]byte, error) {
	if _, err := decodeV1JSONRaw(raw); err == nil {
		return raw, nil
	}

	value, err := decodeLib0AnyRawExact(raw)
	if err != nil {
		return nil, err
	}
	return encodeV1JSONRaw(value)
}

func splitV1StringPrefix(raw []byte) (prefix []byte, rest []byte, err error) {
	if len(raw) == 0 {
		return nil, nil, ErrTrailingBytes
	}
	length, n, err := varint.Decode(raw)
	if err != nil {
		return nil, nil, err
	}
	end := n + int(length)
	if end > len(raw) {
		return nil, nil, varint.ErrUnexpectedEOF
	}
	return raw[:end], raw[end:], nil
}

func decodeV1JSONRaw(raw []byte) (any, error) {
	if len(raw) == 0 {
		return nil, ErrTrailingBytes
	}
	length, n, err := varint.Decode(raw)
	if err != nil {
		return nil, err
	}
	end := n + int(length)
	if end > len(raw) {
		return nil, varint.ErrUnexpectedEOF
	}
	if end != len(raw) {
		return nil, ErrTrailingBytes
	}

	payload := string(raw[n:end])
	if payload == "undefined" {
		return nil, nil
	}

	dec := json.NewDecoder(strings.NewReader(payload))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return nil, err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return nil, ErrTrailingBytes
	}
	return value, nil
}

func encodeV1JSONRaw(value any) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return appendVarStringV1(nil, string(data)), nil
}

func decodeLib0AnyRawExact(raw []byte) (any, error) {
	reader := ybinary.NewReader(raw)
	value, err := decodeLib0AnyValue(reader, "ReadAny")
	if err != nil {
		return nil, err
	}
	if reader.Remaining() != 0 {
		return nil, ErrTrailingBytes
	}
	return value, nil
}

func decodeLib0AnyValue(reader *ybinary.Reader, op string) (any, error) {
	start := reader.Offset()
	tag, err := reader.ReadByte()
	if err != nil {
		return nil, wrapError(op+".tag", start, err)
	}

	switch tag {
	case 127:
		return nil, nil
	case 126:
		return nil, nil
	case 125:
		value, _, err := readLib0VarInt(reader, op+".int")
		return value, err
	case 124:
		data, err := readLib0BigEndian(reader, op+".float32", 4)
		if err != nil {
			return nil, err
		}
		return float64(math.Float32frombits(binary.BigEndian.Uint32(data))), nil
	case 123:
		data, err := readLib0BigEndian(reader, op+".float64", 8)
		if err != nil {
			return nil, err
		}
		return math.Float64frombits(binary.BigEndian.Uint64(data)), nil
	case 122:
		data, err := readLib0BigEndian(reader, op+".bigint", 8)
		if err != nil {
			return nil, err
		}
		return int64(binary.BigEndian.Uint64(data)), nil
	case 121:
		return false, nil
	case 120:
		return true, nil
	case 119:
		value, err := readLib0VarString(reader, op+".string")
		return value, err
	case 118:
		length, err := readLib0VarUint(reader, op+".object.len")
		if err != nil {
			return nil, err
		}
		obj := make(map[string]any, length)
		for i := uint32(0); i < length; i++ {
			key, err := readLib0VarString(reader, op+".object.key")
			if err != nil {
				return nil, err
			}
			value, err := decodeLib0AnyValue(reader, op+".object.value")
			if err != nil {
				return nil, err
			}
			obj[key] = value
		}
		return obj, nil
	case 117:
		length, err := readLib0VarUint(reader, op+".array.len")
		if err != nil {
			return nil, err
		}
		arr := make([]any, 0, length)
		for i := uint32(0); i < length; i++ {
			value, err := decodeLib0AnyValue(reader, op+".array.value")
			if err != nil {
				return nil, err
			}
			arr = append(arr, value)
		}
		return arr, nil
	case 116:
		return readLib0VarUint8Array(reader, op+".uint8")
	default:
		return nil, fmt.Errorf("tag any desconhecida: %d", tag)
	}
}

func encodeLib0AnyRaw(value any) ([]byte, error) {
	return appendLib0Any(nil, value)
}

func appendLib0Any(dst []byte, value any) ([]byte, error) {
	switch v := value.(type) {
	case nil:
		return append(dst, 126), nil
	case bool:
		if v {
			return append(dst, 120), nil
		}
		return append(dst, 121), nil
	case string:
		dst = append(dst, 119)
		return appendLib0VarString(dst, v), nil
	case json.Number:
		if intVal, err := v.Int64(); err == nil && canEncodeAsLib0Int(intVal) {
			dst = append(dst, 125)
			return appendLib0VarInt(dst, intVal), nil
		}
		floatVal, err := v.Float64()
		if err != nil {
			return nil, err
		}
		return appendLib0Number(dst, floatVal)
	case int:
		return appendLib0Int(dst, int64(v)), nil
	case int8:
		return appendLib0Int(dst, int64(v)), nil
	case int16:
		return appendLib0Int(dst, int64(v)), nil
	case int32:
		return appendLib0Int(dst, int64(v)), nil
	case int64:
		return appendLib0Int(dst, v), nil
	case uint:
		return appendLib0Int(dst, int64(v)), nil
	case uint8:
		return appendLib0Int(dst, int64(v)), nil
	case uint16:
		return appendLib0Int(dst, int64(v)), nil
	case uint32:
		return appendLib0Int(dst, int64(v)), nil
	case uint64:
		if v <= math.MaxInt64 {
			return appendLib0Int(dst, int64(v)), nil
		}
		return appendLib0Number(dst, float64(v))
	case float32:
		return appendLib0Number(dst, float64(v))
	case float64:
		return appendLib0Number(dst, v)
	case []byte:
		dst = append(dst, 116)
		return appendLib0VarUint8Array(dst, v), nil
	case []any:
		dst = append(dst, 117)
		dst = appendVarUintV1(dst, uint32(len(v)))
		var err error
		for _, entry := range v {
			dst, err = appendLib0Any(dst, entry)
			if err != nil {
				return nil, err
			}
		}
		return dst, nil
	case map[string]any:
		return appendLib0Object(dst, v)
	default:
		return nil, fmt.Errorf("tipo any nao suportado: %T", value)
	}
}

func appendLib0Int(dst []byte, value int64) []byte {
	if canEncodeAsLib0Int(value) {
		dst = append(dst, 125)
		return appendLib0VarInt(dst, value)
	}
	result, _ := appendLib0Number(dst, float64(value))
	return result
}

func appendLib0Number(dst []byte, value float64) ([]byte, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return nil, fmt.Errorf("numero invalido: %v", value)
	}
	if math.Trunc(value) == value && canEncodeAsLib0Int(int64(value)) {
		dst = append(dst, 125)
		return appendLib0VarInt(dst, int64(value)), nil
	}
	if float64(float32(value)) == value {
		dst = append(dst, 124)
		buf := make([]byte, 4)
		binary.BigEndian.PutUint32(buf, math.Float32bits(float32(value)))
		return append(dst, buf...), nil
	}
	dst = append(dst, 123)
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, math.Float64bits(value))
	return append(dst, buf...), nil
}

func appendLib0Object(dst []byte, value map[string]any) ([]byte, error) {
	dst = append(dst, 118)
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	dst = appendVarUintV1(dst, uint32(len(keys)))
	var err error
	for _, key := range keys {
		dst = appendLib0VarString(dst, key)
		dst, err = appendLib0Any(dst, value[key])
		if err != nil {
			return nil, err
		}
	}
	return dst, nil
}

func canEncodeAsLib0Int(value int64) bool {
	const bits31 = 1 << 31
	return value >= -bits31 && value <= bits31
}
