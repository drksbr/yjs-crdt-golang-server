"use client";

import { useConnectionStatus, useYDoc, useYjsProvider } from "@/lib/collab/react";
import { useCallback, useEffect, useRef } from "react";
import * as Y from "yjs";

const FLUSH_DEBOUNCE_MS = 15000;
const MAX_KEEPALIVE_BYTES = 60 * 1024;
const MAX_PENDING_UPDATES_BEFORE_COMPACT = 50;
const MAX_FLUSH_BATCH_BYTES = 512 * 1024;
const MAX_PENDING_BYTES_BEFORE_EAGER_FLUSH = 512 * 1024;
const MAX_INCREMENTAL_FLUSH_BYTES = 4 * 1024 * 1024;

interface DocumentSyncLifecycleProps {
  documentId: string;
}

function getTotalBytes(updates: Uint8Array[]): number {
  return updates.reduce((total, update) => total + update.byteLength, 0);
}

function splitUpdatesIntoBatches(
  updates: Uint8Array[],
  maxBatchBytes: number,
): Uint8Array[][] {
  const batches: Uint8Array[][] = [];
  let currentBatch: Uint8Array[] = [];
  let currentBatchBytes = 0;

  for (const update of updates) {
    const nextBytes = currentBatchBytes + update.byteLength;
    if (currentBatch.length > 0 && nextBytes > maxBatchBytes) {
      batches.push(currentBatch);
      currentBatch = [];
      currentBatchBytes = 0;
    }

    currentBatch.push(update);
    currentBatchBytes += update.byteLength;
  }

  if (currentBatch.length > 0) {
    batches.push(currentBatch);
  }

  return batches;
}

function toArrayBuffer(update: Uint8Array): ArrayBuffer {
  const buffer = new ArrayBuffer(update.byteLength);
  new Uint8Array(buffer).set(update);
  return buffer;
}

function shouldPersistSnapshot(updates: Uint8Array[]): boolean {
  if (updates.length === 0) {
    return true;
  }
  return (
    getTotalBytes(updates) >= MAX_INCREMENTAL_FLUSH_BYTES ||
    updates.some((update) => update.byteLength >= MAX_FLUSH_BATCH_BYTES)
  );
}

async function persistUpdate(
  url: string,
  update: Uint8Array,
  reason: string,
  preferKeepalive: boolean,
): Promise<boolean> {
  const canUseKeepalive = preferKeepalive && update.byteLength <= MAX_KEEPALIVE_BYTES;
  const body = toArrayBuffer(update);

  if (
    canUseKeepalive &&
    typeof navigator !== "undefined" &&
    typeof navigator.sendBeacon === "function"
  ) {
    const sent = navigator.sendBeacon(
      url,
      new Blob([body], { type: "application/octet-stream" }),
    );
    if (sent) {
      return true;
    }
  }

  try {
    const response = await fetch(url, {
      method: "POST",
      headers: {
        "Content-Type": "application/octet-stream",
        "X-Sync-Reason": reason,
      },
      body,
      cache: "no-store",
      credentials: "same-origin",
      keepalive: canUseKeepalive,
    });

    return response.ok;
  } catch {
    return false;
  }
}

