"use client";

import { useMemo, useState, useEffect } from "react";
import { useSyncStatus } from "@/lib/useSyncStatus";
import { usePresence } from "@/lib/collab/react";

export function SyncStatus({ subdocumentName, compact = false }: { subdocumentName?: string; compact?: boolean }) {
    const activeSubdoc = subdocumentName ?? null;
    const { connectionStatus, hasLocalChanges, isConnected, isSynced } = useSyncStatus();
    const presence = usePresence();
    const [isOnline, setIsOnline] = useState(true);
    const [showPendingState, setShowPendingState] = useState(false);

    useEffect(() => {
        setIsOnline(navigator.onLine);
        const up = () => setIsOnline(true);
        const down = () => setIsOnline(false);
        window.addEventListener("online", up);
        window.addEventListener("offline", down);
        return () => {
            window.removeEventListener("online", up);
            window.removeEventListener("offline", down);
        };
    }, []);

    useEffect(() => {
        if (!isOnline || !isConnected || !hasLocalChanges) {
            setShowPendingState(false);
            return;
        }

        const timeout = window.setTimeout(() => {
            setShowPendingState(true);
        }, 1500);

        return () => window.clearTimeout(timeout);
    }, [hasLocalChanges, isConnected, isOnline]);

    const { colorClass, label } = useMemo(() => {
        if (!isOnline) {
            return { colorClass: "bg-slate-400", label: "Offline" };
        }
        if (connectionStatus === "error") {
            return { colorClass: "bg-red-500 animate-pulse", label: "Reconectando..." };
        }
        if (connectionStatus === "connecting" || connectionStatus === "handshaking") {
            return { colorClass: "bg-amber-500 animate-pulse", label: "Conectando..." };
        }
        if (showPendingState) {
            return { colorClass: "bg-amber-500", label: compact ? "" : "Salvando..." };
        }
        return { colorClass: "bg-green-500", label: "" };
    }, [compact, connectionStatus, isOnline, showPendingState]);

    const showLabel = Boolean(label) && (!compact || !isConnected || !isSynced || !isOnline);

    // Collect collaborators active in the same subdoc context
    const collaborators = presence
        ? Array.from(presence.entries())
            .filter(([, state]: [number, any]) =>
                state?.pagePresence &&
                state.pagePresence.activeSubdoc === activeSubdoc
            )
            .map(([id, state]: [number, any]) => ({
                id,
                name: state.pagePresence.name as string,
                color: state.pagePresence.color as string,
            }))
        : [];

    return (
        <div className={`flex items-center text-xs text-slate-500 dark:text-slate-400 ${compact ? "gap-1.5" : "gap-2"}`}>
            <span className={`h-2 w-2 rounded-full transition-colors duration-300 ${colorClass}`} />
            {showLabel && label && (
                <span className="transition-opacity duration-200">{label}</span>
            )}
            {connectionStatus === "connected" && collaborators.length > 0 && (
                <div className="flex -space-x-1">
                    {collaborators.slice(0, compact ? 3 : 5).map(({ id, name, color }) => (
                        <div
                            key={id}
                            title={name}
                            className={`${compact ? "h-4 w-4" : "h-4 w-4"} rounded-full border border-white dark:border-slate-900 flex items-center justify-center text-white font-bold`}
                            style={{ backgroundColor: color, fontSize: "8px" }}
                        >
                            {name.charAt(0)}
                        </div>
                    ))}
                    {collaborators.length > (compact ? 3 : 5) && (
                        <div
                            className="w-4 h-4 rounded-full border border-white dark:border-slate-900 bg-slate-300 dark:bg-slate-600 flex items-center justify-center text-slate-600 dark:text-slate-300 font-bold"
                            style={{ fontSize: "8px" }}
                        >
                            +{collaborators.length - (compact ? 3 : 5)}
                        </div>
                    )}
                </div>
            )}
        </div>
    );
}
