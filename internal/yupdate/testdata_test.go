package yupdate

import (
	"slices"

	"github.com/drksbr/yjs-crdt-golang-server/internal/varint"
	"github.com/drksbr/yjs-crdt-golang-server/internal/ytypes"
)

type structEncoding func(dst []byte) []byte

type clientBlock struct {
	client  uint32
	clock   uint32
	structs []structEncoding
}

type deleteRange struct {
	client uint32
	clock  uint32
	length uint32
}

type parentSpec struct {
	isRoot bool
	value  string
	id     *ytypes.ID
}

type itemWireOptions struct {
	origin      *ytypes.ID
	rightOrigin *ytypes.ID
	parent      parentSpec
	parentSub   string
}

type anyField struct {
	key   string
	value []byte
}

func rootParent(name string) parentSpec {
	return parentSpec{isRoot: true, value: name}
}

func idParent(client, clock uint32) parentSpec {
	id := ytypes.ID{Client: client, Clock: clock}
	return parentSpec{id: &id}
}

func idPtr(client, clock uint32) *ytypes.ID {
	id := ytypes.ID{Client: client, Clock: clock}
	return &id
}

func itemString(parent parentSpec, value string) structEncoding {
	return itemStringWithOptions(itemWireOptions{parent: parent}, value)
}

func itemStringWithOptions(opts itemWireOptions, value string) structEncoding {
	return func(dst []byte) []byte {
		return appendItemContent(dst, itemContentString, opts, func(dst []byte) []byte {
			return appendVarString(dst, value)
		})
	}
}

func itemDeleted(parent parentSpec, length uint32) structEncoding {
	return itemDeletedWithOptions(itemWireOptions{parent: parent}, length)
}

func itemDeletedWithOptions(opts itemWireOptions, length uint32) structEncoding {
	return func(dst []byte) []byte {
		return appendItemContent(dst, itemContentDeleted, opts, func(dst []byte) []byte {
			return varint.Append(dst, length)
		})
	}
}

func itemType(parent parentSpec, typeRef uint32, typeName string) structEncoding {
	return func(dst []byte) []byte {
		return appendItemContent(dst, itemContentType, itemWireOptions{parent: parent}, func(dst []byte) []byte {
			dst = varint.Append(dst, typeRef)
			switch typeRef {
			case typeRefYXmlElement, typeRefYXmlHook:
				dst = appendVarString(dst, typeName)
			}
			return dst
		})
	}
}

func itemDoc(parent parentSpec, guid string, opts []byte) structEncoding {
	return func(dst []byte) []byte {
		return appendItemContent(dst, itemContentDoc, itemWireOptions{parent: parent}, func(dst []byte) []byte {
			dst = appendVarString(dst, guid)
			return append(dst, opts...)
		})
	}
}

func itemJSON(parent parentSpec, values ...string) structEncoding {
	return itemJSONWithOptions(itemWireOptions{parent: parent}, values...)
}

func itemJSONWithOptions(opts itemWireOptions, values ...string) structEncoding {
	return func(dst []byte) []byte {
		return appendItemContent(dst, itemContentJSON, opts, func(dst []byte) []byte {
			dst = varint.Append(dst, uint32(len(values)))
			for _, value := range values {
				dst = appendVarString(dst, value)
			}
			return dst
		})
	}
}

func itemBinary(parent parentSpec, value []byte) structEncoding {
	return func(dst []byte) []byte {
		return appendItemContent(dst, itemContentBinary, itemWireOptions{parent: parent}, func(dst []byte) []byte {
			dst = varint.Append(dst, uint32(len(value)))
			return append(dst, value...)
		})
	}
}

func itemEmbed(parent parentSpec, value []byte) structEncoding {
	return func(dst []byte) []byte {
		return appendItemContent(dst, itemContentEmbed, itemWireOptions{parent: parent}, func(dst []byte) []byte {
			return append(dst, value...)
		})
	}
}

func itemFormat(parent parentSpec, key string, value []byte) structEncoding {
	return func(dst []byte) []byte {
		return appendItemContent(dst, itemContentFormat, itemWireOptions{parent: parent}, func(dst []byte) []byte {
			dst = appendVarString(dst, key)
			return append(dst, value...)
		})
	}
}

func itemAny(parent parentSpec, values ...[]byte) structEncoding {
	return itemAnyWithOptions(itemWireOptions{parent: parent}, values...)
}