export function DocumentSyncLifecycle({
  documentId,
}: DocumentSyncLifecycleProps) {
  const doc = useYDoc();
  const provider = useYjsProvider();
  const connectionStatus = useConnectionStatus();
  const flushTimerRef = useRef<number | null>(null);
  const isFlushingRef = useRef(false);
  const pendingUpdatesRef = useRef<Uint8Array[]>([]);

  const cancelScheduledFlush = useCallback(() => {
    if (flushTimerRef.current !== null) {
      window.clearTimeout(flushTimerRef.current);
      flushTimerRef.current = null;
    }
  }, []);

  const flushNow = useCallback(
    async (reason: string, preferKeepalive = false) => {
      if (connectionStatus === "offline") {
        return false;
      }

      if (
        (!provider.hasLocalChanges && pendingUpdatesRef.current.length === 0) ||
        isFlushingRef.current
      ) {
        return false;
      }

      isFlushingRef.current = true;
      cancelScheduledFlush();

      try {
        const pendingUpdates = pendingUpdatesRef.current;
        pendingUpdatesRef.current = [];
        const useSnapshot = shouldPersistSnapshot(pendingUpdates);
        const updatesToPersist = useSnapshot
          ? [Y.encodeStateAsUpdateV2(doc)]
          : pendingUpdates;
        const batches = useSnapshot
          ? [updatesToPersist]
          : splitUpdatesIntoBatches(updatesToPersist, MAX_FLUSH_BATCH_BYTES);

        if (batches.length === 0) {
          return false;
        }

        const flushUrl = `/api/documents/${encodeURIComponent(documentId)}/flush`;
        let flushedUpdateCount = 0;

        for (let index = 0; index < batches.length; index += 1) {
          const batch = batches[index];
          const update = batch.length === 1 ? batch[0] : Y.mergeUpdatesV2(batch);
          if (update.byteLength === 0) {
            flushedUpdateCount += batch.length;
            continue;
          }

          const persisted = await persistUpdate(
            flushUrl,
            update,
            batches.length > 1 ? `${reason}-part-${index + 1}` : reason,
            preferKeepalive,
          );

          if (!persisted) {
            if (!useSnapshot) {
              const snapshot = Y.encodeStateAsUpdateV2(doc);
              const snapshotPersisted =
                snapshot.byteLength > 0 &&
                (await persistUpdate(flushUrl, snapshot, `${reason}-snapshot`, preferKeepalive));
              if (snapshotPersisted) {
                return true;
              }
            }

            pendingUpdatesRef.current = pendingUpdates
              .slice(flushedUpdateCount)
              .concat(pendingUpdatesRef.current);
            return false;
          }

          flushedUpdateCount += batch.length;
        }

        return true;
      } finally {
        isFlushingRef.current = false;
      }
    },
    [cancelScheduledFlush, connectionStatus, doc, documentId, provider],
  );

  const scheduleFlush = useCallback(
    (reason: string) => {
      if (
        connectionStatus === "offline" ||
        (!provider.hasLocalChanges && pendingUpdatesRef.current.length === 0)
      ) {
        return;
      }

      cancelScheduledFlush();
      flushTimerRef.current = window.setTimeout(() => {
        void flushNow(reason);
      }, FLUSH_DEBOUNCE_MS);
    },
    [cancelScheduledFlush, connectionStatus, flushNow, provider],
  );

  useEffect(() => {
    const handleUpdate = (
      update: Uint8Array,
      _origin: unknown,
      _doc: Y.Doc,
      transaction: Y.Transaction,
    ) => {
      if (!transaction.local) {
        return;
      }

      pendingUpdatesRef.current.push(new Uint8Array(update));
      if (pendingUpdatesRef.current.length >= MAX_PENDING_UPDATES_BEFORE_COMPACT) {
        pendingUpdatesRef.current = [Y.mergeUpdatesV2(pendingUpdatesRef.current)];
      }

      if (
        connectionStatus !== "offline" &&
        getTotalBytes(pendingUpdatesRef.current) >= MAX_PENDING_BYTES_BEFORE_EAGER_FLUSH
      ) {
        void flushNow("queue-pressure");
        return;
      }

      if (provider.hasLocalChanges || pendingUpdatesRef.current.length > 0) {
        scheduleFlush("debounced-update");
      }
    };

    doc.on("updateV2", handleUpdate);
    return () => {
      doc.off("updateV2", handleUpdate);
    };
  }, [connectionStatus, doc, flushNow, provider, scheduleFlush]);

  useEffect(() => {
    if (
      connectionStatus !== "offline" &&
      (provider.hasLocalChanges || pendingUpdatesRef.current.length > 0)
    ) {
      scheduleFlush("connection-state-change");
      return;
    }

    if (pendingUpdatesRef.current.length === 0) {
      cancelScheduledFlush();
    }
  }, [cancelScheduledFlush, connectionStatus, provider, scheduleFlush]);

  useEffect(() => {
    const handleVisibilityChange = () => {
      if (document.visibilityState === "hidden") {
        void flushNow("visibility-hidden", true);
      }
    };

    const handlePageHide = () => {
      void flushNow("pagehide", true);
    };

    document.addEventListener("visibilitychange", handleVisibilityChange);
    window.addEventListener("pagehide", handlePageHide);

    return () => {
      document.removeEventListener("visibilitychange", handleVisibilityChange);
      window.removeEventListener("pagehide", handlePageHide);
    };
  }, [flushNow]);

  useEffect(() => cancelScheduledFlush, [cancelScheduledFlush]);

  return null;
}
