package security

import (
	"context"
	"fmt"
	"strings"

	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/common"
	"github.com/gin-gonic/gin"
)

func (s *Service) GetDocumentAccessState(c *gin.Context, rawDocID string) (common.DocumentAccessState, error) {
	docID, err := common.NormalizeDocumentID(rawDocID)
	if err != nil {
		return common.DocumentAccessState{}, err
	}

	row, err := s.loadSecurityRow(c.Request.Context(), docID)
	if err != nil {
		return common.DocumentAccessState{}, err
	}

	hasJWT := s.HasDocumentAccessCookie(c.Request, docID)
	return buildDocumentAccessState(docID, row, hasJWT), nil
}

type securityRow struct {
	VisibilityMode common.VisibilityMode
	PinHash        *string
}

func buildDocumentAccessState(docID string, row securityRow, hasJWT bool) common.DocumentAccessState {
	hasPIN := hasSecurityPIN(row.PinHash)
	isProtected := row.VisibilityMode == common.VisibilityPrivate
	hasAccess := !isProtected || hasJWT
	canEdit := row.VisibilityMode == common.VisibilityPublic || hasJWT

	return common.DocumentAccessState{
		DocumentID:     docID,
		IsProtected:    isProtected,
		HasPIN:         hasPIN,
		VisibilityMode: row.VisibilityMode,
		HasAccess:      hasAccess,
		CanEdit:        canEdit,
	}
}

func NormalizeVisibilityMode(mode string, hash *string) common.VisibilityMode {
	hasPIN := hasSecurityPIN(hash)

	if IsVisibilityModeValid(common.VisibilityMode(mode)) {
		if mode == string(common.VisibilityPublic) && hasPIN {
			return common.VisibilityPrivate
		}
		return common.VisibilityMode(mode)
	}

	if hasPIN {
		return common.VisibilityPrivate
	}

	return common.VisibilityPublic
}

func hasSecurityPIN(hash *string) bool {
	return hash != nil && strings.TrimSpace(*hash) != ""
}

func (s *Service) loadSecurityRow(ctx context.Context, docID string) (securityRow, error) {
	query := fmt.Sprintf(
		`SELECT visibility_mode, pin_hash
		FROM %s.document_security
		WHERE namespace=$1 AND document_id=$2`,
		s.schemaSQL,
	)

	var mode string
	var hash *string
	err := s.db.QueryRow(ctx, query, s.namespace, docID).Scan(&mode, &hash)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return securityRow{VisibilityMode: common.VisibilityPublic, PinHash: nil}, nil
		}
		return securityRow{}, err
	}
	return securityRow{VisibilityMode: NormalizeVisibilityMode(mode, hash), PinHash: hash}, nil
}

func (s *Service) upsertSecurityRow(ctx context.Context, docID string, mode common.VisibilityMode, pinHash *string) error {
	query := fmt.Sprintf(
		`INSERT INTO %s.document_security (namespace, document_id, visibility_mode, pin_hash, updated_at)
		VALUES ($1,$2,$3,$4,now())
		ON CONFLICT (namespace, document_id)
		DO UPDATE SET visibility_mode=EXCLUDED.visibility_mode, pin_hash=EXCLUDED.pin_hash, updated_at=now()`,
		s.schemaSQL,
	)
	_, err := s.db.Exec(ctx, query, s.namespace, docID, string(mode), pinHash)
	return err
}
