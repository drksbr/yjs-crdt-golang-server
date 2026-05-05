package server

import (
	"context"
	"fmt"
)

func (a *Server) ensureMetadataSchema(ctx context.Context) error {
	queries := []string{
		fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS %s`, a.schemaSQL),
		fmt.Sprintf(
			`CREATE TABLE IF NOT EXISTS %s.document_security (
				namespace text NOT NULL,
				document_id text NOT NULL,
				visibility_mode text NOT NULL DEFAULT 'public',
				pin_hash text NULL,
				updated_at timestamptz NOT NULL DEFAULT now(),
				PRIMARY KEY (namespace, document_id),
				CHECK (visibility_mode IN ('public','public-readonly','private'))
			)`,
			a.schemaSQL,
		),
		fmt.Sprintf(
			`CREATE TABLE IF NOT EXISTS %s.document_versions (
				namespace text NOT NULL,
				document_id text NOT NULL,
				version_id text NOT NULL,
				subdocument_name text NULL,
				label text NULL,
				created_by text NULL,
				created_at timestamptz NOT NULL DEFAULT now(),
				size_bytes bigint NOT NULL,
				update_data bytea NOT NULL,
				PRIMARY KEY (namespace, document_id, version_id)
			)`,
			a.schemaSQL,
		),
		fmt.Sprintf(
			`CREATE TABLE IF NOT EXISTS %s.document_children (
				namespace text NOT NULL,
				parent_document_id text NOT NULL,
				slug text NOT NULL,
				document_id text NOT NULL,
				name text NOT NULL,
				document_type text NOT NULL DEFAULT 'texto',
				owner_slug text NULL,
				created_at timestamptz NOT NULL DEFAULT now(),
				updated_at timestamptz NOT NULL DEFAULT now(),
				PRIMARY KEY (namespace, parent_document_id, slug),
				UNIQUE (namespace, document_id)
			)`,
			a.schemaSQL,
		),
		fmt.Sprintf(
			`CREATE TABLE IF NOT EXISTS %s.document_files (
				namespace text NOT NULL,
				document_id text NOT NULL,
				subdocument_id text NOT NULL DEFAULT '',
				file_id text NOT NULL,
				stored_name text NOT NULL,
				original_name text NOT NULL,
				mime_type text NOT NULL,
				size_bytes bigint NOT NULL,
				storage_path text NOT NULL,
				uploaded_at timestamptz NOT NULL DEFAULT now(),
				deleted_at timestamptz NULL,
				PRIMARY KEY (namespace, document_id, subdocument_id, file_id)
			)`,
			a.schemaSQL,
		),
		fmt.Sprintf(
			`CREATE TABLE IF NOT EXISTS %s.document_audio_notes (
				namespace text NOT NULL,
				document_id text NOT NULL,
				subdocument_id text NOT NULL DEFAULT '',
				note_id text NOT NULL,
				name text NOT NULL,
				duration_seconds double precision NOT NULL DEFAULT 0,
				mime_type text NOT NULL,
				size_bytes bigint NOT NULL,
				storage_path text NOT NULL,
				created_at timestamptz NOT NULL DEFAULT now(),
				deleted_at timestamptz NULL,
				PRIMARY KEY (namespace, document_id, subdocument_id, note_id)
			)`,
			a.schemaSQL,
		),
		fmt.Sprintf(
			`CREATE INDEX IF NOT EXISTS document_children_parent_idx
			 ON %s.document_children (namespace, parent_document_id, updated_at DESC)`,
			a.schemaSQL,
		),
		fmt.Sprintf(
			`CREATE INDEX IF NOT EXISTS document_versions_created_at_idx
			 ON %s.document_versions (namespace, document_id, created_at DESC)`,
			a.schemaSQL,
		),
		fmt.Sprintf(
			`CREATE INDEX IF NOT EXISTS document_versions_subdoc_idx
			 ON %s.document_versions (namespace, document_id, subdocument_name, created_at DESC)`,
			a.schemaSQL,
		),
		fmt.Sprintf(
			`CREATE INDEX IF NOT EXISTS document_files_scope_idx
			 ON %s.document_files (namespace, document_id, subdocument_id, uploaded_at DESC)
			 WHERE deleted_at IS NULL`,
			a.schemaSQL,
		),
		fmt.Sprintf(
			`CREATE INDEX IF NOT EXISTS document_audio_notes_scope_idx
			 ON %s.document_audio_notes (namespace, document_id, subdocument_id, created_at DESC)
			 WHERE deleted_at IS NULL`,
			a.schemaSQL,
		),
	}
	for _, query := range queries {
		if _, err := a.metaDB.Exec(ctx, query); err != nil {
			return fmt.Errorf("metadata schema query failed: %w", err)
		}
	}
	return nil
}
