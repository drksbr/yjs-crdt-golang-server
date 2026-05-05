package documents

import (
	"context"
	"fmt"
	"strings"

	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/common"
)

func (s *Service) listVersions(ctx context.Context, documentID string, subdoc *string) ([]common.DocumentVersion, error) {
	query, args := s.ListVersionsQuery(documentID, subdoc)
	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]common.DocumentVersion, 0, 32)
	for rows.Next() {
		var v common.DocumentVersion
		var millis float64
		if err := rows.Scan(&v.ID, &v.SubdocumentName, &millis, &v.Label, &v.Size, &v.CreatedBy); err != nil {
			return nil, err
		}
		v.DocumentID = documentID
		v.Timestamp = int64(millis)
		result = append(result, v)
	}
	return result, rows.Err()
}

func (s *Service) ListVersionsQuery(documentID string, subdoc *string) (string, []any) {
	base := fmt.Sprintf(
		`SELECT version_id, subdocument_name, EXTRACT(EPOCH FROM created_at) * 1000, label, size_bytes, created_by
		FROM %s.document_versions
		WHERE namespace=$1 AND document_id=$2`,
		s.schemaSQL,
	)
	args := []any{s.namespace, documentID}
	if subdoc != nil {
		base += " AND subdocument_name=$3"
		args = append(args, *subdoc)
	}
	base += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d", len(args)+1)
	args = append(args, common.MaxVersionsPerDocument)
	return base, args
}

func (s *Service) insertVersion(ctx context.Context, version common.DocumentVersion, data []byte, contentLength int64) error {
	if contentLength <= 0 {
		contentLength = int64(len(data))
	}
	query := fmt.Sprintf(
		`INSERT INTO %s.document_versions
		(namespace, document_id, version_id, subdocument_name, label, created_by, created_at, size_bytes, update_data)
		VALUES ($1,$2,$3,$4,$5,$6,now(),$7,$8)`,
		s.schemaSQL,
	)
	_, err := s.db.Exec(
		ctx,
		query,
		s.namespace,
		version.DocumentID,
		version.ID,
		version.SubdocumentName,
		version.Label,
		version.CreatedBy,
		contentLength,
		data,
	)
	return err
}

func (s *Service) getVersion(ctx context.Context, documentID, versionID string, withData bool) (common.DocumentVersion, []byte, error) {
	query := fmt.Sprintf(
		`SELECT subdocument_name, EXTRACT(EPOCH FROM created_at) * 1000, label, size_bytes, created_by%s
		FROM %s.document_versions
		WHERE namespace=$1 AND document_id=$2 AND version_id=$3
		LIMIT 1`,
		func() string {
			if withData {
				return ", update_data"
			}
			return ""
		}(),
		s.schemaSQL,
	)

	var v common.DocumentVersion
	var millis float64
	var data []byte
	if withData {
		err := s.db.QueryRow(ctx, query, s.namespace, documentID, versionID).Scan(
			&v.SubdocumentName, &millis, &v.Label, &v.Size, &v.CreatedBy, &data,
		)
		if err != nil {
			if strings.Contains(err.Error(), "no rows") {
				return common.DocumentVersion{}, nil, common.ErrNotFound
			}
			return common.DocumentVersion{}, nil, err
		}
	} else {
		err := s.db.QueryRow(ctx, query, s.namespace, documentID, versionID).Scan(
			&v.SubdocumentName, &millis, &v.Label, &v.Size, &v.CreatedBy,
		)
		if err != nil {
			if strings.Contains(err.Error(), "no rows") {
				return common.DocumentVersion{}, nil, common.ErrNotFound
			}
			return common.DocumentVersion{}, nil, err
		}
	}
	v.ID = versionID
	v.DocumentID = documentID
	v.Timestamp = int64(millis)
	return v, data, nil
}

func (s *Service) deleteVersion(ctx context.Context, documentID, versionID string) error {
	query := fmt.Sprintf(
		`DELETE FROM %s.document_versions WHERE namespace=$1 AND document_id=$2 AND version_id=$3`,
		s.schemaSQL,
	)
	tag, err := s.db.Exec(ctx, query, s.namespace, documentID, versionID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return common.ErrNotFound
	}
	return nil
}

func (s *Service) deleteDocumentMetadata(ctx context.Context, documentID string) error {
	stmts := []string{
		fmt.Sprintf(`DELETE FROM %s.document_children WHERE namespace=$1 AND (parent_document_id=$2 OR document_id=$2)`, s.schemaSQL),
		fmt.Sprintf(`DELETE FROM %s.document_versions WHERE namespace=$1 AND document_id=$2`, s.schemaSQL),
		fmt.Sprintf(`DELETE FROM %s.document_security WHERE namespace=$1 AND document_id=$2`, s.schemaSQL),
		fmt.Sprintf(`DELETE FROM %s.document_update_logs WHERE namespace=$1 AND document_id=$2`, s.schemaSQL),
		fmt.Sprintf(`DELETE FROM %s.document_update_log_heads WHERE namespace=$1 AND document_id=$2`, s.schemaSQL),
		fmt.Sprintf(`DELETE FROM %s.document_snapshots WHERE namespace=$1 AND document_id=$2`, s.schemaSQL),
		fmt.Sprintf(`DELETE FROM %s.document_placements WHERE namespace=$1 AND document_id=$2`, s.schemaSQL),
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(ctx, stmt, s.namespace, documentID); err != nil {
			return err
		}
	}
	return nil
}
