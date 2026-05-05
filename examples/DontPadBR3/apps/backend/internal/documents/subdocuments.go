package documents

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/common"
)

func NewDocumentChild(parentDocumentID string, req common.DocumentChildRequest) (common.DocumentChild, error) {
	slugSource := firstNonEmpty(req.Slug, req.ID, req.Name)
	slug, err := common.NormalizeDocumentID(slugSource)
	if err != nil {
		return common.DocumentChild{}, errors.New("invalid subdocument slug")
	}

	documentID := strings.TrimSpace(req.DocumentID)
	if documentID == "" {
		documentID = stableSubdocumentDocumentID(parentDocumentID, slug)
	} else {
		documentID, err = common.NormalizeDocumentID(documentID)
		if err != nil {
			return common.DocumentChild{}, errors.New("invalid subdocument documentId")
		}
	}
	if documentID == parentDocumentID {
		return common.DocumentChild{}, errors.New("subdocument documentId must be independent")
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = slug
	}
	documentType := normalizeSubdocumentType(req.Type)
	ownerSlug := optionalNormalizedSlug(req.OwnerSlug)
	now := time.Now().UnixMilli()

	return common.DocumentChild{
		ID:               slug,
		DocumentID:       documentID,
		ParentDocumentID: parentDocumentID,
		Slug:             slug,
		Name:             name,
		Type:             documentType,
		OwnerSlug:        ownerSlug,
		CreatedAt:        now,
		UpdatedAt:        now,
	}, nil
}

func stableSubdocumentDocumentID(parentDocumentID, slug string) string {
	sum := sha256.Sum256([]byte(parentDocumentID + "\x00" + slug))
	documentID, err := common.NormalizeDocumentID(fmt.Sprintf("%s-%s-%s", parentDocumentID, slug, hex.EncodeToString(sum[:])[:12]))
	if err != nil {
		return parentDocumentID + "-" + slug
	}
	return documentID
}

func normalizeSubdocumentType(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return "texto"
	}
	value = common.DocUnsafePattern.ReplaceAllString(value, "")
	value = strings.Join(strings.Fields(value), "-")
	value = strings.Trim(value, "-")
	if value == "" {
		return "texto"
	}
	if len(value) > 32 {
		value = value[:32]
	}
	return value
}

func optionalNormalizedSlug(raw string) *string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	value, err := common.NormalizeDocumentID(raw)
	if err != nil {
		return nil
	}
	return &value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (s *Service) listDocumentChildren(ctx context.Context, parentDocumentID string) ([]common.DocumentChild, error) {
	query := fmt.Sprintf(
		`SELECT slug, document_id, name, document_type, owner_slug,
			EXTRACT(EPOCH FROM created_at) * 1000,
			EXTRACT(EPOCH FROM updated_at) * 1000
		FROM %s.document_children
		WHERE namespace=$1 AND parent_document_id=$2
		ORDER BY name ASC, slug ASC`,
		s.schemaSQL,
	)
	rows, err := s.db.Query(ctx, query, s.namespace, parentDocumentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	children := make([]common.DocumentChild, 0, 16)
	for rows.Next() {
		child := common.DocumentChild{ParentDocumentID: parentDocumentID}
		var createdMillis, updatedMillis float64
		if err := rows.Scan(&child.Slug, &child.DocumentID, &child.Name, &child.Type, &child.OwnerSlug, &createdMillis, &updatedMillis); err != nil {
			return nil, err
		}
		child.ID = child.Slug
		child.CreatedAt = int64(createdMillis)
		child.UpdatedAt = int64(updatedMillis)
		children = append(children, child)
	}
	return children, rows.Err()
}

func (s *Service) getDocumentChild(ctx context.Context, parentDocumentID, slug string) (common.DocumentChild, error) {
	query := fmt.Sprintf(
		`SELECT document_id, name, document_type, owner_slug,
			EXTRACT(EPOCH FROM created_at) * 1000,
			EXTRACT(EPOCH FROM updated_at) * 1000
		FROM %s.document_children
		WHERE namespace=$1 AND parent_document_id=$2 AND slug=$3
		LIMIT 1`,
		s.schemaSQL,
	)
	child := common.DocumentChild{
		ID:               slug,
		ParentDocumentID: parentDocumentID,
		Slug:             slug,
	}
	var createdMillis, updatedMillis float64
	err := s.db.QueryRow(ctx, query, s.namespace, parentDocumentID, slug).Scan(
		&child.DocumentID,
		&child.Name,
		&child.Type,
		&child.OwnerSlug,
		&createdMillis,
		&updatedMillis,
	)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return common.DocumentChild{}, common.ErrNotFound
		}
		return common.DocumentChild{}, err
	}
	child.CreatedAt = int64(createdMillis)
	child.UpdatedAt = int64(updatedMillis)
	return child, nil
}

func (s *Service) upsertDocumentChild(ctx context.Context, child *common.DocumentChild) error {
	if child == nil {
		return errors.New("nil document child")
	}
	query := fmt.Sprintf(
		`INSERT INTO %s.document_children
		(namespace, parent_document_id, slug, document_id, name, document_type, owner_slug, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,now(),now())
		ON CONFLICT (namespace, parent_document_id, slug)
		DO UPDATE SET name=EXCLUDED.name, document_type=EXCLUDED.document_type, owner_slug=EXCLUDED.owner_slug, updated_at=now()
		RETURNING document_id, EXTRACT(EPOCH FROM created_at) * 1000, EXTRACT(EPOCH FROM updated_at) * 1000`,
		s.schemaSQL,
	)
	var createdMillis, updatedMillis float64
	err := s.db.QueryRow(
		ctx,
		query,
		s.namespace,
		child.ParentDocumentID,
		child.Slug,
		child.DocumentID,
		child.Name,
		child.Type,
		child.OwnerSlug,
	).Scan(&child.DocumentID, &createdMillis, &updatedMillis)
	child.ID = child.Slug
	child.CreatedAt = int64(createdMillis)
	child.UpdatedAt = int64(updatedMillis)
	return err
}

func (s *Service) deleteDocumentChild(ctx context.Context, parentDocumentID, slug string) error {
	query := fmt.Sprintf(
		`DELETE FROM %s.document_children WHERE namespace=$1 AND parent_document_id=$2 AND slug=$3`,
		s.schemaSQL,
	)
	tag, err := s.db.Exec(ctx, query, s.namespace, parentDocumentID, slug)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return common.ErrNotFound
	}
	return nil
}
