"use client";

import { useMap } from "@/lib/collab/react";
import { useCallback, useRef } from "react";
import { getMetaFieldKey } from "@/lib/noteStateAdapters";

/**
 * Reads and writes document-level metadata stored in Y.Map('meta').
 * Values are synced in real time across all collaborators via Y-Sweet.
 *
 * - lastModified: updated whenever any text in the document changes
 * - lastAccessed: updated when a user opens the document
 */
function readMetaValue(
  meta: ReturnType<typeof useMap>,
  field: "lastModified" | "lastAccessed",
  subdocumentName?: string,
) {
  const scopedValue = meta?.get(getMetaFieldKey(field, subdocumentName)) as number | undefined;
  if (typeof scopedValue === "number") {
    return scopedValue;
  }

  if (!subdocumentName) {
    return meta?.get(field) as number | undefined;
  }

  return undefined;
}

export function useDocumentMeta(subdocumentName?: string) {
  const meta = useMap("meta");

  const updateLastModified = useCallback(() => {
    const now = Date.now();
    meta?.set(getMetaFieldKey("lastModified", subdocumentName), now);
    if (!subdocumentName) {
      meta?.set("lastModified", now);
    }
  }, [meta, subdocumentName]);

  const updateLastAccessed = useCallback(() => {
    const now = Date.now();
    meta?.set(getMetaFieldKey("lastAccessed", subdocumentName), now);
    if (!subdocumentName) {
      meta?.set("lastAccessed", now);
    }
  }, [meta, subdocumentName]);

  return {
    lastModified: readMetaValue(meta, "lastModified", subdocumentName),
    lastAccessed: readMetaValue(meta, "lastAccessed", subdocumentName),
    updateLastModified,
    updateLastAccessed,
  };
}

export function useDebouncedLastModified(subdocumentName?: string, intervalMs = 5000) {
  const { updateLastModified } = useDocumentMeta(subdocumentName);
  const lastModifiedTimeRef = useRef(0);

  return useCallback(() => {
    const now = Date.now();
    if (now - lastModifiedTimeRef.current < intervalMs) {
      return;
    }

    lastModifiedTimeRef.current = now;
    updateLastModified();
  }, [intervalMs, updateLastModified]);
}
