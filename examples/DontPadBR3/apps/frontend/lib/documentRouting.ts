import type * as Y from "yjs";
import { sanitizeDocumentId } from "@/lib/colors";
import { SubdocType } from "@/lib/subdocTypes";

export interface SubdocumentEntry {
  id: string;
  documentId?: string;
  name: string;
  createdAt: number;
  type: SubdocType;
}

export type DocumentRoute =
  | {
      kind: "document";
      documentId: string;
      displayDocumentId: string;
      parentHref: string;
    }
  | {
      kind: "subdocument";
      parentDocumentId: string;
      displayDocumentId: string;
      parentHref: string;
      subdocumentSlug: string;
      subdocumentName: string;
    };

function decodeSegment(segment: string): string {
  try {
    return decodeURIComponent(segment);
  } catch {
    return segment;
  }
}

function nonEmptyDocumentId(value: string): string {
  return sanitizeDocumentId(value) || "documento";
}

function encodePathSegment(segment: string): string {
  return encodeURIComponent(segment);
}

export function getSubdocumentDocumentId(
  parentDocumentId: string,
  subdocumentSlug: string,
): string {
  return nonEmptyDocumentId(`${parentDocumentId}-subdoc-${subdocumentSlug}`);
}

export function getSubdocumentHref(parentHref: string, subdocumentSlug: string): string {
  return `${parentHref.replace(/\/+$/, "")}/${encodePathSegment(subdocumentSlug)}`;
}

export function resolveDocumentRoute(rawSegments: string[]): DocumentRoute {
  const segments = rawSegments.map(decodeSegment).filter(Boolean);
  const encodedSegments = segments.map(encodePathSegment);

  if (segments.length >= 2) {
    const legacySeparatorOffset = segments[1] === "~" ? 2 : 1;
    const parentDocumentId = nonEmptyDocumentId(segments[0]);
    const subdocumentSegments = segments.slice(legacySeparatorOffset);
    return {
      kind: "subdocument",
      parentDocumentId,
      displayDocumentId: segments[0],
      parentHref: `/${encodedSegments[0]}`,
      subdocumentSlug: nonEmptyDocumentId(subdocumentSegments.join("-")),
      subdocumentName: subdocumentSegments.join("/"),
    };
  }

  const documentId = nonEmptyDocumentId(segments[0] || "documento");
  return {
    kind: "document",
    documentId,
    displayDocumentId: segments[0] || documentId,
    parentHref: `/${encodePathSegment(segments[0] || documentId)}`,
  };
}

export function resolveSubdocumentEntry(
  subdocumentsMap: Y.Map<unknown> | null | undefined,
  slug: string,
  fallbackName = slug,
): SubdocumentEntry {
  const normalizedSlug = nonEmptyDocumentId(slug);

  if (subdocumentsMap) {
    const direct = subdocumentsMap.get(normalizedSlug) as Partial<SubdocumentEntry> | undefined;
    if (direct) {
      return {
        id: normalizedSlug,
        documentId: direct.documentId,
        name: direct.name || normalizedSlug,
        createdAt: direct.createdAt || Date.now(),
        type: direct.type || "texto",
      };
    }

    for (const [id, rawEntry] of Array.from(subdocumentsMap.entries())) {
      const entry = rawEntry as Partial<SubdocumentEntry> | undefined;
      if (!entry) continue;
      const entryName = entry.name || id;
      if (nonEmptyDocumentId(entryName) === normalizedSlug || entryName === slug) {
        return {
          id,
          documentId: entry.documentId,
          name: entryName,
          createdAt: entry.createdAt || Date.now(),
          type: entry.type || "texto",
        };
      }
    }
  }

  return {
    id: normalizedSlug,
    name: fallbackName,
    createdAt: Date.now(),
    type: "texto",
  };
}
