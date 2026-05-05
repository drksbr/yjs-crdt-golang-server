"use client";

import { useMap } from "@/lib/collab/react";
import { useRouter } from "next/navigation";
import { ReactNode, useEffect, useMemo, useRef, useState } from "react";
import type * as Y from "yjs";
import type { DocumentPanelId } from "./DocumentActions";
import { getSubdocumentHref } from "@/lib/documentRouting";
import { SUBDOC_TYPE_CONFIGS, SubdocType } from "@/lib/subdocTypes";

interface DocumentCommandPaletteProps {
    open: boolean;
    onClose: () => void;
    documentId: string;
    subdocumentName?: string;
    parentHref?: string;
    subdocumentsMap?: Y.Map<unknown>;
    onTogglePanel: (panel: DocumentPanelId) => void;
    onCopyLink: () => void;
    onOpenSettings: () => void;
}

interface PaletteEntry {
    id: string;
    label: string;
    description: string;
    group: "Ações" | "Subdocumentos";
    keywords: string;
    icon: ReactNode;
    run: () => void;
}

function SubdocIcon({ type }: { type?: SubdocType }) {
    switch (type) {
        case "markdown":
            return (
                <svg xmlns="http://www.w3.org/2000/svg" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <path d="M4 19h16" /><path d="M4 15h3l3-8 3 8h3" /><path d="M15 15v-4l2 2 2-2v4" />
                </svg>
            );
        case "checklist":
            return (
                <svg xmlns="http://www.w3.org/2000/svg" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <path d="M9 11l3 3L22 4" /><path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11" />
                </svg>
            );
        case "kanban":
            return (
                <svg xmlns="http://www.w3.org/2000/svg" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <rect x="3" y="3" width="5" height="14" rx="1" /><rect x="10" y="3" width="5" height="9" rx="1" /><rect x="17" y="3" width="5" height="11" rx="1" />
                </svg>
            );
        case "desenho":
            return (
                <svg xmlns="http://www.w3.org/2000/svg" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <path d="M12 20h9" /><path d="M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4L16.5 3.5z" />
                </svg>
            );
        default:
            return (
                <svg xmlns="http://www.w3.org/2000/svg" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" /><polyline points="14 2 14 8 20 8" />
                </svg>
            );
    }
}

