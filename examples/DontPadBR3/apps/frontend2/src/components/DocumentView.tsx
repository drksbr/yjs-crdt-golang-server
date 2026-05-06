"use client";

import { Toast, useToast } from "./Toast";
import { CollaboratorCursors } from "./CollaboratorCursors";
import { DocumentActions, DocumentPanelId } from "./DocumentActions";
import { DocumentCommandPalette } from "./DocumentCommandPalette";
import { DocumentHeader } from "./DocumentHeader";
import { ResponsivePanel } from "./ResponsivePanel";
import { Suspense, lazy, useState, useEffect } from "react";
import { useDocumentMeta } from "@/lib/useDocumentMeta";
import { useOfflinePersistence } from "@/lib/useOfflinePersistence";
import { useMap } from "@/lib/collab/react";
import type * as Y from "yjs";
import { SubdocType } from "@/lib/subdocTypes";
import { useDocumentSecurity } from "@/lib/documentSecurityContext";
import { useFrontendEvents } from "@/stores/events";
import { DocumentLoadingProgress } from "./DocumentLoadingProgress";

const TextEditor = lazy(() => import("./TextEditor").then((module) => ({ default: module.TextEditor })));
const MarkdownEditor = lazy(() => import("./MarkdownEditor").then((module) => ({ default: module.MarkdownEditor })));
const ChecklistEditor = lazy(() => import("./ChecklistEditor").then((module) => ({ default: module.ChecklistEditor })));
const KanbanEditor = lazy(() => import("./KanbanEditor").then((module) => ({ default: module.KanbanEditor })));
const DrawingEditor = lazy(() => import("./DrawingEditor").then((module) => ({ default: module.DrawingEditor })));
const SubdocumentManager = lazy(() => import("./SubdocumentManager").then((module) => ({ default: module.SubdocumentManager })));
const FileManager = lazy(() => import("./FileManager").then((module) => ({ default: module.FileManager })));
const AudioNotes = lazy(() => import("./AudioNotes").then((module) => ({ default: module.AudioNotes })));
const DocumentSettings = lazy(() => import("./DocumentSettings").then((module) => ({ default: module.DocumentSettings })));

type ActivePanelId = DocumentPanelId | "settings";

function EditorChunkFallback({ label }: { label: string }) {
    return (
        <DocumentLoadingProgress
            stage="editor"
            mode="panel"
            title={label}
            description="A nota ja esta conectada. Estamos carregando apenas o modulo visual necessario para este tipo de documento."
            detail="Preparando recursos do editor."
        />
    );
}

function PanelChunkFallback({ label }: { label: string }) {
    return (
        <div className="flex h-full min-h-0 flex-col gap-4 p-1">
            <div className="flex items-center gap-3 rounded-xl border border-slate-200/80 bg-white/70 px-3 py-2.5 dark:border-slate-800 dark:bg-slate-900/55">
                <span className="inline-block size-2.5 animate-pulse rounded-full bg-slate-400 dark:bg-slate-500" />
                <span className="text-sm font-medium text-slate-600 dark:text-slate-300">
                    {label}
                </span>
            </div>
            <div className="flex flex-col gap-2">
                <div className="h-10 rounded-lg bg-slate-100 dark:bg-slate-800" />
                <div className="h-10 rounded-lg bg-slate-100/80 dark:bg-slate-800/80" />
                <div className="h-10 rounded-lg bg-slate-100/60 dark:bg-slate-800/60" />
            </div>
        </div>
    );
}

interface DocumentViewProps {
    documentId: string;
    activeDocumentId?: string;
    displayDocumentId?: string;
    parentHref?: string;
    subdocumentName?: string;
    subdocumentSlug?: string;
    subdocumentType?: SubdocType;
    parentSubdocumentsMap?: Y.Map<unknown>;
}

