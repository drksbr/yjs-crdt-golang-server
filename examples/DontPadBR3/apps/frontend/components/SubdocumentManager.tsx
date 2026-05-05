"use client";

import { useState, useRef, useEffect } from "react";
import Link from "next/link";
import { useMap } from "@/lib/collab/react";
import type * as Y from "yjs";
import { ConfirmDeleteModal } from "./ConfirmDeleteModal";
import { sanitizeDocumentId } from "@/lib/colors";
import { getSubdocumentHref } from "@/lib/documentRouting";
import { SubdocType, SUBDOC_TYPE_CONFIGS } from "@/lib/subdocTypes";
import { useDocumentSecurity } from "@/lib/documentSecurityContext";

interface SubdocumentManagerProps {
    documentId: string;
    parentHref?: string;
    subdocumentsMap?: Y.Map<unknown>;
    embedded?: boolean;
}

interface SubdocumentEntry {
    id: string;
    documentId?: string;
    name: string;
    createdAt: number;
    type?: SubdocType;
}

// SVG icons per type
function SubdocIcon({ type, size = 16 }: { type?: SubdocType; size?: number }) {
    switch (type) {
        case 'markdown':
            return (
                <svg xmlns="http://www.w3.org/2000/svg" width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <path d="M4 19h16" /><path d="M4 15h3l3-8 3 8h3" /><path d="M15 15v-4l2 2 2-2v4" />
                </svg>
            )
        case 'checklist':
            return (
                <svg xmlns="http://www.w3.org/2000/svg" width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <path d="M9 11l3 3L22 4" /><path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11" />
                </svg>
            )
        case 'kanban':
            return (
                <svg xmlns="http://www.w3.org/2000/svg" width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <rect x="3" y="3" width="5" height="14" rx="1" /><rect x="10" y="3" width="5" height="9" rx="1" /><rect x="17" y="3" width="5" height="11" rx="1" />
                </svg>
            )
        case 'desenho':
            return (
                <svg xmlns="http://www.w3.org/2000/svg" width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <path d="M12 20h9" /><path d="M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4L16.5 3.5z" />
                </svg>
            )
        default: // texto
            return (
                <svg xmlns="http://www.w3.org/2000/svg" width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" /><polyline points="14 2 14 8 20 8" /><line x1="16" y1="13" x2="8" y2="13" /><line x1="16" y1="17" x2="8" y2="17" /><line x1="10" y1="9" x2="8" y2="9" />
                </svg>
            )
    }
}

