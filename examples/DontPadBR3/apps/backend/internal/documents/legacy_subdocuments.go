package documents

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/common"
	"github.com/drksbr/yjs-crdt-golang-server/internal/ytypes"
	"github.com/drksbr/yjs-crdt-golang-server/internal/yupdate"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yjsbridge"
)

type legacySubdocumentSpec struct {
	slug  string
	name  string
	typ   string
	roots map[string]string
}

type legacyIDMapping struct {
	oldID ytypes.ID
	newID ytypes.ID
	len   uint32
}

func (s *Service) ensureLegacySubdocumentsMigrated(ctx context.Context, parentDocumentID string) error {
	if s == nil || s.legacy == nil || s.store == nil {
		return nil
	}

	children, err := s.listDocumentChildren(ctx, parentDocumentID)
	if err != nil {
		return err
	}
	if len(children) > 0 {
		return nil
	}

	legacyKey := s.legacy.documentKey(parentDocumentID)
	exists, err := s.legacy.exists(ctx, legacyKey)
	if err != nil {
		return fmt.Errorf("stat legacy ysweet %s: %w", legacyKey, err)
	}
	if !exists {
		return nil
	}

	update, err := s.legacy.readUpdate(ctx, legacyKey)
	if err != nil {
		return err
	}
	return s.migrateLegacySubdocuments(ctx, parentDocumentID, update)
}

func (s *Service) migrateLegacySubdocuments(ctx context.Context, parentDocumentID string, legacyUpdate []byte) error {
	specs, err := discoverLegacySubdocuments(legacyUpdate)
	if err != nil {
		return fmt.Errorf("discover legacy subdocuments for %s: %w", parentDocumentID, err)
	}
	if len(specs) == 0 {
		return nil
	}

	for _, spec := range specs {
		child, err := NewDocumentChild(parentDocumentID, common.DocumentChildRequest{
			Slug: spec.slug,
			Name: spec.name,
			Type: spec.typ,
		})
		if err != nil {
			return fmt.Errorf("create migrated legacy subdocument %s/%s: %w", parentDocumentID, spec.slug, err)
		}
		if err := s.upsertDocumentChild(ctx, &child); err != nil {
			return fmt.Errorf("upsert migrated legacy subdocument %s/%s: %w", parentDocumentID, spec.slug, err)
		}

		childKey := storage.DocumentKey{Namespace: s.namespace, DocumentID: child.DocumentID}
		hasState, err := s.hasPersistedDocumentState(ctx, childKey)
		if err != nil {
			return err
		}
		if hasState {
			continue
		}

		update, err := extractLegacySubdocumentUpdate(legacyUpdate, spec.roots)
		if err != nil {
			return fmt.Errorf("extract legacy subdocument %s/%s: %w", parentDocumentID, spec.slug, err)
		}
		snapshot, err := yjsbridge.PersistedSnapshotFromUpdate(update)
		if err != nil {
			return fmt.Errorf("decode legacy subdocument %s/%s: %w", parentDocumentID, spec.slug, err)
		}
		if _, err := s.store.SaveSnapshotCheckpoint(ctx, childKey, snapshot, 0); err != nil {
			return fmt.Errorf("save migrated legacy subdocument %s/%s: %w", parentDocumentID, spec.slug, err)
		}
		log.Printf("legacy ysweet migrated subdoc parent=%s slug=%s doc=%s type=%s", parentDocumentID, spec.slug, child.DocumentID, spec.typ)
	}
	return nil
}

func discoverLegacySubdocuments(update []byte) ([]legacySubdocumentSpec, error) {
	decoded, err := yupdate.DecodeUpdate(update)
	if err != nil {
		return nil, err
	}

	bySlug := make(map[string]*legacySubdocumentSpec)
	for _, current := range decoded.Structs {
		item, ok := current.(*ytypes.Item)
		if !ok || item.Parent.Kind() != ytypes.ParentRoot {
			continue
		}

		root := item.Parent.Root()
		if slug, ok := legacyMetaSubdocumentSlug(root, item.ParentSub); ok {
			ensureLegacySubdocumentSpec(bySlug, slug, "", "texto")
			continue
		}

		slug, typ, currentRoot, ok := legacySubdocumentRoot(root)
		if !ok {
			continue
		}
		spec := ensureLegacySubdocumentSpec(bySlug, slug, root, typ)
		spec.roots[root] = currentRoot
		spec.typ = chooseLegacySubdocumentType(spec.typ, typ)
	}

	specs := make([]legacySubdocumentSpec, 0, len(bySlug))
	for _, spec := range bySlug {
		if spec.name == "" {
			spec.name = spec.slug
		}
		specs = append(specs, *spec)
	}
	sort.Slice(specs, func(i, j int) bool {
		return specs[i].slug < specs[j].slug
	})
	return specs, nil
}