export function DocumentView({
    documentId,
    activeDocumentId,
    displayDocumentId,
    parentHref,
    subdocumentName,
    subdocumentSlug,
    subdocumentType,
    parentSubdocumentsMap,
}: DocumentViewProps) {
    const currentDocumentId = activeDocumentId ?? documentId;
    const [activePanel, setActivePanel] = useState<ActivePanelId | null>(null);
    const [showCommandPalette, setShowCommandPalette] = useState(false);
    const { toast, showToast } = useToast();
    const [hideToast, setHideToast] = useState(false);
    const { lastModified, lastAccessed, updateLastAccessed } = useDocumentMeta();
    useOfflinePersistence(currentDocumentId);
    const { visibilityMode, isReadOnly, requestEdit } = useDocumentSecurity();
    const emit = useFrontendEvents((state) => state.emit);

    // Read subdoc type from the parent's subdocuments Y.Map when a route boundary
    // provides it. The current Y.Doc may be the independent child document.
    const subdocsMap = useMap("subdocuments");
    const subdocType: SubdocType = (() => {
        if (subdocumentType) return subdocumentType;
        const metadataMap = parentSubdocumentsMap ?? subdocsMap;
        if (!subdocumentName || !metadataMap) return 'texto';
        const entries = Array.from(metadataMap.entries());
        for (let i = 0; i < entries.length; i++) {
            const entry = entries[i][1] as any;
            if ((entry?.name === subdocumentName || entries[i][0] === subdocumentSlug) && entry?.type) {
                return entry.type as SubdocType;
            }
        }
        return 'texto';
    })();

    useEffect(() => {
        updateLastAccessed();
    }, [updateLastAccessed]);

    useEffect(() => {
        const handleKeydown = (event: KeyboardEvent) => {
            const isShortcut = (event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "j";
            if (!isShortcut) return;

            const target = event.target as HTMLElement | null;
            const tagName = target?.tagName?.toLowerCase();
            const isTypingTarget =
                tagName === "input" ||
                tagName === "textarea" ||
                tagName === "select" ||
                target?.isContentEditable;

            const isEditorShortcutScope = Boolean(
                target?.closest('[data-command-palette-scope="editor"]'),
            );

            if (isTypingTarget && !isEditorShortcutScope) {
                return;
            }

            event.preventDefault();
            setShowCommandPalette((current) => !current);
        };

        document.addEventListener("keydown", handleKeydown);
        return () => document.removeEventListener("keydown", handleKeydown);
    }, []);

    const handleTogglePanel = (panel: DocumentPanelId) => {
        setActivePanel((current) => {
            const next = current === panel ? null : panel;
            if (next) emit("panel:opened", { panel: next });
            return next;
        });
    };

    const handleCopyLink = async () => {
        try {
            await navigator.clipboard.writeText(window.location.href);
            showToast("Link copiado com sucesso!");
            setHideToast(false);
        } catch (err) {
            showToast("Erro ao copiar link");
        }
    };

    const handleOpenSettings = () => {
        setActivePanel((current) => current === "settings" ? null : "settings");
    };

    const closePanel = () => {
        setActivePanel(null);
    };

    const activePanelTitle =
        activePanel === "subdocs"
            ? "Subdocumentos"
            : activePanel === "files"
                ? "Arquivos"
                : activePanel === "audio"
                    ? "Notas de áudio"
                    : activePanel === "settings"
                        ? "Configurações da Nota"
                        : "";
    const activeToolPanel = activePanel === "settings" ? null : activePanel;
    const showSettings = activePanel === "settings";

    return (
        <div className="h-dvh min-h-0 flex flex-col overflow-hidden bg-[var(--app-bg)] font-sans text-slate-900 transition-colors dark:bg-slate-950 dark:text-slate-100">
            <CollaboratorCursors subdocumentName={subdocumentName} />
            <DocumentHeader
                documentId={displayDocumentId ?? documentId}
                subdocumentName={subdocumentName}
                parentHref={parentHref}
                visibilityMode={visibilityMode}
                isReadOnly={isReadOnly}
                onRequestEdit={requestEdit}
                lastModified={lastModified}
                lastAccessed={lastAccessed}
                actions={
                    <DocumentActions
                        activePanel={activeToolPanel}
                        onTogglePanel={handleTogglePanel}
                        onCopyLink={handleCopyLink}
                        onOpenSettings={handleOpenSettings}
                        isSettingsOpen={activePanel === "settings"}
                    />
                }
            />

            {/* Toast Notification */}
            {
                toast && !hideToast && (
                    <Toast
                        message={toast.message}
                        onClose={() => setHideToast(true)}
                    />
                )
            }

            {/* Main Content */}
            <div className="min-h-0 flex-1 flex overflow-hidden">
                {/* Editor */}
                <div className="min-w-0 flex-1 overflow-hidden w-full flex flex-col bg-[var(--app-bg-muted)] dark:bg-slate-950">
                    <Suspense fallback={<EditorChunkFallback label="Carregando editor..." />}>
                        {subdocType === 'markdown' && subdocumentName ? (
                            <MarkdownEditor subdocumentName={subdocumentName} />
                        ) : subdocType === 'checklist' && subdocumentName ? (
                            <ChecklistEditor subdocumentName={subdocumentName} />
                        ) : subdocType === 'kanban' && subdocumentName ? (
                            <KanbanEditor documentId={currentDocumentId} subdocumentName={subdocumentName} />
                        ) : subdocType === 'desenho' && subdocumentName ? (
                            <DrawingEditor subdocumentName={subdocumentName} />
                        ) : (
                            <TextEditor documentId={currentDocumentId} subdocumentName={subdocumentName} />
                        )}
                    </Suspense>
                </div>

                <ResponsivePanel open={activePanel !== null} title={activePanelTitle} onClose={closePanel}>
                    <Suspense fallback={<PanelChunkFallback label="Carregando painel..." />}>
                        {activePanel === "subdocs" ? (
                            <SubdocumentManager
                                documentId={documentId}
                                parentHref={parentHref}
                                subdocumentsMap={parentSubdocumentsMap}
                                embedded
                            />
                        ) : activePanel === "files" ? (
                            <FileManager
                                documentId={currentDocumentId}
                                embedded
                            />
                        ) : activePanel === "audio" ? (
                            <AudioNotes
                                documentId={currentDocumentId}
                                embedded
                            />
                        ) : activePanel === "settings" ? (
                            <DocumentSettings
                                documentId={currentDocumentId}
                                isOpen={showSettings}
                                onClose={closePanel}
                                embedded
                            />
                        ) : null}
                    </Suspense>
                </ResponsivePanel>
            </div>

            {/* Voice Chat PoC
            {showVoiceChat && (
                <VoiceChat documentId={documentId} onClose={() => setShowVoiceChat(false)} />
            )} */}

            <DocumentCommandPalette
                open={showCommandPalette}
                onClose={() => setShowCommandPalette(false)}
                documentId={documentId}
                subdocumentName={subdocumentName}
                parentHref={parentHref}
                subdocumentsMap={parentSubdocumentsMap}
                onTogglePanel={handleTogglePanel}
                onCopyLink={handleCopyLink}
                onOpenSettings={handleOpenSettings}
            />
        </div >
    );
}
