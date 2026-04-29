package ycluster

import (
	"context"
	"fmt"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
)

// StoragePlacementDocumentSourceConfig configura uma fonte de documentos a
// partir da listagem de placements persistidos.
type StoragePlacementDocumentSourceConfig struct {
	Placements storage.PlacementListStore
	Namespace  string
	Limit      int
}

// Validate confirma se a fonte storage-backed pode ser usada.
func (c StoragePlacementDocumentSourceConfig) Validate() error {
	if c.Placements == nil {
		return ErrNilPlacementStore
	}
	if c.Limit < 0 {
		return fmt.Errorf("%w: limit negativo", ErrInvalidRebalancePlan)
	}
	return nil
}

// StoragePlacementDocumentSource alimenta o RebalanceController com documentos
// descobertos a partir do placement store.
type StoragePlacementDocumentSource struct {
	placements storage.PlacementListStore
	namespace  string
	limit      int
}

var _ RebalanceDocumentSource = (*StoragePlacementDocumentSource)(nil)

// NewStoragePlacementDocumentSource cria uma fonte storage-backed para o
// RebalanceController.
func NewStoragePlacementDocumentSource(cfg StoragePlacementDocumentSourceConfig) (*StoragePlacementDocumentSource, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &StoragePlacementDocumentSource{
		placements: cfg.Placements,
		namespace:  cfg.Namespace,
		limit:      cfg.Limit,
	}, nil
}

// RebalanceDocuments lista as chaves de documentos conhecidas pelo placement
// store.
func (s *StoragePlacementDocumentSource) RebalanceDocuments(ctx context.Context) ([]storage.DocumentKey, error) {
	if s == nil || s.placements == nil {
		return nil, ErrNilPlacementStore
	}
	if ctx == nil {
		ctx = context.Background()
	}
	placements, err := s.placements.ListPlacements(ctx, storage.PlacementListOptions{
		Namespace: s.namespace,
		Limit:     s.limit,
	})
	if err != nil {
		return nil, err
	}
	keys := make([]storage.DocumentKey, 0, len(placements))
	for _, placement := range placements {
		if placement == nil {
			continue
		}
		keys = append(keys, placement.Key)
	}
	return keys, nil
}
