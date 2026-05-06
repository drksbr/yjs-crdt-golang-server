"use client";

import Link from "next/link";
import { ReactNode } from "react";
import { SyncStatus } from "./SyncStatus";

interface DocumentHeaderProps {
    documentId: string;
    subdocumentName?: string;
    parentHref?: string;
    visibilityMode: "public" | "public-readonly" | "private";
    isReadOnly: boolean;
    onRequestEdit: () => void;
    lastModified?: number;
    lastAccessed?: number;
    actions: ReactNode;
}

function relativeTime(ts: number): string {
    const diff = Date.now() - ts;
    if (diff < 60_000) return "agora mesmo";
    if (diff < 3_600_000) return `${Math.floor(diff / 60_000)} min atrás`;
    if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)} h atrás`;
    return `${Math.floor(diff / 86_400_000)} dias atrás`;
}

export function DocumentHeader({
    documentId,
    subdocumentName,
    parentHref,
    visibilityMode,
    isReadOnly,
    onRequestEdit,
    lastModified,
    lastAccessed,
    actions,
}: DocumentHeaderProps) {
    const primaryLabel = decodeURIComponent(documentId);
    const secondaryLabel = subdocumentName ? decodeURIComponent(subdocumentName) : null;
    const lastActivity = Math.max(lastModified ?? 0, lastAccessed ?? 0);

    return (
        <header className="sticky top-0 z-20 border-b border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-900">
            <div className="mx-auto flex h-16 max-w-screen-2xl items-center gap-3 px-4 sm:px-6">
                <div className="min-w-0 flex flex-1 items-center gap-3">
                    <Link
                        href="/"
                        className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-slate-900 text-sm font-bold tracking-tight text-white transition hover:bg-slate-800 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-slate-200"
                        aria-label="Voltar para a página inicial"
                    >
                        DP
                    </Link>

                    <div className="min-w-0 flex-1">
                        <div className="flex min-w-0 items-center gap-2">
                            <div className="min-w-0 flex items-center gap-1.5 text-sm font-semibold text-slate-900 dark:text-slate-100">
                                <Link
                                    href={parentHref ?? `/${encodeURIComponent(documentId)}`}
                                    className="truncate transition hover:text-slate-700 dark:hover:text-slate-300"
                                    title={primaryLabel}
                                >
                                    {primaryLabel}
                                </Link>
                                {secondaryLabel && (
                                    <>
                                        <span className="shrink-0 text-slate-300 dark:text-slate-600">/</span>
                                        <span className="truncate text-slate-600 dark:text-slate-400" title={secondaryLabel}>
                                            {secondaryLabel}
                                        </span>
                                    </>
                                )}
                            </div>

                            {visibilityMode === "public-readonly" && (
                                <button
                                    type="button"
                                    onClick={onRequestEdit}
                                    className={`inline-flex shrink-0 items-center gap-1 rounded-full border px-2 py-1 text-[11px] font-medium transition ${isReadOnly
                                        ? "border-amber-200 bg-amber-50 text-amber-700 hover:bg-amber-100 dark:border-amber-900/60 dark:bg-amber-900/20 dark:text-amber-300 dark:hover:bg-amber-900/30"
                                        : "border-emerald-200 bg-emerald-50 text-emerald-700 hover:bg-emerald-100 dark:border-emerald-900/60 dark:bg-emerald-900/20 dark:text-emerald-300 dark:hover:bg-emerald-900/30"
                                        }`}
                                    title={isReadOnly ? "Somente leitura — clique para desbloquear edição" : "Edição desbloqueada"}
                                    aria-label={isReadOnly ? "Desbloquear edição" : "Edição desbloqueada"}
                                >
                                    <svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                                        <rect x="3" y="11" width="18" height="11" rx="2" ry="2"></rect>
                                        {isReadOnly ? (
                                            <path d="M7 11V7a5 5 0 0 1 10 0v4"></path>
                                        ) : (
                                            <path d="M7 11V7a5 5 0 0 1 9.9-1"></path>
                                        )}
                                    </svg>
                                    <span className="hidden sm:inline">{isReadOnly ? "Somente leitura" : "Edição liberada"}</span>
                                </button>
                            )}
                        </div>

                        <div className="mt-0.5 flex min-w-0 items-center gap-3 text-xs text-slate-500 dark:text-slate-400">
                            <SyncStatus subdocumentName={subdocumentName} compact />
                            {lastActivity > 0 && (
                                <span className="hidden truncate sm:inline">
                                    Última atividade {relativeTime(lastActivity)}
                                </span>
                            )}
                        </div>
                    </div>
                </div>

                <div className="shrink-0">{actions}</div>
            </div>
        </header>
    );
}
