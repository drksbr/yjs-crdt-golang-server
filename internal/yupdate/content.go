package yupdate

import (
	"fmt"

	"yjs-go-bridge/internal/ytypes"
)

const (
	itemContentDeleted byte = 1
	itemContentJSON    byte = 2
	itemContentBinary  byte = 3
	itemContentString  byte = 4
	itemContentEmbed   byte = 5
	itemContentFormat  byte = 6
	itemContentType    byte = 7
	itemContentAny     byte = 8
	itemContentDoc     byte = 9
)

const (
	typeRefYArray uint32 = iota
	typeRefYMap
	typeRefYText
	typeRefYXmlElement
	typeRefYXmlFragment
	typeRefYXmlHook
	typeRefYXmlText
)

// ParsedContent é a menor representação de conteúdo necessária para parsing de update.
// Ela preserva o ref, o comprimento estrutural e alguns metadados usados por testes
// e futuras funções derivadas como state vector/content ids.
type ParsedContent struct {
	Ref       byte
	LengthVal uint32
	Countable bool
	TypeRef   uint32
	TypeName  string
	Raw       []byte
	Text      string
	JSON      []string
	Any       [][]byte
}

func (c ParsedContent) Length() uint32       { return c.LengthVal }
func (c ParsedContent) IsCountable() bool    { return c.Countable }
func (c ParsedContent) ContentRef() byte     { return c.Ref }
func (c ParsedContent) EmbeddedType() uint32 { return c.TypeRef }

func readItemContentV1(d *decoderV1, info byte) (ParsedContent, error) {
	switch info & ytypes.ItemContentRefMask {
	case itemContentDeleted:
		length, err := d.readVarUint("ReadContentDeleted")
		return ParsedContent{
			Ref:       itemContentDeleted,
			LengthVal: length,
			Countable: false,
			Raw:       appendVarUintV1(nil, length),
		}, err
	case itemContentJSON:
		length, err := d.readVarUint("ReadContentJSON.len")
		if err != nil {
			return ParsedContent{}, err
		}
		values := make([]string, 0, length)
		raw := appendVarUintV1(nil, length)
		for i := uint32(0); i < length; i++ {
			value, err := d.readString("ReadContentJSON.value")
			if err != nil {
				return ParsedContent{}, err
			}
			values = append(values, value)
			raw = appendVarStringV1(raw, value)
		}
		return ParsedContent{Ref: itemContentJSON, LengthVal: length, Countable: true, Raw: raw, JSON: values}, nil
	case itemContentBinary:
		buf, err := d.readBuf("ReadContentBinary")
		if err != nil {
			return ParsedContent{}, err
		}
		raw := appendVarUintV1(nil, uint32(len(buf)))
		raw = append(raw, buf...)
		return ParsedContent{Ref: itemContentBinary, LengthVal: 1, Countable: true, Raw: raw}, nil
	case itemContentString:
		value, err := d.readString("ReadContentString")
		if err != nil {
			return ParsedContent{}, err
		}
		return ParsedContent{
			Ref:       itemContentString,
			LengthVal: utf16Length(value),
			Countable: true,
			Raw:       appendVarStringV1(nil, value),
			Text:      value,
		}, nil
	case itemContentEmbed:
		raw, err := d.readAnyRaw("ReadContentEmbed")
		if err != nil {
			return ParsedContent{}, err
		}
		return ParsedContent{Ref: itemContentEmbed, LengthVal: 1, Countable: true, Raw: raw}, nil
	case itemContentFormat:
		key, err := d.readString("ReadContentFormat.key")
		if err != nil {
			return ParsedContent{}, err
		}
		valueRaw, err := d.readAnyRaw("ReadContentFormat.value")
		if err != nil {
			return ParsedContent{}, err
		}
		raw := appendVarStringV1(nil, key)
		raw = append(raw, valueRaw...)
		return ParsedContent{Ref: itemContentFormat, LengthVal: 1, Countable: false, TypeName: key, Raw: raw}, nil
	case itemContentType:
		typeRef, err := d.readVarUint("ReadContentType.ref")
		if err != nil {
			return ParsedContent{}, err
		}
		typeName, err := readTypePayloadV1(d, typeRef)
		if err != nil {
			return ParsedContent{}, err
		}
		raw := appendVarUintV1(nil, typeRef)
		switch typeRef {
		case typeRefYXmlElement, typeRefYXmlHook:
			raw = appendVarStringV1(raw, typeName)
		}
		return ParsedContent{Ref: itemContentType, LengthVal: 1, Countable: true, TypeRef: typeRef, TypeName: typeName, Raw: raw}, nil
	case itemContentAny:
		length, err := d.readVarUint("ReadContentAny.len")
		if err != nil {
			return ParsedContent{}, err
		}
		values := make([][]byte, 0, length)
		raw := appendVarUintV1(nil, length)
		for i := uint32(0); i < length; i++ {
			valueRaw, err := d.readAnyRaw("ReadContentAny.value")
			if err != nil {
				return ParsedContent{}, err
			}
			values = append(values, valueRaw)
			raw = append(raw, valueRaw...)
		}
		return ParsedContent{Ref: itemContentAny, LengthVal: length, Countable: true, Raw: raw, Any: values}, nil
	case itemContentDoc:
		name, err := d.readString("ReadContentDoc.guid")
		if err != nil {
			return ParsedContent{}, err
		}
		optsRaw, err := d.readAnyRaw("ReadContentDoc.opts")
		if err != nil {
			return ParsedContent{}, err
		}
		raw := appendVarStringV1(nil, name)
		raw = append(raw, optsRaw...)
		return ParsedContent{Ref: itemContentDoc, LengthVal: 1, Countable: true, TypeName: name, Raw: raw}, nil
	default:
		return ParsedContent{}, wrapError("ReadItemContent", d.offset(), fmt.Errorf("%w: %d", ErrUnknownContentRef, info&ytypes.ItemContentRefMask))
	}
}

func readTypePayloadV1(d *decoderV1, typeRef uint32) (string, error) {
	switch typeRef {
	case typeRefYArray, typeRefYMap, typeRefYText, typeRefYXmlFragment, typeRefYXmlText:
		return "", nil
	case typeRefYXmlElement, typeRefYXmlHook:
		return d.readString("ReadTypePayload.key")
	default:
		return "", wrapError("ReadTypePayload", d.offset(), fmt.Errorf("%w: %d", ErrUnknownTypeRef, typeRef))
	}
}
