"use client";

import { SubdocumentManager } from "./SubdocumentManager";
import { Toast, useToast } from "./Toast";
import { TextEditor } from "./TextEditor";
import { MarkdownEditor } from "./MarkdownEditor";
import { ChecklistEditor } from "./ChecklistEditor";
import { KanbanEditor } from "./KanbanEditor";
import { DrawingEditor } from "./DrawingEditor";
import { FileManager } from "./FileManager";
import { AudioNotes } from "./AudioNotes";
import { DocumentSettings } from "./DocumentSettings";
import { CollaboratorCursors } from "./CollaboratorCursors";
import { DocumentActions, DocumentPanelId } from "./DocumentActions";
import { DocumentCommandPalette } from "./DocumentCommandPalette";
import { DocumentHeader } from "./DocumentHeader";
import { ResponsivePanel } from "./ResponsivePanel";
import { useState, useEffect } from "react";
import { useDocumentMeta } from "@/lib/useDocumentMeta";
import { useOfflinePersistence } from "@/lib/useOfflinePersistence";
import { useMap } from "@/lib/collab/react";
import type * as Y from "yjs";
import { SubdocType } from "@/lib/subdocTypes";
import { useDocumentSecurity } from "@/lib/documentSecurityContext";

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
    const [activePanel, setActivePanel] = useState<DocumentPanelId | null>(null);
    const [showCommandPalette, setShowCommandPalette] = useState(false);
    const [showSettings, setShowSettings] = useState(false);
    const { toast, showToast } = useToast();
    const [hideToast, setHideToast] = useState(false);
    const { lastModified, lastAccessed, updateLastAccessed } = useDocumentMeta();
    useOfflinePersistence(currentDocumentId);
    const { visibilityMode, isReadOnly, requestEdit } = useDocumentSecurity();

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
        setActivePanel((current) => (current === panel ? null : panel));
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
        setActivePanel(null);
        setShowSettings(true);
    };

    const closePanel = () => setActivePanel(null);

    const activePanelTitle =
        activePanel === "subdocs"
            ? "Subdocumentos"
            : activePanel === "files"
                ? "Arquivos"
                : activePanel === "audio"
                    ? "Notas de áudio"
                    : "";

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
                        activePanel={activePanel}
                        onTogglePanel={handleTogglePanel}
                        onCopyLink={handleCopyLink}
                        onOpenSettings={handleOpenSettings}
                        isSettingsOpen={showSettings}
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
                </div>

                <ResponsivePanel open={activePanel !== null} title={activePanelTitle} onClose={closePanel}>
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
                    ) : null}
                </ResponsivePanel>

                <ResponsivePanel open={showSettings} title="Configurações da Nota" onClose={() => setShowSettings(false)}>
                    <DocumentSettings
                        documentId={currentDocumentId}
                        isOpen={showSettings}
                        onClose={() => setShowSettings(false)}
                        embedded
                    />
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