export function SubdocumentManager({
    documentId,
    parentHref,
    subdocumentsMap,
    embedded = false,
}: SubdocumentManagerProps) {
    const [newSubdocName, setNewSubdocName] = useState("");
    const [searchQuery, setSearchQuery] = useState("");
    const [sortDirection, setSortDirection] = useState<"asc" | "desc">("asc");
    const [isLoading, setIsLoading] = useState(false);
    const [showTypeDropdown, setShowTypeDropdown] = useState(false);
    const dropdownRef = useRef<HTMLDivElement>(null);
    const [deleteModal, setDeleteModal] = useState<{ isOpen: boolean; subdocId: string; subdocName: string }>({
        isOpen: false,
        subdocId: "",
        subdocName: "",
    });
    const [isDeleting, setIsDeleting] = useState(false);
    const { isReadOnly } = useDocumentSecurity();

    // Close dropdown on outside click
    useEffect(() => {
        const handler = (e: MouseEvent) => {
            if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
                setShowTypeDropdown(false);
            }
        };
        document.addEventListener("mousedown", handler);
        return () => document.removeEventListener("mousedown", handler);
    }, []);

    // Get subdocuments map from Y-Sweet
    // The Y.Map 'subdocuments' contains all subdocument metadata
    const localSubdocsMap = useMap("subdocuments");
    const subdocsMap = subdocumentsMap ?? localSubdocsMap;

    // Convert Y.Map to array for display
    const subdocs: SubdocumentEntry[] = subdocsMap
        ? Array.from(subdocsMap.entries()).map(([id, data]: [string, any]) => ({
            id,
            documentId: data.documentId,
            name: data.name || id,
            createdAt: data.createdAt || Date.now(),
            type: (data.type as SubdocType) || 'texto',
        }))
        : [];

    const normalizedSearch = searchQuery.trim().toLowerCase();
    const visibleSubdocs = subdocs
        .filter((subdoc) => {
            if (!normalizedSearch) return true;
            const typeLabel = SUBDOC_TYPE_CONFIGS.find((cfg) => cfg.type === subdoc.type)?.label || "Texto";
            return `${subdoc.name} ${typeLabel}`.toLowerCase().includes(normalizedSearch);
        })
        .sort((a, b) => {
            const comparison = a.name.localeCompare(b.name, "pt-BR", { sensitivity: "base" });
            return sortDirection === "asc" ? comparison : comparison * -1;
        });

    const handleCreateSubdoc = async (type: SubdocType) => {
        if (isReadOnly) return;
        if (!newSubdocName.trim()) return;
        setShowTypeDropdown(false);
        setIsLoading(true);

        try {
            const trimmedName = newSubdocName.trim();
            const sanitizedId = sanitizeDocumentId(trimmedName);
            if (!sanitizedId) return;

            if (subdocsMap) {
                const response = await fetch(`/api/documents/${encodeURIComponent(documentId)}/subdocuments`, {
                    method: "POST",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify({
                        id: sanitizedId,
                        slug: sanitizedId,
                        name: trimmedName,
                        type,
                    }),
                });
                if (!response.ok) {
                    throw new Error("Failed to create subdocument metadata");
                }
                const payload = await response.json();
                const subdocument = payload.subdocument ?? {};
                subdocsMap.set(sanitizedId, {
                    id: sanitizedId,
                    documentId: subdocument.documentId,
                    name: trimmedName,
                    createdAt: subdocument.createdAt || Date.now(),
                    updatedAt: subdocument.updatedAt || Date.now(),
                    type,
                });
                setNewSubdocName("");
            }
        } catch (error) {
            console.error("Failed to create subdocument:", error);
        } finally {
            setIsLoading(false);
        }
    };

    const handleDeleteSubdoc = (id: string, name: string) => {
        setDeleteModal({
            isOpen: true,
            subdocId: id,
            subdocName: name,
        });
    };

    const handleDeleteSubdocConfirm = async () => {
        if (isReadOnly) return;
        if (!subdocsMap || !deleteModal.subdocId) return;

        setIsDeleting(true);
        try {
            const response = await fetch(
                `/api/documents/${encodeURIComponent(documentId)}/subdocuments/${encodeURIComponent(deleteModal.subdocId)}`,
                { method: "DELETE" },
            );
            if (!response.ok && response.status !== 404) {
                throw new Error("Failed to delete subdocument metadata");
            }
            subdocsMap.delete(deleteModal.subdocId);
            setDeleteModal({
                isOpen: false,
                subdocId: "",
                subdocName: "",
            });
        } catch (error) {
            console.error("Failed to delete subdocument:", error);
        } finally {
            setIsDeleting(false);
        }
    };

    const handleDeleteSubdocCancel = () => {
        setDeleteModal({
            isOpen: false,
            subdocId: "",
            subdocName: "",
        });
    };

    return (
        <div className="h-full min-h-0 flex flex-col">
            {!embedded && (
                <h2 className="text-sm font-semibold text-slate-900 dark:text-slate-100 mb-4 uppercase tracking-wider">Subdocumentos</h2>
            )}

            {/* Create Subdocument Form */}
            <div className="mb-6 flex flex-col gap-2">
                <input
                    type="text"
                    placeholder="Nome do subdocumento..."
                    value={newSubdocName}
                    onChange={(e) => setNewSubdocName(e.target.value)}
                    onKeyDown={(e) => { if (!isReadOnly && e.key === 'Enter' && newSubdocName.trim()) setShowTypeDropdown(true); }}
                    disabled={isReadOnly || isLoading || !subdocsMap}
                    className="px-3 py-2 rounded-md border border-slate-300 dark:border-slate-600 bg-white dark:bg-slate-800 focus:outline-none focus:border-slate-500 dark:focus:border-slate-400 focus:ring-1 focus:ring-slate-500 dark:focus:ring-slate-400 text-sm placeholder-slate-400 dark:placeholder-slate-500 text-slate-900 dark:text-slate-100 transition"
                />
                {/* Dropdown button */}
                <div className="relative" ref={dropdownRef}>
                    <button
                        type="button"
                        disabled={isReadOnly || !newSubdocName.trim() || isLoading || !subdocsMap}
                        onClick={() => setShowTypeDropdown(v => !v)}
                        className="w-full flex items-center justify-between px-3 py-2 bg-slate-900 dark:bg-slate-100 text-white dark:text-slate-900 rounded-md hover:bg-slate-800 dark:hover:bg-slate-200 transition disabled:opacity-50 disabled:cursor-not-allowed text-sm font-medium"
                    >
                        <span>{isReadOnly ? "Somente leitura" : isLoading ? "⏳ Criando..." : "+ Criar"}</span>
                        <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" className={`transition-transform ${showTypeDropdown ? 'rotate-180' : ''}`}>
                            <polyline points="6 9 12 15 18 9" />
                        </svg>
                    </button>
                    {showTypeDropdown && (
                        <div className="absolute top-full left-0 right-0 mt-1 bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-md shadow-lg z-10 overflow-hidden">
                            {SUBDOC_TYPE_CONFIGS.map(cfg => (
                                <button
                                    key={cfg.type}
                                    type="button"
                                    onClick={() => handleCreateSubdoc(cfg.type)}
                                    className="w-full flex items-center gap-3 px-3 py-2.5 hover:bg-slate-50 dark:hover:bg-slate-700 transition text-left"
                                >
                                    <span className="text-slate-500 dark:text-slate-400">
                                        <SubdocIcon type={cfg.type} size={15} />
                                    </span>
                                    <div>
                                        <div className="text-sm font-medium text-slate-900 dark:text-slate-100">{cfg.label}</div>
                                        <div className="text-xs text-slate-400 dark:text-slate-500">{cfg.description}</div>
                                    </div>
                                </button>
                            ))}
                        </div>
                    )}
                </div>
            </div>

            <div className="mb-4 flex flex-col gap-2">
                <div className="flex items-center gap-2">
                    <div className="relative flex-1">
                        <input
                            type="text"
                            value={searchQuery}
                            onChange={(event) => setSearchQuery(event.target.value)}
                            placeholder="Pesquisar subdocumentos..."
                            className="w-full rounded-md border border-slate-300 bg-white py-2 pl-9 pr-3 text-sm text-slate-900 placeholder-slate-400 transition focus:border-slate-500 focus:outline-none focus:ring-1 focus:ring-slate-500 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-100 dark:placeholder-slate-500 dark:focus:border-slate-400 dark:focus:ring-slate-400"
                        />
                        <span className="pointer-events-none absolute inset-y-0 left-3 flex items-center text-slate-400 dark:text-slate-500">
                            <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                                <circle cx="11" cy="11" r="8" />
                                <path d="m21 21-4.3-4.3" />
                            </svg>
                        </span>
                    </div>
                    <button
                        type="button"
                        onClick={() => setSortDirection((current) => current === "asc" ? "desc" : "asc")}
                        className="inline-flex h-10 items-center gap-2 rounded-md border border-slate-200 bg-white px-3 text-sm font-medium text-slate-600 transition hover:border-slate-300 hover:bg-slate-50 hover:text-slate-900 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-300 dark:hover:border-slate-600 dark:hover:bg-slate-800 dark:hover:text-slate-100"
                        title={sortDirection === "asc" ? "Ordenar de Z a A" : "Ordenar de A a Z"}
                    >
                        <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                            <path d="m3 16 4 4 4-4" />
                            <path d="M7 20V4" />
                            <path d="M11 4h10" />
                            <path d="M11 8h7" />
                            <path d="M11 12h4" />
                        </svg>
                        <span>{sortDirection === "asc" ? "A-Z" : "Z-A"}</span>
                    </button>
                </div>

                <div className="flex items-center justify-between gap-3 text-xs text-slate-500 dark:text-slate-400">
                    <span>
                        {visibleSubdocs.length} de {subdocs.length} subdocumento{subdocs.length !== 1 ? "s" : ""}
                    </span>
                    <span className="rounded-full border border-slate-200 bg-slate-50 px-2.5 py-1 font-medium dark:border-slate-700 dark:bg-slate-800">
                        Ctrl/Cmd + J
                    </span>
                </div>
            </div>

            {/* Subdocuments List */}
            <div className="min-h-0 flex-1 overflow-y-auto overscroll-contain pr-1">
                {subdocs.length === 0 ? (
                    <div className="text-center py-8">
                        <p className="text-slate-400 dark:text-slate-500 text-sm">Nenhum subdocumento ainda</p>
                        <p className="text-slate-400 dark:text-slate-500 text-xs mt-2">Crie um novo acima</p>
                    </div>
                ) : visibleSubdocs.length === 0 ? (
                    <div className="py-10 text-center">
                        <p className="text-sm text-slate-500 dark:text-slate-400">Nenhum subdocumento encontrado.</p>
                        <p className="mt-2 text-xs text-slate-400 dark:text-slate-500">Ajuste a busca ou abra a paleta rápida.</p>
                    </div>
                ) : (
                    <ul className="space-y-2">
                        {visibleSubdocs.map((subdoc) => (
                            <li key={subdoc.id} className="group">
                                <Link href={getSubdocumentHref(parentHref ?? `/${encodeURIComponent(documentId)}`, subdoc.id)}>
                                    <div className="flex items-start justify-between p-3 rounded-lg border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-800 hover:bg-slate-100 dark:hover:bg-slate-700 hover:border-slate-300 dark:hover:border-slate-600 transition-all cursor-pointer">
                                        <div className="flex items-start gap-2.5 flex-1 min-w-0">
                                            <span className="mt-0.5 flex-shrink-0 text-slate-400 dark:text-slate-500">
                                                <SubdocIcon type={subdoc.type} size={15} />
                                            </span>
                                            <div className="min-w-0">
                                                <h3 className="font-medium text-slate-900 dark:text-slate-100 text-sm truncate">{subdoc.name}</h3>
                                                <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                                                    {subdoc.type ? SUBDOC_TYPE_CONFIGS.find(c => c.type === subdoc.type)?.label : 'Texto'}
                                                    {' · '}
                                                    {new Date(subdoc.createdAt).toLocaleDateString("pt-BR", { year: "numeric", month: "short", day: "numeric" })}
                                                </p>
                                            </div>
                                        </div>
                                        <button
                                            onClick={(e) => {
                                                e.preventDefault();
                                                e.stopPropagation();
                                                handleDeleteSubdoc(subdoc.id, subdoc.name);
                                            }}
                                            disabled={isReadOnly}
                                            className="ml-2 p-1 text-slate-400 dark:text-slate-500 hover:text-red-600 dark:hover:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 rounded transition md:opacity-0 md:group-hover:opacity-100 opacity-100"
                                            title="Deletar subdocumento"
                                        >
                                            <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                                                <path d="M3 6h18" /><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6" /><path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2" />
                                            </svg>
                                        </button>
                                    </div>
                                </Link>
                            </li>
                        ))}
                    </ul>
                )}
            </div>

            {/* Delete Confirmation Modal */}
            <ConfirmDeleteModal
                isOpen={deleteModal.isOpen}
                title="Deletar subdocumento"
                message="Tem certeza que deseja deletar este subdocumento? Todos os arquivos anexados também serão deletados. Esta ação não pode ser desfeita."
                itemName={deleteModal.subdocName}
                isLoading={isDeleting}
                onConfirm={handleDeleteSubdocConfirm}
                onCancel={handleDeleteSubdocCancel}
            />
        </div>
    );
}
