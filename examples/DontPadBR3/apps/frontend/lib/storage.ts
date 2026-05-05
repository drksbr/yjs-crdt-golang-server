import fs from "fs";
import path from "path";

export const DATA_DIR = path.join(process.cwd(), "data");

const UUID_RE =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
const VERSION_ID_RE = /^\d{13}-[a-z0-9]+$/i;

function ensureWithinRoot(root: string, candidate: string): string {
  const resolvedRoot = path.resolve(root);
  const resolvedCandidate = path.resolve(candidate);

  if (
    resolvedCandidate !== resolvedRoot &&
    !resolvedCandidate.startsWith(`${resolvedRoot}${path.sep}`)
  ) {
    throw new Error("Unsafe storage path");
  }

  return resolvedCandidate;
}

export function ensureDir(dir: string): string {
  if (!fs.existsSync(dir)) {
    fs.mkdirSync(dir, { recursive: true });
  }

  return dir;
}

export function normalizeOptionalSubdocumentId(
  rawSubdocumentId?: string | null,
): string | undefined {
  if (!rawSubdocumentId) {
    return undefined;
  }

  const decoded = decodeURIComponent(rawSubdocumentId).trim();
  if (
    !decoded ||
    decoded === "." ||
    decoded === ".." ||
    /[\\/\0]/.test(decoded)
  ) {
    throw new Error("Invalid subdocument ID");
  }

  return decoded;
}

export function getDocumentDirectory(
  documentId: string,
  ensureExists: boolean = true,
): string {
  ensureDir(DATA_DIR);
  const documentDir = ensureWithinRoot(DATA_DIR, path.join(DATA_DIR, documentId));

  if (ensureExists) {
    ensureDir(documentDir);
  }

  return documentDir;
}

export function getDocumentScopedDirectory(
  documentId: string,
  segments: string[] = [],
  ensureExists: boolean = true,
): string {
  const documentDir = getDocumentDirectory(documentId, ensureExists);
  const targetDir = ensureWithinRoot(documentDir, path.join(documentDir, ...segments));

  if (ensureExists) {
    ensureDir(targetDir);
  }

  return targetDir;
}

export function resolveDocumentScopedPath(
  documentId: string,
  ...segments: string[]
): string {
  const documentDir = getDocumentDirectory(documentId, false);
  return ensureWithinRoot(documentDir, path.join(documentDir, ...segments));
}

export function assertUuid(value: string, label: string): string {
  if (!UUID_RE.test(value)) {
    throw new Error(`Invalid ${label}`);
  }

  return value;
}

export function assertVersionId(versionId: string): string {
  if (!VERSION_ID_RE.test(versionId)) {
    throw new Error("Invalid version ID");
  }

  return versionId;
}

export function assertStoredFileName(
  fileId: string,
  fileName: string,
): string {
  assertUuid(fileId, "file ID");

  const escapedFileId = fileId.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const pattern = new RegExp(`^${escapedFileId}\\.[a-z0-9]{1,16}$`, "i");

  if (!pattern.test(fileName)) {
    throw new Error("Invalid file name");
  }

  return fileName;
}
