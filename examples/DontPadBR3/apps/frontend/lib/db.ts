import fs from "fs";
import path from "path";
import { DocumentFile } from "./types";
import {
  ensureDir,
  getDocumentScopedDirectory,
  resolveDocumentScopedPath,
} from "./storage";

const FILE_MANIFEST = ".files-manifest.json";

function getManifestPath(documentId: string, subdocumentId?: string): string {
  const segments = subdocumentId ? [subdocumentId, FILE_MANIFEST] : [FILE_MANIFEST];
  return resolveDocumentScopedPath(documentId, ...segments);
}

function readFileManifest(
  documentId: string,
  subdocumentId?: string,
): DocumentFile[] {
  const manifestPath = getManifestPath(documentId, subdocumentId);
  if (!fs.existsSync(manifestPath)) {
    return [];
  }

  try {
    const manifest = JSON.parse(fs.readFileSync(manifestPath, "utf-8"));
    return Array.isArray(manifest.files) ? manifest.files : [];
  } catch {
    return [];
  }
}

function writeFileManifest(
  documentId: string,
  files: DocumentFile[],
  subdocumentId?: string,
): void {
  const manifestPath = getManifestPath(documentId, subdocumentId);
  ensureDir(path.dirname(manifestPath));
  fs.writeFileSync(
    manifestPath,
    JSON.stringify({ files }, null, 2),
    "utf-8",
  );
}

// Get document/subdocument specific uploads directory
export function getDocumentUploadsDir(
  documentId: string,
  subdocumentId?: string,
): string {
  const segments = subdocumentId ? [subdocumentId] : [];
  return getDocumentScopedDirectory(documentId, segments, true);
}

// Add file to document/subdocument
// NOTE: File metadata is now stored in Y-Sweet via Y.Array in the document
// This function returns the file metadata for the API response
export function addFileToDocument(
  documentId: string,
  file: DocumentFile,
  subdocumentId?: string,
): DocumentFile {
  // Get the appropriate upload directory
  getDocumentUploadsDir(documentId, subdocumentId);

  const files = readFileManifest(documentId, subdocumentId).filter(
    (entry) => entry.id !== file.id,
  );
  files.push(file);
  writeFileManifest(documentId, files, subdocumentId);

  return file;
}

export function getDocumentFile(
  documentId: string,
  fileId: string,
  subdocumentId?: string,
): DocumentFile | null {
  return (
    readFileManifest(documentId, subdocumentId).find((file) => file.id === fileId) ??
    null
  );
}

export function removeFileFromDocument(
  documentId: string,
  fileId: string,
  subdocumentId?: string,
): DocumentFile | null {
  const files = readFileManifest(documentId, subdocumentId);
  const file = files.find((entry) => entry.id === fileId) ?? null;

  if (!file) {
    return null;
  }

  writeFileManifest(
    documentId,
    files.filter((entry) => entry.id !== fileId),
    subdocumentId,
  );

  return file;
}

// Get files for document/subdocument
// NOTE: File metadata comes from Y-Sweet Y.Array
// This also returns the server-side manifest used to validate downloads/deletes.
export function getDocumentFiles(
  documentId: string,
  subdocumentId?: string,
): DocumentFile[] {
  return readFileManifest(documentId, subdocumentId);
}
