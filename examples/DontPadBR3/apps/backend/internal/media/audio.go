package media

import (
	"context"
	"fmt"
	"strings"

	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/common"
)

func (s *Service) listAudioNotes(ctx context.Context, documentID string, subdoc *string) ([]common.AudioNote, error) {
	query := fmt.Sprintf(
		`SELECT note_id, name, duration_seconds, mime_type, size_bytes, storage_path,
			EXTRACT(EPOCH FROM created_at) * 1000
		FROM %s.document_audio_notes
		WHERE namespace=$1 AND document_id=$2 AND subdocument_id=$3 AND deleted_at IS NULL
		ORDER BY created_at DESC`,
		s.schemaSQL,
	)
	rows, err := s.db.Query(ctx, query, s.namespace, documentID, common.SubdocumentDBScope(subdoc))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	notes := make([]common.AudioNote, 0, 8)
	for rows.Next() {
		var note common.AudioNote
		var createdMillis float64
		if err := rows.Scan(&note.ID, &note.Name, &note.Duration, &note.MimeType, &note.Size, &note.StoragePath, &createdMillis); err != nil {
			return nil, err
		}
		note.CreatedAt = int64(createdMillis)
		notes = append(notes, note)
	}
	return notes, rows.Err()
}

func (s *Service) upsertAudioNote(ctx context.Context, documentID string, subdoc *string, note common.AudioNote) error {
	query := fmt.Sprintf(
		`INSERT INTO %s.document_audio_notes
		(namespace, document_id, subdocument_id, note_id, name, duration_seconds, mime_type, size_bytes, storage_path, created_at, deleted_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,to_timestamp($10::double precision / 1000),NULL)
		ON CONFLICT (namespace, document_id, subdocument_id, note_id) DO UPDATE SET
			name=EXCLUDED.name,
			duration_seconds=EXCLUDED.duration_seconds,
			mime_type=EXCLUDED.mime_type,
			size_bytes=EXCLUDED.size_bytes,
			storage_path=EXCLUDED.storage_path,
			created_at=EXCLUDED.created_at,
			deleted_at=NULL`,
		s.schemaSQL,
	)
	_, err := s.db.Exec(
		ctx,
		query,
		s.namespace,
		documentID,
		common.SubdocumentDBScope(subdoc),
		note.ID,
		note.Name,
		note.Duration,
		note.MimeType,
		note.Size,
		note.StoragePath,
		note.CreatedAt,
	)
	return err
}

func (s *Service) getAudioNote(ctx context.Context, documentID string, subdoc *string, noteID string) (*common.AudioNote, error) {
	query := fmt.Sprintf(
		`SELECT note_id, name, duration_seconds, mime_type, size_bytes, storage_path,
			EXTRACT(EPOCH FROM created_at) * 1000
		FROM %s.document_audio_notes
		WHERE namespace=$1 AND document_id=$2 AND subdocument_id=$3 AND note_id=$4 AND deleted_at IS NULL
		LIMIT 1`,
		s.schemaSQL,
	)
	var note common.AudioNote
	var createdMillis float64
	err := s.db.QueryRow(ctx, query, s.namespace, documentID, common.SubdocumentDBScope(subdoc), noteID).Scan(
		&note.ID,
		&note.Name,
		&note.Duration,
		&note.MimeType,
		&note.Size,
		&note.StoragePath,
		&createdMillis,
	)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, nil
		}
		return nil, err
	}
	note.CreatedAt = int64(createdMillis)
	return &note, nil
}

func (s *Service) removeAudioNote(ctx context.Context, documentID string, subdoc *string, noteID string) error {
	query := fmt.Sprintf(
		`UPDATE %s.document_audio_notes
		SET deleted_at=now()
		WHERE namespace=$1 AND document_id=$2 AND subdocument_id=$3 AND note_id=$4`,
		s.schemaSQL,
	)
	_, err := s.db.Exec(ctx, query, s.namespace, documentID, common.SubdocumentDBScope(subdoc), noteID)
	return err
}
