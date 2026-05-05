"use client";

import { useConnectionStatus, useHasLocalChanges } from "@/lib/collab/react";

export function useSyncStatus() {
  const connectionStatus = useConnectionStatus();
  const hasLocalChanges = useHasLocalChanges();

  const isConnected = connectionStatus === "connected";
  const isSynced = isConnected && !hasLocalChanges;

  return {
    connectionStatus,
    hasLocalChanges,
    isConnected,
    isSynced,
  };
}
