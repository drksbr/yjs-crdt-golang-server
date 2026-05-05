import { sanitizeDocumentId } from "./colors";

export function getUserDocumentId(username: string, documentSlug: string): string {
  return sanitizeDocumentId(`${username}-${documentSlug}`);
}

export function getSubdocumentDocumentId(
  parentDocumentId: string,
  subdocumentSlug: string,
): string {
  return sanitizeDocumentId(`${parentDocumentId}-${subdocumentSlug}`);
}

export function getDocumentRouteBase(
  username: string,
  documentSlug: string,
): string {
  return `/${encodeURIComponent(username)}/${encodeURIComponent(documentSlug)}`;
}
