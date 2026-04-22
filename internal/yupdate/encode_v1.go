package yupdate

import (
	"fmt"
	"slices"

	"yjs-go-bridge/internal/varint"
	"yjs-go-bridge/internal/ytypes"
)

// EncodeV1 serializa novamente um update V1 já materializado.
func EncodeV1(decoded *DecodedUpdate) ([]byte, error) {
	if decoded == nil {
		return encodeEmptyUpdateV1(), nil
	}

	clients := groupStructsByClient(decoded.Structs)
	out, err := encodeStructGroupsV1(clients)
	if err != nil {
		return nil, err
	}
	return AppendDeleteSetBlockV1(out, decoded.DeleteSet), nil
}

func encodeStructGroupsV1(groups map[uint32][]ytypes.Struct) ([]byte, error) {
	clients := make([]uint32, 0, len(groups))
	for client := range groups {
		clients = append(clients, client)
	}
	slices.SortFunc(clients, func(a, b uint32) int {
		switch {
		case a > b:
			return -1
		case a < b:
			return 1
		default:
			return 0
		}
	})

	writer := newLazyWriterV1()
	for _, client := range clients {
		structs := groups[client]
		if len(structs) == 0 {
			continue
		}
		for _, current := range structs {
			if err := writer.write(current, 0, 0); err != nil {
				return nil, err
			}
		}
	}
	return writer.finish(nil)
}

func appendStructV1(dst []byte, current ytypes.Struct) ([]byte, error) {
	switch value := current.(type) {
	case ytypes.GC:
		dst = append(dst, 0)
		return appendVarUintV1(dst, value.Length()), nil
	case ytypes.Skip:
		dst = append(dst, 10)
		return appendVarUintV1(dst, value.Length()), nil
	case *ytypes.Item:
		return appendItemV1(dst, value)
	default:
		return nil, fmt.Errorf("yupdate: struct nao suportada para encode: %T", current)
	}
}

func appendItemV1(dst []byte, item *ytypes.Item) ([]byte, error) {
	content, ok := item.Content.(ParsedContent)
	if !ok {
		return nil, fmt.Errorf("%w: %T", ErrUnsupportedContentEncode, item.Content)
	}

	header := ytypes.ItemHeader{
		ContentRef:     content.ContentRef(),
		HasOrigin:      item.Origin != nil,
		HasRightOrigin: item.RightOrigin != nil,
		HasParentSub:   item.Origin == nil && item.RightOrigin == nil && item.ParentSub != "",
	}
	info, err := header.Encode()
	if err != nil {
		return nil, err
	}
	dst = append(dst, info)

	if item.Origin != nil {
		dst = ytypes.AppendID(dst, *item.Origin)
	}
	if item.RightOrigin != nil {
		dst = ytypes.AppendID(dst, *item.RightOrigin)
	}
	if item.Origin == nil && item.RightOrigin == nil {
		switch item.Parent.Kind() {
		case ytypes.ParentRoot:
			dst = appendVarUintV1(dst, 1)
			dst = appendVarStringV1(dst, item.Parent.Root())
		case ytypes.ParentID:
			parentID, _ := item.Parent.ID()
			dst = appendVarUintV1(dst, 0)
			dst = ytypes.AppendID(dst, parentID)
		default:
			return nil, fmt.Errorf("yupdate: item sem parent wire")
		}
		if item.ParentSub != "" {
			dst = appendVarStringV1(dst, item.ParentSub)
		}
	}
	return content.AppendV1(dst)
}

func groupStructsByClient(structs []ytypes.Struct) map[uint32][]ytypes.Struct {
	groups := make(map[uint32][]ytypes.Struct)
	for _, current := range structs {
		client := current.ID().Client
		groups[client] = append(groups[client], current)
	}
	return groups
}

func encodeEmptyUpdateV1() []byte {
	return AppendDeleteSetBlockV1(varint.Append(nil, 0), ytypes.NewDeleteSet())
}
