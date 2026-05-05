"use client";

import { useEffect, useRef, useCallback } from "react";
import { useAwareness, usePresence } from "@/lib/collab/react";
import { getSessionIdentity } from "@/lib/session";

interface PagePresence {
    name: string;
    color: string;
    activeSubdoc: string | null;
    cursor: { x: number; y: number } | null;
}

/**
 * Renders floating mouse cursors for all other connected collaborators.
 * Also sets up the current user's own presence (name + color) in the awareness state.
 * Must be rendered inside a Y-Sweet provider context.
 */
export function CollaboratorCursors({ subdocumentName }: { subdocumentName?: string }) {
    const activeSubdoc = subdocumentName ?? null;
    const awareness = useAwareness();
    const presence = usePresence();
    const sessionIdRef = useRef<string | null>(null);
    const lastUpdateRef = useRef(0);

    // Register own presence on mount using a stable session ID
    useEffect(() => {
        if (!awareness) return;

        const { name, color } = getSessionIdentity();
        sessionIdRef.current = name;

        // pagePresence: used for floating mouse cursors (CollaboratorCursors)
        awareness.setLocalStateField("pagePresence", { name, color, activeSubdoc, cursor: null });
        // user: used by QuillBinding for text cursors inside the editor
        awareness.setLocalStateField("user", { name, color });

        return () => {
            awareness.setLocalStateField("pagePresence", null);
            awareness.setLocalStateField("user", null);
        };
    }, [awareness, activeSubdoc]);

    // Track mouse position and broadcast to other collaborators (max 20fps)
    const handleMouseMove = useCallback((e: MouseEvent) => {
        if (!awareness) return;
        const now = Date.now();
        if (now - lastUpdateRef.current < 50) return;
        lastUpdateRef.current = now;

        const current = awareness.getLocalState()?.pagePresence as PagePresence | undefined;
        if (!current) return;

        awareness.setLocalStateField("pagePresence", {
            ...current,
            cursor: { x: e.clientX, y: e.clientY },
        });
    }, [awareness]);

    const handleMouseLeave = useCallback(() => {
        if (!awareness) return;
        const current = awareness.getLocalState()?.pagePresence as PagePresence | undefined;
        if (!current) return;
        awareness.setLocalStateField("pagePresence", { ...current, cursor: null });
    }, [awareness]);

    useEffect(() => {
        window.addEventListener("mousemove", handleMouseMove);
        document.documentElement.addEventListener("mouseleave", handleMouseLeave);
        return () => {
            window.removeEventListener("mousemove", handleMouseMove);
            document.documentElement.removeEventListener("mouseleave", handleMouseLeave);
        };
    }, [handleMouseMove, handleMouseLeave]);

    // Collect other users with visible cursors in the same subdoc context (exclude self)
    const myClientId = awareness?.clientID;
    const others = presence
        ? Array.from(presence.entries())
            .filter(([id, state]: [number, any]) =>
                id !== myClientId &&
                (state?.pagePresence as PagePresence | undefined)?.cursor != null &&
                (state?.pagePresence as PagePresence | undefined)?.activeSubdoc === activeSubdoc
            )
            .map(([id, state]: [number, any]) => ({
                id,
                ...(state.pagePresence as PagePresence),
            }))
        : [];

    if (others.length === 0) return null;

    return (
        <div className="pointer-events-none fixed inset-0 z-50 overflow-hidden">
            {others.map(({ id, name, color, cursor }) =>
                cursor && (
                    <div
                        key={id}
                        className="absolute transition-[left,top] duration-75 ease-linear"
                        style={{ left: cursor.x, top: cursor.y, transform: "translate(-2px, -2px)" }}
                    >
                        {/* Arrow cursor */}
                        <svg width="16" height="20" viewBox="0 0 16 20" fill="none">
                            <path
                                d="M0 0L0 16L4 12L7 18L9 17L6 11L11 11Z"
                                fill={color}
                                stroke="white"
                                strokeWidth="1.5"
                                strokeLinejoin="round"
                            />
                        </svg>
                        {/* Name badge */}
                        <div
                            className="absolute left-4 top-0 px-2 py-0.5 rounded-full text-white text-xs font-medium whitespace-nowrap shadow"
                            style={{ backgroundColor: color }}
                        >
                            {name}
                        </div>
                    </div>
                )
            )}
        </div>
    );
}
