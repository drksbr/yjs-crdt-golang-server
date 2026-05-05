import { sanitizeDocumentId } from "./colors";
import { hasDocumentAccess } from "./jwt";
import { getDocumentSecurity } from "./ysweet";

export type DocumentPermission = "view" | "edit";

export interface DocumentAccessState {
  documentId: string;
  isProtected: boolean;
  visibilityMode: "public" | "public-readonly" | "private";
  hasAccess: boolean;
  canEdit: boolean;
}

export function normalizeDocumentId(rawDocumentId: string): string {
  let sanitizedId = sanitizeDocumentId(decodeURIComponent(rawDocumentId));

  if (sanitizedId.startsWith("doc_")) {
    sanitizedId = sanitizedId.slice(4);
  }

  if (!sanitizedId) {
    throw new Error("Invalid document ID");
  }

  return sanitizedId;
}

export async function getDocumentAccessState(
  rawDocumentId: string,
): Promise<DocumentAccessState> {
  const documentId = normalizeDocumentId(rawDocumentId);
  const { isProtected, visibilityMode } = await getDocumentSecurity(documentId);
  const hasJwt = await hasDocumentAccess(documentId);
  const hasAccess = !isProtected || hasJwt;
  const canEdit = visibilityMode === "public" || hasJwt;

  return {
    documentId,
    isProtected,
    visibilityMode,
    hasAccess,
    canEdit,
  };
}

export function hasDocumentPermission(
  access: DocumentAccessState,
  permission: DocumentPermission,
): boolean {
  return permission === "edit" ? access.canEdit : access.hasAccess;
}
