"use client";

import { useEffect, useState } from "react";
import { useYDoc } from "@/lib/collab/react";

type PersistenceStatus = "loading" | "ready";

function isOfflinePersistenceEnabled(): boolean {
  const value = import.meta.env.VITE_DONTPAD_OFFLINE_PERSISTENCE;
  return value === "1" || value === "true";
}

/**
 * Persiste o Y.Doc localmente via IndexedDB usando y-indexeddb.
 *
 * Desabilitado por padrão no DontPadBR2: o backend Go/Postgres é a fonte
 * autoritativa, e updates antigos do IndexedDB podem ser reintroduzidos no
 * documento remoto durante o handshake CRDT.
 *
 * Para testes explícitos de offline-first, defina
 * VITE_DONTPAD_OFFLINE_PERSISTENCE=1.
 *
 * - A chave no IndexedDB é `dp_doc_${docId}` para evitar colisões entre documentos.
 */
export function useOfflinePersistence(docId: string): PersistenceStatus {
  const ydoc = useYDoc();
  const enabled = isOfflinePersistenceEnabled();
  const [status, setStatus] = useState<PersistenceStatus>(
    enabled ? "loading" : "ready",
  );

  useEffect(() => {
    if (!enabled) {
      setStatus("ready");
      return;
    }
    if (!ydoc || typeof window === "undefined") return;

    // y-indexeddb é browser-only, importar de forma lazy
    let persistence: any;
    let cancelled = false;
    import("y-indexeddb").then(({ IndexeddbPersistence }) => {
      if (cancelled) return;
      persistence = new IndexeddbPersistence(`dp_doc_${docId}`, ydoc);
      persistence.on("synced", () => setStatus("ready"));
    });

    return () => {
      cancelled = true;
      persistence?.destroy();
    };
  }, [enabled, ydoc, docId]);

  return status;
}