func extractLegacySubdocumentUpdate(update []byte, roots map[string]string) ([]byte, error) {
	if len(roots) == 0 {
		return yupdate.EncodeV1(&yupdate.DecodedUpdate{DeleteSet: ytypes.NewDeleteSet()})
	}

	decoded, err := yupdate.DecodeUpdate(update)
	if err != nil {
		return nil, err
	}

	selected := selectLegacySubdocumentStructs(decoded.Structs, roots)
	out, err := rewriteLegacySubdocumentStructs(decoded.Structs, selected, roots)
	if err != nil {
		return nil, err
	}

	return yupdate.EncodeV1(&yupdate.DecodedUpdate{
		Structs:   out,
		DeleteSet: filterLegacySubdocumentDeleteSet(decoded, selected),
	})
}

func ensureLegacySubdocumentSpec(specs map[string]*legacySubdocumentSpec, slug, legacyRoot, typ string) *legacySubdocumentSpec {
	spec := specs[slug]
	if spec == nil {
		spec = &legacySubdocumentSpec{
			slug:  slug,
			name:  legacySubdocumentDisplayName(slug),
			typ:   typ,
			roots: make(map[string]string),
		}
		specs[slug] = spec
	}
	if spec.typ == "" {
		spec.typ = typ
	}
	if legacyRoot != "" {
		spec.roots[legacyRoot] = ""
	}
	return spec
}

func legacySubdocumentRoot(root string) (slug, typ, currentRoot string, ok bool) {
	switch {
	case strings.HasPrefix(root, "text:"):
		return strings.TrimPrefix(root, "text:"), "texto", "text", true
	case strings.HasPrefix(root, "md:"):
		return strings.TrimPrefix(root, "md:"), "markdown", "markdown", true
	case strings.HasPrefix(root, "markdown:"):
		return strings.TrimPrefix(root, "markdown:"), "markdown", "markdown", true
	case strings.HasPrefix(root, "checklist:"):
		return strings.TrimPrefix(root, "checklist:"), "checklist", "checklist", true
	case strings.HasPrefix(root, "kanban-cols:"):
		return strings.TrimPrefix(root, "kanban-cols:"), "kanban", "kanban-cols", true
	case strings.HasPrefix(root, "kanban-items:"):
		return strings.TrimPrefix(root, "kanban-items:"), "kanban", "kanban-items", true
	case strings.HasPrefix(root, "drawing:"):
		return strings.TrimPrefix(root, "drawing:"), "desenho", "drawing", true
	default:
		return "", "", "", false
	}
}

func legacyMetaSubdocumentSlug(root, parentSub string) (string, bool) {
	if root != "meta" || !strings.HasPrefix(parentSub, "subdoc:") {
		return "", false
	}
	value := strings.TrimPrefix(parentSub, "subdoc:")
	if idx := strings.LastIndex(value, ":"); idx > 0 {
		value = value[:idx]
	}
	return value, value != ""
}

func chooseLegacySubdocumentType(current, candidate string) string {
	rank := map[string]int{
		"":          0,
		"texto":     1,
		"markdown":  2,
		"desenho":   3,
		"checklist": 4,
		"kanban":    5,
	}
	if rank[candidate] > rank[current] {
		return candidate
	}
	return current
}

func legacySubdocumentDisplayName(slug string) string {
	name := strings.TrimSpace(slug)
	if name == "" {
		return slug
	}
	name = strings.ReplaceAll(name, "20", " ")
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.Join(strings.Fields(name), " ")
	if name == "" {
		return slug
	}
	return name
}

