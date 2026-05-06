package ytypes

import internal "github.com/drksbr/yjs-crdt-golang-server/internal/ytypes"

type (
	Content     = internal.Content
	DeleteRange = internal.DeleteRange
	DeleteSet   = internal.DeleteSet
	GC          = internal.GC
	ID          = internal.ID
	Item        = internal.Item
	ItemFlags   = internal.ItemFlags
	ItemOptions = internal.ItemOptions
	Kind        = internal.Kind
	ParentKind  = internal.ParentKind
	ParentRef   = internal.ParentRef
	Skip        = internal.Skip
	Struct      = internal.Struct
)

const (
	ItemFlagKeep      = internal.ItemFlagKeep
	ItemFlagCountable = internal.ItemFlagCountable
	ItemFlagDeleted   = internal.ItemFlagDeleted
	ItemFlagMarker    = internal.ItemFlagMarker

	KindGC   = internal.KindGC
	KindItem = internal.KindItem
	KindSkip = internal.KindSkip

	ParentNone = internal.ParentNone
	ParentID   = internal.ParentID
	ParentRoot = internal.ParentRoot
)

var (
	NewDeleteSet  = internal.NewDeleteSet
	NewGC         = internal.NewGC
	NewItem       = internal.NewItem
	NewParentID   = internal.NewParentID
	NewParentRoot = internal.NewParentRoot
	NewSkip       = internal.NewSkip
)