export function DocumentCommandPalette({
    open,
    onClose,
    documentId,
    subdocumentName,
    parentHref,
    subdocumentsMap,
    onTogglePanel,
    onCopyLink,
    onOpenSettings,
}: DocumentCommandPaletteProps) {
    const router = useRouter();
    const inputRef = useRef<HTMLInputElement | null>(null);
    const [query, setQuery] = useState("");
    const [selectedIndex, setSelectedIndex] = useState(0);
    const localSubdocsMap = useMap("subdocuments");
    const subdocsMap = subdocumentsMap ?? localSubdocsMap;

    const subdocuments = subdocsMap
        ? Array.from(subdocsMap.entries()).map(([id, entry]: [string, any]) => ({
            id,
            name: entry?.name || id,
            type: (entry?.type as SubdocType) || "texto",
        }))
        : [];

    useEffect(() => {
        if (!open) {
            setQuery("");
            setSelectedIndex(0);
            return;
        }

        const timeout = setTimeout(() => inputRef.current?.focus(), 10);
        return () => clearTimeout(timeout);
    }, [open]);

    const entries = useMemo(() => {
        const actionEntries: PaletteEntry[] = [
            {
                id: "action-subdocs",
                label: "Abrir subdocumentos",
                description: "Mostra o painel com a lista completa de subnotas.",
                group: "Ações",
                keywords: "subdocs subdocumentos lista painel",
                icon: <SubdocIcon type="texto" />,
                run: () => onTogglePanel("subdocs"),
            },
            {
                id: "action-files",
                label: "Abrir arquivos",
                description: "Mostra anexos e uploads da nota atual.",
                group: "Ações",
                keywords: "arquivos anexos upload painel",
                icon: (
                    <svg xmlns="http://www.w3.org/2000/svg" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                        <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path>
                        <polyline points="14 2 14 8 20 8"></polyline>
                    </svg>
                ),
                run: () => onTogglePanel("files"),
            },
            {
                id: "action-audio",
                label: "Abrir áudio",
                description: "Mostra gravações e notas de áudio da nota atual.",
                group: "Ações",
                keywords: "audio gravação gravacao som notas de audio",
                icon: (
                    <svg xmlns="http://www.w3.org/2000/svg" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                        <path d="M12 2a3 3 0 0 0-3 3v7a3 3 0 0 0 6 0V5a3 3 0 0 0-3-3Z"></path>
                        <path d="M19 10v2a7 7 0 0 1-14 0v-2"></path>
                        <line x1="12" x2="12" y1="19" y2="22"></line>
                    </svg>
                ),
                run: () => onTogglePanel("audio"),
            },
            {
                id: "action-share",
                label: "Copiar link da nota",
                description: "Copia a URL atual para compartilhar.",
                group: "Ações",
                keywords: "compartilhar copiar link url",
                icon: (
                    <svg xmlns="http://www.w3.org/2000/svg" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                        <circle cx="18" cy="5" r="3"></circle>
                        <circle cx="6" cy="12" r="3"></circle>
                        <circle cx="18" cy="19" r="3"></circle>
                        <line x1="8.59" y1="13.51" x2="15.42" y2="17.49"></line>
                        <line x1="15.41" y1="6.51" x2="8.59" y2="10.49"></line>
                    </svg>
                ),
                run: () => onCopyLink(),
            },
            {
                id: "action-settings",
                label: "Abrir configurações",
                description: "Abre permissões, versões e opções do documento.",
                group: "Ações",
                keywords: "configurações configuracoes security versoes pin",
                icon: (
                    <svg xmlns="http://www.w3.org/2000/svg" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                        <path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"></path>
                        <circle cx="12" cy="12" r="3"></circle>
                    </svg>
                ),
                run: () => onOpenSettings(),
            },
        ];

        if (subdocumentName) {
            actionEntries.unshift({
                id: "action-main-note",
                label: "Ir para a nota principal",
                description: "Volta para o documento principal.",
                group: "Ações",
                keywords: "nota principal documento raiz voltar",
                icon: (
                    <svg xmlns="http://www.w3.org/2000/svg" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                        <path d="M15 18l-6-6 6-6" />
                    </svg>
                ),
                run: () => router.push(parentHref ?? `/${encodeURIComponent(documentId)}`),
            });
        }

        const subdocEntries: PaletteEntry[] = subdocuments
            .filter((entry) => Boolean(entry.name))
            .map((entry) => {
                const typeLabel = SUBDOC_TYPE_CONFIGS.find((cfg) => cfg.type === entry.type)?.label || "Texto";
                return {
                    id: `subdoc-${entry.id}`,
                    label: entry.name,
                    description: typeLabel,
                    group: "Subdocumentos" as const,
                    keywords: `${entry.name} ${typeLabel} subdocumento`,
                    icon: <SubdocIcon type={entry.type} />,
                    run: () => router.push(getSubdocumentHref(parentHref ?? `/${encodeURIComponent(documentId)}`, entry.id)),
                };
            });

        return actionEntries.concat(subdocEntries);
    }, [documentId, onCopyLink, onOpenSettings, onTogglePanel, parentHref, router, subdocumentName, subdocuments]);

    const filteredEntries = entries.filter((entry) => {
        const normalizedQuery = query.trim().toLowerCase();
        if (!normalizedQuery) return true;
        return `${entry.label} ${entry.description} ${entry.keywords}`.toLowerCase().includes(normalizedQuery);
    });

    useEffect(() => {
        setSelectedIndex(0);
    }, [query, open]);

    useEffect(() => {
        if (!open) return;

        const handleKeydown = (event: KeyboardEvent) => {
            if (event.key === "Escape") {
                event.preventDefault();
                onClose();
            }

            if (event.key === "ArrowDown") {
                event.preventDefault();
                setSelectedIndex((current) => Math.min(current + 1, Math.max(filteredEntries.length - 1, 0)));
            }

            if (event.key === "ArrowUp") {
                event.preventDefault();
                setSelectedIndex((current) => Math.max(current - 1, 0));
            }

            if (event.key === "Enter") {
                const selectedEntry = filteredEntries[selectedIndex];
                if (!selectedEntry) return;
                event.preventDefault();
                selectedEntry.run();
                onClose();
            }
        };

        document.addEventListener("keydown", handleKeydown);
        return () => document.removeEventListener("keydown", handleKeydown);
    }, [filteredEntries, onClose, open, selectedIndex]);

    if (!open) return null;

    return (
        <div className="fixed inset-0 z-50 flex items-start justify-center bg-slate-950/45 px-4 py-[12vh]" onClick={onClose}>
            <div
                className="w-full max-w-2xl overflow-hidden rounded-[1.75rem] border border-slate-200 bg-white shadow-2xl dark:border-slate-700 dark:bg-slate-900"
                onClick={(event) => event.stopPropagation()}
                role="dialog"
                aria-modal="true"
                aria-label="Paleta de ações e subdocumentos"
            >
                <div className="border-b border-slate-200 px-5 py-4 dark:border-slate-800">
                    <div className="flex items-center gap-3 rounded-2xl border border-slate-200 bg-slate-50 px-4 py-3 dark:border-slate-700 dark:bg-slate-800">
                        <svg xmlns="http://www.w3.org/2000/svg" width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="text-slate-400 dark:text-slate-500">
                            <circle cx="11" cy="11" r="8" />
                            <path d="m21 21-4.3-4.3" />
                        </svg>
                        <input
                            ref={inputRef}
                            type="text"
                            value={query}
                            onChange={(event) => setQuery(event.target.value)}
                            placeholder="Buscar subdocumentos e ações..."
                            className="min-w-0 flex-1 bg-transparent text-sm text-slate-950 placeholder-slate-400 focus:outline-none dark:text-slate-100 dark:placeholder-slate-500"
                        />
                        <span className="rounded-full border border-slate-200 bg-white px-2 py-0.5 text-[11px] font-medium text-slate-500 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-400">
                            Esc
                        </span>
                    </div>
                </div>

                <div className="max-h-[60vh] overflow-y-auto p-3">
                    {filteredEntries.length === 0 ? (
                        <div className="flex flex-col items-center justify-center px-6 py-12 text-center">
                            <p className="text-sm font-medium text-slate-700 dark:text-slate-300">Nada encontrado.</p>
                            <p className="mt-2 text-sm text-slate-500 dark:text-slate-400">Tente outro termo ou use as ações visíveis da nota.</p>
                        </div>
                    ) : (
                        <div className="space-y-1">
                            {filteredEntries.map((entry, index) => (
                                <button
                                    key={entry.id}
                                    type="button"
                                    onClick={() => {
                                        entry.run();
                                        onClose();
                                    }}
                                    className={`flex w-full items-start gap-3 rounded-2xl px-4 py-3 text-left transition ${index === selectedIndex
                                        ? "bg-slate-100 text-slate-950 dark:bg-slate-800 dark:text-slate-100"
                                        : "text-slate-700 hover:bg-slate-50 dark:text-slate-300 dark:hover:bg-slate-800/70"
                                        }`}
                                >
                                    <span className="mt-0.5 text-slate-500 dark:text-slate-400">{entry.icon}</span>
                                    <span className="min-w-0 flex-1">
                                        <span className="flex items-center gap-2">
                                            <span className="truncate text-sm font-medium">{entry.label}</span>
                                            <span className="rounded-full border border-slate-200 bg-white px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.14em] text-slate-500 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-400">
                                                {entry.group}
                                            </span>
                                        </span>
                                        <span className="mt-1 block text-xs text-slate-500 dark:text-slate-400">
                                            {entry.description}
                                        </span>
                                    </span>
                                </button>
                            ))}
                        </div>
                    )}
                </div>
            </div>
        </div>
    );
}