func selectLegacySubdocumentStructs(structs []ytypes.Struct, roots map[string]string) map[int]bool {
	selected := make(map[int]bool)
	for idx, current := range structs {
		item, ok := current.(*ytypes.Item)
		if !ok || item.Parent.Kind() != ytypes.ParentRoot {
			continue
		}
		if _, ok := roots[item.Parent.Root()]; ok {
			selected[idx] = true
		}
	}

	for changed := true; changed; {
		changed = false
		for idx, current := range structs {
			if selected[idx] {
				if item, ok := current.(*ytypes.Item); ok {
					changed = selectLegacyDependency(structs, selected, item.Origin) || changed
					changed = selectLegacyDependency(structs, selected, item.RightOrigin) || changed
					if parentID, ok := item.Parent.ID(); ok {
						changed = selectLegacyDependency(structs, selected, &parentID) || changed
					}
				}
				continue
			}

			item, ok := current.(*ytypes.Item)
			if !ok {
				continue
			}
			if legacyIDSelected(structs, selected, item.Origin) ||
				legacyIDSelected(structs, selected, item.RightOrigin) {
				selected[idx] = true
				changed = true
				continue
			}
			if parentID, ok := item.Parent.ID(); ok && legacyIDSelected(structs, selected, &parentID) {
				selected[idx] = true
				changed = true
			}
		}
	}
	return selected
}

func selectLegacyDependency(structs []ytypes.Struct, selected map[int]bool, id *ytypes.ID) bool {
	if id == nil || legacyIDSelected(structs, selected, id) {
		return false
	}
	idx := legacyStructIndexContainingID(structs, *id)
	if idx < 0 {
		return false
	}
	selected[idx] = true
	return true
}

func legacyIDSelected(structs []ytypes.Struct, selected map[int]bool, id *ytypes.ID) bool {
	if id == nil {
		return false
	}
	for idx := range selected {
		current := structs[idx]
		if current.ID().Client == id.Client && current.ContainsClock(id.Clock) {
			return true
		}
	}
	return false
}

func legacyStructIndexContainingID(structs []ytypes.Struct, id ytypes.ID) int {
	for idx, current := range structs {
		if current.ID().Client == id.Client && current.ContainsClock(id.Clock) {
			return idx
		}
	}
	return -1
}

func rewriteLegacySubdocumentStructs(structs []ytypes.Struct, selected map[int]bool, roots map[string]string) ([]ytypes.Struct, error) {
	indexes := make([]int, 0, len(selected))
	for idx := range selected {
		indexes = append(indexes, idx)
	}
	sort.Slice(indexes, func(i, j int) bool {
		left := structs[indexes[i]].ID()
		right := structs[indexes[j]].ID()
		if left.Client != right.Client {
			return left.Client > right.Client
		}
		return left.Clock < right.Clock
	})

	mappings := make([]legacyIDMapping, 0, len(indexes))
	nextClockByClient := make(map[uint32]uint32)
	for _, idx := range indexes {
		current := structs[idx]
		id := current.ID()
		nextClock := nextClockByClient[id.Client]
		mappings = append(mappings, legacyIDMapping{
			oldID: id,
			newID: ytypes.ID{
				Client: id.Client,
				Clock:  nextClock,
			},
			len: current.Length(),
		})
		nextClockByClient[id.Client] = nextClock + current.Length()
	}

	out := make([]ytypes.Struct, 0, len(indexes))
	for offset, idx := range indexes {
		current := structs[idx]
		rewritten, err := rewriteLegacySubdocumentStruct(current, roots, mappings, mappings[offset].newID)
		if err != nil {
			return nil, err
		}
		out = append(out, rewritten)
	}
	return out, nil
}