func itemAnyWithOptions(opts itemWireOptions, values ...[]byte) structEncoding {
	return func(dst []byte) []byte {
		return appendItemContent(dst, itemContentAny, opts, func(dst []byte) []byte {
			dst = varint.Append(dst, uint32(len(values)))
			for _, value := range values {
				dst = append(dst, value...)
			}
			return dst
		})
	}
}

func gc(length uint32) structEncoding {
	return func(dst []byte) []byte {
		dst = append(dst, 0)
		return varint.Append(dst, length)
	}
}

func skip(length uint32) structEncoding {
	return func(dst []byte) []byte {
		dst = append(dst, 10)
		return varint.Append(dst, length)
	}
}

func buildUpdate(blocks ...any) []byte {
	clientBlocks := make([]clientBlock, 0)
	ds := ytypes.NewDeleteSet()

	for _, block := range blocks {
		switch value := block.(type) {
		case clientBlock:
			clientBlocks = append(clientBlocks, value)
		case deleteRange:
			_ = ds.Add(value.client, value.clock, value.length)
		}
	}

	out := varint.Append(nil, uint32(len(clientBlocks)))
	for _, block := range clientBlocks {
		out = varint.Append(out, uint32(len(block.structs)))
		out = varint.Append(out, block.client)
		out = varint.Append(out, block.clock)
		for _, encode := range block.structs {
			out = encode(out)
		}
	}
	return AppendDeleteSetBlockV1(out, ds)
}

func appendItemContent(dst []byte, contentRef byte, opts itemWireOptions, appendContent func([]byte) []byte) []byte {
	info := contentRef
	if opts.origin != nil {
		info |= ytypes.ItemHasOrigin
	}
	if opts.rightOrigin != nil {
		info |= ytypes.ItemHasRightOrigin
	}
	if opts.origin == nil && opts.rightOrigin == nil && opts.parentSub != "" {
		info |= ytypes.ItemHasParentSub
	}
	dst = append(dst, info)
	if opts.origin != nil {
		dst = appendID(dst, opts.origin.Client, opts.origin.Clock)
	}
	if opts.rightOrigin != nil {
		dst = appendID(dst, opts.rightOrigin.Client, opts.rightOrigin.Clock)
	}
	if opts.origin == nil && opts.rightOrigin == nil {
		dst = appendParent(dst, opts.parent)
		if opts.parentSub != "" {
			dst = appendVarString(dst, opts.parentSub)
		}
	}
	return appendContent(dst)
}

func appendParent(dst []byte, parent parentSpec) []byte {
	if parent.isRoot {
		dst = varint.Append(dst, 1)
		return appendVarString(dst, parent.value)
	}
	if parent.id != nil {
		dst = varint.Append(dst, 0)
		return appendID(dst, parent.id.Client, parent.id.Clock)
	}
	dst = varint.Append(dst, 0)
	return appendID(dst, 0, 0)
}

func appendVarString(dst []byte, value string) []byte {
	bytes := []byte(value)
	dst = varint.Append(dst, uint32(len(bytes)))
	return append(dst, bytes...)
}

func appendID(dst []byte, client, clock uint32) []byte {
	dst = varint.Append(dst, client)
	return varint.Append(dst, clock)
}

func appendAnyBool(dst []byte, value bool) []byte {
	if value {
		return append(dst, 120)
	}
	return append(dst, 121)
}

func appendAnyString(dst []byte, value string) []byte {
	dst = append(dst, 119)
	return appendVarString(dst, value)
}

func appendAnyObject(dst []byte, entries map[string][]byte) []byte {
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	dst = append(dst, 118)
	dst = varint.Append(dst, uint32(len(keys)))
	for _, key := range keys {
		dst = appendVarString(dst, key)
		dst = append(dst, entries[key]...)
	}
	return dst
}

func appendAnyObjectFields(dst []byte, fields ...anyField) []byte {
	dst = append(dst, 118)
	dst = varint.Append(dst, uint32(len(fields)))
	for _, field := range fields {
		dst = appendVarString(dst, field.key)
		dst = append(dst, field.value...)
	}
	return dst
}

func decodeStateVector(data []byte) (map[uint32]uint32, error) {
	count, n, err := varint.Decode(data)
	if err != nil {
		return nil, err
	}

	out := make(map[uint32]uint32, count)
	rest := data[n:]
	for i := uint32(0); i < count; i++ {
		client, cn, err := varint.Decode(rest)
		if err != nil {
			return nil, err
		}
		clock, kn, err := varint.Decode(rest[cn:])
		if err != nil {
			return nil, err
		}
		out[client] = clock
		rest = rest[cn+kn:]
	}
	return out, nil
}
