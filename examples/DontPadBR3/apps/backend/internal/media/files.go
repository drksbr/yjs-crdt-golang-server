package media

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/common"
)

func (s *Service) readFilesManifest(documentID string, subdoc *string) ([]common.DocumentFile, error) {
	path := s.paths.FilesManifestPath(documentID, subdoc)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []common.DocumentFile{}, nil
		}
		return nil, err
	}
	var manifest common.FilesManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return []common.DocumentFile{}, nil
	}
	if manifest.Files == nil {
		return []common.DocumentFile{}, nil
	}
	return manifest.Files, nil
}

func (s *Service) writeFilesManifest(documentID string, subdoc *string, files []common.DocumentFile) error {
	path := s.paths.FilesManifestPath(documentID, subdoc)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(common.FilesManifest{Files: files}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, payload, 0o644)
}

func (s *Service) listFileEntries(ctx context.Context, documentID string, subdoc *string) ([]common.DocumentFile, error) {
	query := fmt.Sprintf(
		`SELECT file_id, stored_name, original_name, mime_type, size_bytes, storage_path,
			EXTRACT(EPOCH FROM uploaded_at) * 1000
		FROM %s.document_files
		WHERE namespace=$1 AND document_id=$2 AND subdocument_id=$3 AND deleted_at IS NULL
		ORDER BY uploaded_at DESC`,
		s.schemaSQL,
	)
	rows, err := s.db.Query(ctx, query, s.namespace, documentID, common.SubdocumentDBScope(subdoc))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	files := make([]common.DocumentFile, 0, 16)
	for rows.Next() {
		var file common.DocumentFile
		var uploadedMillis float64
		if err := rows.Scan(&file.ID, &file.Name, &file.OriginalName, &file.MimeType, &file.Size, &file.StoragePath, &uploadedMillis); err != nil {
			return nil, err
		}
		file.UploadedAt = int64(uploadedMillis)
		files = append(files, file)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(files) > 0 {
		return files, nil
	}
	return s.readFilesManifest(documentID, subdoc)
}

func (s *Service) upsertFileEntry(ctx context.Context, documentID string, subdoc *string, entry common.DocumentFile) error {
	if entry.StoragePath == "" {
		entry.StoragePath = filepath.ToSlash(filepath.Join("documents", documentID, entry.Name))
	}
	query := fmt.Sprintf(
		`INSERT INTO %s.document_files
		(namespace, document_id, subdocument_id, file_id, stored_name, original_name, mime_type, size_bytes, storage_path, uploaded_at, deleted_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,to_timestamp($10::double precision / 1000),NULL)
		ON CONFLICT (namespace, document_id, subdocument_id, file_id) DO UPDATE SET
			stored_name=EXCLUDED.stored_name,
			original_name=EXCLUDED.original_name,
			mime_type=EXCLUDED.mime_type,
			size_bytes=EXCLUDED.size_bytes,
			storage_path=EXCLUDED.storage_path,
			uploaded_at=EXCLUDED.uploaded_at,
			deleted_at=NULL`,
		s.schemaSQL,
	)
	if _, err := s.db.Exec(
		ctx,
		query,
		s.namespace,
		documentID,
		common.SubdocumentDBScope(subdoc),
		entry.ID,
		entry.Name,
		entry.OriginalName,
		entry.MimeType,
		entry.Size,
		entry.StoragePath,
		entry.UploadedAt,
	); err != nil {
		return err
	}

	files, err := s.readFilesManifest(documentID, subdoc)
	if err != nil {
		return err
	}
	next := make([]common.DocumentFile, 0, len(files)+1)
	for _, existing := range files {
		if existing.ID == entry.ID {
			continue
		}
		next = append(next, existing)
	}
	next = append(next, entry)
	return s.writeFilesManifest(documentID, subdoc, next)
}

func (s *Service) getFileEntry(ctx context.Context, documentID string, subdoc *string, fileID string) (*common.DocumentFile, error) {
	query := fmt.Sprintf(
		`SELECT file_id, stored_name, original_name, mime_type, size_bytes, storage_path,
			EXTRACT(EPOCH FROM uploaded_at) * 1000
		FROM %s.document_files
		WHERE namespace=$1 AND document_id=$2 AND subdocument_id=$3 AND file_id=$4 AND deleted_at IS NULL
		LIMIT 1`,
		s.schemaSQL,
	)
	var file common.DocumentFile
	var uploadedMillis float64
	err := s.db.QueryRow(ctx, query, s.namespace, documentID, common.SubdocumentDBScope(subdoc), fileID).Scan(
		&file.ID,
		&file.Name,
		&file.OriginalName,
		&file.MimeType,
		&file.Size,
		&file.StoragePath,
		&uploadedMillis,
	)
	if err == nil {
		file.UploadedAt = int64(uploadedMillis)
		return &file, nil
	}
	if !strings.Contains(err.Error(), "no rows") {
		return nil, err
	}

	files, err := s.readFilesManifest(documentID, subdoc)
	if err != nil {
		return nil, err
	}
	for i := range files {
		if files[i].ID == fileID {
			value := files[i]
			return &value, nil
		}
	}
	return nil, nil
}

func (s *Service) removeFileEntry(ctx context.Context, documentID string, subdoc *string, fileID string) error {
	query := fmt.Sprintf(
		`UPDATE %s.document_files
		SET deleted_at=now()
		WHERE namespace=$1 AND document_id=$2 AND subdocument_id=$3 AND file_id=$4`,
		s.schemaSQL,
	)
	if _, err := s.db.Exec(ctx, query, s.namespace, documentID, common.SubdocumentDBScope(subdoc), fileID); err != nil {
		return err
	}

	files, err := s.readFilesManifest(documentID, subdoc)
	if err != nil {
		return err
	}
	next := make([]common.DocumentFile, 0, len(files))
	for _, file := range files {
		if file.ID == fileID {
			continue
		}
		next = append(next, file)
	}
	return s.writeFilesManifest(documentID, subdoc, next)
}