func rewriteLegacySubdocumentStruct(current ytypes.Struct, roots map[string]string, mappings []legacyIDMapping, id ytypes.ID) (ytypes.Struct, error) {
	item, ok := current.(*ytypes.Item)
	if !ok {
		switch current.Kind() {
		case ytypes.KindGC:
			return ytypes.NewGC(id, current.Length())
		case ytypes.KindSkip:
			return ytypes.NewSkip(id, current.Length())
		default:
			return nil, fmt.Errorf("unsupported legacy subdocument struct: %T", current)
		}
	}

	parent := item.Parent
	if item.Parent.Kind() == ytypes.ParentRoot {
		if currentRoot, ok := roots[item.Parent.Root()]; ok && currentRoot != "" {
			nextParent, err := ytypes.NewParentRoot(currentRoot)
			if err != nil {
				return nil, err
			}
			parent = nextParent
		}
	} else if parentID, ok := item.Parent.ID(); ok {
		nextParentID, err := remapLegacyID(parentID, mappings)
		if err != nil {
			return nil, err
		}
		parent = ytypes.NewParentID(nextParentID)
	}

	origin, err := remapOptionalLegacyID(item.Origin, mappings)
	if err != nil {
		return nil, err
	}
	rightOrigin, err := remapOptionalLegacyID(item.RightOrigin, mappings)
	if err != nil {
		return nil, err
	}
	redone, err := remapOptionalLegacyID(item.Redone, mappings)
	if err != nil {
		return nil, err
	}

	return ytypes.NewItem(id, item.Content, ytypes.ItemOptions{
		Origin:      origin,
		RightOrigin: rightOrigin,
		Parent:      parent,
		ParentSub:   item.ParentSub,
		Redone:      redone,
		Flags:       item.Info,
	})
}

func filterLegacySubdocumentDeleteSet(decoded *yupdate.DecodedUpdate, selected map[int]bool) *ytypes.DeleteSet {
	out := ytypes.NewDeleteSet()
	if decoded == nil || decoded.DeleteSet == nil {
		return out
	}
	indexes := make([]int, 0, len(selected))
	for idx := range selected {
		indexes = append(indexes, idx)
	}
	sort.Slice(indexes, func(i, j int) bool {
		left := decoded.Structs[indexes[i]].ID()
		right := decoded.Structs[indexes[j]].ID()
		if left.Client != right.Client {
			return left.Client > right.Client
		}
		return left.Clock < right.Clock
	})
	mappings := make([]legacyIDMapping, 0, len(indexes))
	nextClockByClient := make(map[uint32]uint32)
	for _, idx := range indexes {
		current := decoded.Structs[idx]
		id := current.ID()
		nextClock := nextClockByClient[id.Client]
		mappings = append(mappings, legacyIDMapping{
			oldID: id,
			newID: ytypes.ID{
				Client: id.Client,
				Clock:  nextClock,
			},
			len: current.Length(),
		})
		nextClockByClient[id.Client] = nextClock + current.Length()
	}

	for _, client := range decoded.DeleteSet.Clients() {
		for _, deletion := range decoded.DeleteSet.Ranges(client) {
			deleteStart := uint64(deletion.Clock)
			deleteEnd := deletion.End()
			for _, mapping := range mappings {
				if mapping.oldID.Client != client {
					continue
				}
				oldStart := uint64(mapping.oldID.Clock)
				oldEnd := oldStart + uint64(mapping.len)
				start := maxUint64(deleteStart, oldStart)
				end := minUint64(deleteEnd, oldEnd)
				if end > start {
					nextClock := mapping.newID.Clock + uint32(start-oldStart)
					_ = out.Add(client, nextClock, uint32(end-start))
				}
			}
		}
	}
	return out
}

func remapOptionalLegacyID(id *ytypes.ID, mappings []legacyIDMapping) (*ytypes.ID, error) {
	if id == nil {
		return nil, nil
	}
	next, err := remapLegacyID(*id, mappings)
	if err != nil {
		return nil, err
	}
	return &next, nil
}

func remapLegacyID(id ytypes.ID, mappings []legacyIDMapping) (ytypes.ID, error) {
	for _, mapping := range mappings {
		if mapping.oldID.Client != id.Client {
			continue
		}
		oldStart := mapping.oldID.Clock
		oldEnd := oldStart + mapping.len
		if id.Clock >= oldStart && id.Clock < oldEnd {
			return ytypes.ID{
				Client: mapping.newID.Client,
				Clock:  mapping.newID.Clock + (id.Clock - oldStart),
			}, nil
		}
	}
	return ytypes.ID{}, fmt.Errorf("legacy id %d:%d is outside selected subdocument structs", id.Client, id.Clock)
}

func minUint64(left, right uint64) uint64 {
	if left < right {
		return left
	}
	return right
}

func maxUint64(left, right uint64) uint64 {
	if left > right {
		return left
	}
	return right
}
