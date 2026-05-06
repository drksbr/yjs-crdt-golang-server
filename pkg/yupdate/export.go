package yupdate

import internal "github.com/drksbr/yjs-crdt-golang-server/internal/yupdate"

type (
	DecodedUpdate = internal.DecodedUpdate
	UpdateFormat  = internal.UpdateFormat
)

const (
	UpdateFormatUnknown = internal.UpdateFormatUnknown
	UpdateFormatV1      = internal.UpdateFormatV1
	UpdateFormatV2      = internal.UpdateFormatV2
)

var (
	DecodeUpdate     = internal.DecodeUpdate
	DecodeV1         = internal.DecodeV1
	DecodeV2         = internal.DecodeV2
	EncodeV1         = internal.EncodeV1
	EncodeV2         = internal.EncodeV2
	FormatFromUpdate = internal.FormatFromUpdate
	ValidateUpdate   = internal.ValidateUpdate
)
