"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { useMap, useYDoc } from "@/lib/collab/react";
import type * as YTypes from "yjs";
import { applyVersionSnapshot } from "@/lib/noteStateAdapters";
import { SubdocType } from "@/lib/subdocTypes";
import { formatBytes } from "@/lib/utils";
import * as Y from 'yjs';
import { useDocumentSecurity } from "@/lib/documentSecurityContext";

// Constantes de versão
const AUTO_VERSION_PREFIX = '[Auto] ';
const MAX_AUTO_VERSIONS = 10;
const AUTO_SAVE_INTERVAL = 60000; // 1 minuto

interface DocumentVersion {
    id: string;
    documentId: string;
    subdocumentName?: string;
    timestamp: number;
    label?: string;
    size: number;
    createdBy?: string;
}

interface DocumentSettingsProps {
    documentId: string;
    subdocumentName?: string;
    subdocumentSlug?: string;
    subdocumentsMap?: YTypes.Map<unknown>;
    isOpen: boolean;
    onClose: () => void;
    embedded?: boolean;
}

type TabType = 'versions' | 'security' | 'danger';

export function DocumentSettings({
    documentId,
    subdocumentName,
    subdocumentSlug,
    subdocumentsMap,
    isOpen,
    onClose,
    embedded = false,
}: DocumentSettingsProps) {
    const [activeTab, setActiveTab] = useState<TabType>('versions');
    const { isReadOnly } = useDocumentSecurity();

    // PIN / security states
    const [pin, setPin] = useState("");
    const [confirmPin, setConfirmPin] = useState("");
    const [isProtected, setIsProtected] = useState(false);
    const [hasExistingPin, setHasExistingPin] = useState(false);
    const [currentVisibilityMode, setCurrentVisibilityMode] = useState<"public" | "public-readonly" | "private">("public");
    const [selectedVisibilityMode, setSelectedVisibilityMode] = useState<"public" | "public-readonly" | "private">("public");
    const [isLoading, setIsLoading] = useState(false);
    const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
    const [isDeleting, setIsDeleting] = useState(false);
    const [message, setMessage] = useState("");
    const [messageType, setMessageType] = useState<"success" | "error" | "">();
    const [showPassword, setShowPassword] = useState(false);
    const localSubdocsMap = useMap("subdocuments");
    const subdocsMap = subdocumentsMap ?? localSubdocsMap;

    // Version states
    const [versions, setVersions] = useState<DocumentVersion[]>([]);
    const [loadingVersions, setLoadingVersions] = useState(false);
    const [savingVersion, setSavingVersion] = useState(false);
    const [restoringVersion, setRestoringVersion] = useState<string | null>(null);
    const [newVersionLabel, setNewVersionLabel] = useState('');
    const lastAutoSaveRef = useRef<string | null>(null);

    const doc = useYDoc();
    const currentSubdocType: SubdocType = (() => {
        if (!subdocumentName || !subdocsMap) return "texto";
        const entries = Array.from(subdocsMap.entries());
        for (const [id, value] of entries) {
            const entry = value as { name?: string; type?: SubdocType };
            if ((entry?.name === subdocumentName || subdocumentSlug === id) && entry?.type) {
                return entry.type;
            }
        }
        return "texto";
    })();

    // Load settings from backend
    useEffect(() => {
        if (!isOpen) return;

        const loadSecurity = async () => {
            try {
                const response = await fetch(`/api/documents/${encodeURIComponent(documentId)}/security`);
                if (!response.ok) return;
                const data = await response.json();
                const mode = (data.visibilityMode as "public" | "public-readonly" | "private") || "public";
                const hasPin = typeof data.hasPin === "boolean" ? data.hasPin : mode !== "public";
                setIsProtected(mode === "private");
                setHasExistingPin(hasPin);
                setCurrentVisibilityMode(mode);
                setSelectedVisibilityMode(mode);
            } catch (error) {
                console.error("Error loading settings:", error);
            }
        };

        void loadSecurity();
        setPin("");
        setConfirmPin("");
    }, [documentId, isOpen]);

    // Fetch versions
    const fetchVersions = useCallback(async () => {
        setLoadingVersions(true);
        try {
            const params = new URLSearchParams();
            if (subdocumentName) {
                params.set('subdocument', subdocumentName);
            }

            const response = await fetch(
                `/api/documents/${documentId}/versions?${params.toString()}`
            );

            if (response.ok) {
                const data = await response.json();
                setVersions(data.versions || []);
            }
        } catch (error) {
            console.error('Error fetching versions:', error);
            showMessage('Erro ao carregar versões', 'error');
        } finally {
            setLoadingVersions(false);
        }
    }, [documentId, subdocumentName]);

    // Load versions when opening and on versions tab
    useEffect(() => {
        if (isOpen && activeTab === 'versions') {
            fetchVersions();
        }
    }, [isOpen, activeTab, fetchVersions]);

    // Auto-save functionality
    const createAutoVersion = useCallback(async () => {
        if (isReadOnly || !doc) return;

        try {
            const params = new URLSearchParams();
            if (subdocumentName) {
                params.set('subdocument', subdocumentName);
            }

            const listResponse = await fetch(
                `/api/documents/${documentId}/versions?${params.toString()}`
            );

            if (!listResponse.ok) return;

            const data = await listResponse.json();
            const allVersions: DocumentVersion[] = data.versions || [];
            const autoVersions = allVersions.filter(v => v.label?.startsWith(AUTO_VERSION_PREFIX));

            if (autoVersions.length >= MAX_AUTO_VERSIONS) {
                const oldestAuto = autoVersions.sort((a, b) => a.timestamp - b.timestamp)[0];
                if (oldestAuto) {
                    await fetch(
                        `/api/documents/${documentId}/versions/${oldestAuto.id}`,
                        { method: 'DELETE' }
                    );
                }
            }

            const update = Y.encodeStateAsUpdateV2(doc);
            const currentHash = Array.from(new Uint8Array(update.slice(0, 100)))
                .map(b => b.toString(16).padStart(2, '0'))
                .join('');

            if (lastAutoSaveRef.current === currentHash) {
                return;
            }

            const formData = new FormData();
            formData.append('update', new Blob([new Uint8Array(update).buffer], { type: 'application/octet-stream' }));
            formData.append('label', `${AUTO_VERSION_PREFIX}${new Date().toLocaleTimeString('pt-BR')}`);
            if (subdocumentName) {
                formData.append('subdocument', subdocumentName);
            }

            const response = await fetch(`/api/documents/${documentId}/versions`, {
                method: 'POST',
                body: formData,
            });

            if (response.ok) {
                lastAutoSaveRef.current = currentHash;
            }
        } catch (error) {
            console.error('Erro no auto-save:', error);
        }
    }, [doc, documentId, isReadOnly, subdocumentName]);

    // Timer para auto-save
    useEffect(() => {
        const intervalId = setInterval(() => {
            createAutoVersion();
        }, AUTO_SAVE_INTERVAL);

        return () => clearInterval(intervalId);
    }, [createAutoVersion]);

    const showMessage = (msg: string, type: "success" | "error") => {
        setMessage(msg);
        setMessageType(type);
        setTimeout(() => setMessage(""), 3000);
    };

    // Version handlers
    const createVersion = async () => {
        if (isReadOnly) {
            showMessage('Documento em modo somente leitura', 'error');
            return;
        }

        if (!doc) {
            showMessage('Documento não está conectado', 'error');
            return;
        }

        setSavingVersion(true);
        try {
            const update = Y.encodeStateAsUpdateV2(doc);
            const formData = new FormData();
            formData.append('update', new Blob([new Uint8Array(update).buffer], { type: 'application/octet-stream' }));
            if (newVersionLabel) {
                formData.append('label', newVersionLabel);
            }
            if (subdocumentName) {
                formData.append('subdocument', subdocumentName);
            }

            const response = await fetch(`/api/documents/${documentId}/versions`, {
                method: 'POST',
                body: formData,
            });

            if (response.ok) {
                showMessage('Versão salva com sucesso!', 'success');
                setNewVersionLabel('');
                fetchVersions();
            } else {
                throw new Error('Failed to save version');
            }
        } catch (error) {
            console.error('Error creating version:', error);
            showMessage('Erro ao salvar versão', 'error');
        } finally {
            setSavingVersion(false);
        }
    };

    const restoreVersion = async (versionId: string) => {
        if (isReadOnly) {
            showMessage('Documento em modo somente leitura', 'error');
            return;
        }

        if (!doc) {
            showMessage('Documento não está conectado', 'error');
            return;
        }

        const confirmed = window.confirm(
            'Tem certeza que deseja restaurar esta versão? O conteúdo atual será substituído.'
        );

        if (!confirmed) return;

        setRestoringVersion(versionId);
        try {
            // Save current state as backup
            const currentUpdate = Y.encodeStateAsUpdateV2(doc);
            const backupFormData = new FormData();
            backupFormData.append('update', new Blob([new Uint8Array(currentUpdate).buffer], { type: 'application/octet-stream' }));
            backupFormData.append('label', 'Backup antes de restaurar');
            if (subdocumentName) {
                backupFormData.append('subdocument', subdocumentName);
            }

            await fetch(`/api/documents/${documentId}/versions`, {
                method: 'POST',
                body: backupFormData,
            });

            // Fetch the version to restore
            const response = await fetch(
                `/api/documents/${documentId}/versions/${versionId}`,
                { method: 'POST' }
            );

            if (!response.ok) {
                throw new Error('Failed to fetch version data');
            }

            const updateBuffer = await response.arrayBuffer();
            const update = new Uint8Array(updateBuffer);

            const tempDoc = new Y.Doc();
            Y.applyUpdateV2(tempDoc, update);

            applyVersionSnapshot({
                sourceDoc: tempDoc,
                targetDoc: doc,
                subdocType: currentSubdocType,
                subdocumentName,
            });

            tempDoc.destroy();

            showMessage('Versão restaurada com sucesso!', 'success');
            fetchVersions();
        } catch (error) {
            console.error('Error restoring version:', error);
            showMessage('Erro ao restaurar versão', 'error');
        } finally {
            setRestoringVersion(null);
        }
    };

    const deleteVersion = async (versionId: string) => {
        if (isReadOnly) {
            showMessage('Documento em modo somente leitura', 'error');
            return;
        }

        const confirmed = window.confirm('Tem certeza que deseja excluir esta versão?');
        if (!confirmed) return;

        try {
            const response = await fetch(
                `/api/documents/${documentId}/versions/${versionId}`,
                { method: 'DELETE' }
            );

            if (response.ok) {
                showMessage('Versão excluída', 'success');
                fetchVersions();
            } else {
                throw new Error('Failed to delete version');
            }
        } catch (error) {
            console.error('Error deleting version:', error);
            showMessage('Erro ao excluir versão', 'error');
        }
    };

    // Security handlers
    const handleSaveSecuritySettings = async () => {
        if (isReadOnly) {
            showMessage("Documento em modo somente leitura", "error");
            return;
        }

        setIsLoading(true);
        try {
            const trimmedPin = pin.trim();
            if (selectedVisibilityMode !== "public") {
                if (trimmedPin) {
                    if (trimmedPin.length < 4) {
                        showMessage("PIN deve ter pelo menos 4 dígitos", "error");
                        setIsLoading(false);
                        return;
                    }
                    if (trimmedPin !== confirmPin) {
                        showMessage("Os PINs não correspondem", "error");
                        setIsLoading(false);
                        return;
                    }
                } else if (!hasExistingPin) {
                    showMessage("É necessário configurar um PIN para este modo", "error");
                    setIsLoading(false);
                    return;
                }
            }

            const response = await fetch(`/api/documents/${encodeURIComponent(documentId)}/security`, {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({
                    visibilityMode: selectedVisibilityMode,
                    pin: trimmedPin,
                }),
            });
            if (!response.ok) {
                const payload = await response.json().catch(() => ({}));
                throw new Error(payload.error || "Falha ao salvar segurança");
            }

            setCurrentVisibilityMode(selectedVisibilityMode);
            setIsProtected(selectedVisibilityMode === "private");
            setHasExistingPin(selectedVisibilityMode !== "public");
            setPin("");
            setConfirmPin("");
            showMessage("Configurações de segurança salvas", "success");
        } catch (error) {
            console.error("[DocumentSettings] Erro ao salvar segurança:", error);
            showMessage("Erro ao salvar configurações", "error");
        } finally {
            setIsLoading(false);
        }
    };

    const handleDeleteDocument = async () => {
        if (isReadOnly) {
            showMessage("Documento em modo somente leitura", "error");
            return;
        }

        if (!showDeleteConfirm) {
            setShowDeleteConfirm(true);
            return;
        }

        setIsDeleting(true);

        try {
            const response = await fetch(`/api/documents/${encodeURIComponent(documentId)}`, {
                method: "DELETE",
            });

            if (response.ok) {
                showMessage("Nota deletada. Redirecionando...", "success");
                setTimeout(() => {
                    window.location.href = "/";
                }, 1500);
            } else {
                showMessage("Erro ao deletar nota", "error");
            }
        } catch (error) {
            console.error("Failed to delete document:", error);
            showMessage("Erro ao deletar nota", "error");
        } finally {
            setIsDeleting(false);
        }
    };

    // Helper functions
    const formatDate = (timestamp: number) => {
        return new Date(timestamp).toLocaleString('pt-BR', {
            day: '2-digit',
            month: '2-digit',
            year: 'numeric',
            hour: '2-digit',
            minute: '2-digit',
        });
    };

    const getRelativeTime = (timestamp: number) => {
        const seconds = Math.floor((Date.now() - timestamp) / 1000);

        if (seconds < 60) return 'agora mesmo';
        if (seconds < 3600) return `${Math.floor(seconds / 60)} min atrás`;
        if (seconds < 86400) return `${Math.floor(seconds / 3600)} h atrás`;
        if (seconds < 604800) return `${Math.floor(seconds / 86400)} dias atrás`;
        return formatDate(timestamp);
    };

    if (!isOpen) return null;

    const tabs: { id: TabType; label: string; icon: React.ReactNode }[] = [
        {
            id: 'versions',
            label: 'Versões',
            icon: (
                <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <circle cx="12" cy="12" r="10"></circle>
                    <polyline points="12 6 12 12 16 14"></polyline>
                </svg>
            ),
        },
        {
            id: 'security',
            label: 'Segurança',
            icon: (
                <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <rect x="3" y="11" width="18" height="11" rx="2" ry="2"></rect>
                    <path d="M7 11V7a5 5 0 0110 0v4"></path>
                </svg>
            ),
        },
        {
            id: 'danger',
            label: 'Perigo',
            icon: (
                <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <path d="M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z"></path>
                    <line x1="12" y1="9" x2="12" y2="13"></line>
                    <line x1="12" y1="17" x2="12.01" y2="17"></line>
                </svg>
            ),
        },
    ];

    const settingsContent = (
        <>
            {/* Tabs */}
            <div className="border-b border-slate-200/50 dark:border-slate-700/50 px-6">
                <div className="flex gap-1">
                    {tabs.map((tab) => (
                        <button
                            key={tab.id}
                            onClick={() => setActiveTab(tab.id)}
                            className={`flex items-center gap-2 px-3 py-3 text-sm font-medium border-b-2 transition ${activeTab === tab.id
                                ? 'border-slate-900 dark:border-slate-100 text-slate-900 dark:text-slate-100'
                                : 'border-transparent text-slate-500 dark:text-slate-400 hover:text-slate-700 dark:hover:text-slate-300'
                                }`}
                        >
                            {tab.icon}
                            <span className="hidden sm:inline">{tab.label}</span>
                        </button>
                    ))}
                </div>
            </div>

            {/* Content */}
            <div className={embedded ? "min-h-0 flex-1 overflow-y-auto" : "overflow-y-auto max-h-[calc(90dvh-140px)] sm:max-h-[calc(80vh-140px)]"}>
                <div className="px-6 py-6 space-y-6">
                    {/* Message */}
                    {message && (
                        <div
                            className={`p-3 rounded-lg text-sm font-medium transition ${messageType === "success"
                                ? "bg-green-50 dark:bg-green-900/20 text-green-700 dark:text-green-400 border border-green-200/50 dark:border-green-800/50"
                                : "bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-400 border border-red-200/50 dark:border-red-800/50"
                                }`}
                        >
                            {message}
                        </div>
                    )}

                    {/* Versions Tab */}
                    {activeTab === 'versions' && (
                        <div className="space-y-4">
                            {/* Create Version */}
                            <div className="flex gap-2">
                                <input
                                    type="text"
                                    value={newVersionLabel}
                                    onChange={(e) => setNewVersionLabel(e.target.value)}
                                    placeholder="Nome da versão (opcional)"
                                    disabled={isReadOnly}
                                    className="flex-1 px-3 py-2.5 rounded-lg border border-slate-300 dark:border-slate-600 bg-white dark:bg-slate-700/50 focus:outline-none focus:ring-2 focus:ring-slate-400 dark:focus:ring-slate-500 focus:border-transparent text-slate-900 dark:text-slate-100 placeholder-slate-400 dark:placeholder-slate-500 text-sm transition"
                                />
                                <button
                                    onClick={createVersion}
                                    disabled={isReadOnly || savingVersion || !doc}
                                    className="px-4 py-2.5 text-sm font-medium rounded-lg transition disabled:opacity-50 disabled:cursor-not-allowed bg-slate-900 dark:bg-slate-100 text-white dark:text-slate-900 hover:bg-slate-800 dark:hover:bg-slate-200"
                                >
                                    {savingVersion ? '...' : 'Salvar'}
                                </button>
                            </div>

                            {/* Info */}
                            <p className="text-xs text-slate-500 dark:text-slate-400">
                                Auto-save a cada 1 minuto (máx. 10 versões automáticas).
                            </p>

                            {/* Versions List */}
                            {loadingVersions ? (
                                <div className="flex items-center justify-center py-8">
                                    <span className="inline-block w-6 h-6 border-2 border-slate-300 dark:border-slate-600 border-t-slate-600 dark:border-t-slate-300 rounded-full animate-spin" />
                                </div>
                            ) : versions.length === 0 ? (
                                <div className="text-center py-8">
                                    <p className="text-slate-600 dark:text-slate-400">Nenhuma versão salva ainda.</p>
                                    <p className="text-sm text-slate-500 dark:text-slate-500 mt-1">Clique em "Salvar" para criar um ponto de restauração.</p>
                                </div>
                            ) : (
                                <div className="space-y-2 max-h-64 overflow-y-auto">
                                    {versions.map((version) => {
                                        const isAutoVersion = version.label?.startsWith(AUTO_VERSION_PREFIX);
                                        return (
                                            <div
                                                key={version.id}
                                                className="p-3 bg-slate-50 dark:bg-slate-700/30 rounded-lg border border-slate-200/50 dark:border-slate-600/50 hover:border-slate-300 dark:hover:border-slate-500 transition"
                                            >
                                                <div className="flex items-start justify-between gap-3">
                                                    <div className="flex-1 min-w-0">
                                                        <div className="flex items-center gap-2">
                                                            <span className="font-medium text-sm text-slate-800 dark:text-slate-200 truncate">
                                                                {version.label || 'Versão sem nome'}
                                                            </span>
                                                            {isAutoVersion && (
                                                                <span className="shrink-0 text-[10px] font-medium px-1.5 py-0.5 rounded bg-slate-200 dark:bg-slate-600 text-slate-600 dark:text-slate-300">
                                                                    AUTO
                                                                </span>
                                                            )}
                                                        </div>
                                                        <div className="text-xs text-slate-500 dark:text-slate-400 mt-1">
                                                            {getRelativeTime(version.timestamp)} • {formatBytes(version.size)}
                                                        </div>
                                                    </div>
                                                    <div className="flex gap-1.5 shrink-0">
                                                        <button
                                                            onClick={() => restoreVersion(version.id)}
                                                            disabled={isReadOnly || restoringVersion === version.id}
                                                            className="px-2.5 py-1 text-xs font-medium rounded-md transition disabled:opacity-50 bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400 hover:bg-green-200 dark:hover:bg-green-900/50"
                                                            title="Restaurar esta versão"
                                                        >
                                                            {restoringVersion === version.id ? '...' : '↩️'}
                                                        </button>
                                                        <button
                                                            onClick={() => deleteVersion(version.id)}
                                                            disabled={isReadOnly}
                                                            className="px-2 py-1 text-xs font-medium rounded-md transition disabled:opacity-50 bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-400 hover:bg-red-200 dark:hover:bg-red-900/50"
                                                            title="Excluir versão"
                                                        >
                                                            Excluir
                                                        </button>
                                                    </div>
                                                </div>
                                            </div>
                                        );
                                    })}
                                </div>
                            )}
                        </div>
                    )}

                    {/* Security Tab */}
                    {activeTab === 'security' && (
                        <div className="space-y-5">
                            {/* Visibility mode selector */}
                            <div>
                                <h3 className="text-sm font-medium text-slate-900 dark:text-slate-100 mb-3">Visibilidade</h3>
                                <div className="space-y-2">
                                    {([
                                        { mode: "public" as const, title: "Pública", desc: "Qualquer pessoa pode ler e editar", badge: "PUB" },
                                        { mode: "public-readonly" as const, title: "Somente leitura", desc: "Qualquer pessoa pode ler; edição requer PIN", badge: "PIN" },
                                        { mode: "private" as const, title: "Privada", desc: "Acesso requer PIN", badge: "LOCK" },
                                    ]).map(({ mode, title, desc, badge }) => (
                                        <button
                                            key={mode}
                                            onClick={() => setSelectedVisibilityMode(mode)}
                                            disabled={isReadOnly}
                                            className={`w-full flex items-start gap-3 p-3 rounded-lg border-2 text-left transition ${selectedVisibilityMode === mode
                                                ? 'border-slate-900 dark:border-slate-100 bg-slate-50 dark:bg-slate-700/50'
                                                : 'border-slate-200 dark:border-slate-700 hover:border-slate-300 dark:hover:border-slate-600'
                                                }`}
                                        >
                                            <span className="mt-0.5 rounded-md bg-slate-100 px-1.5 py-1 text-[10px] font-semibold uppercase tracking-[0.12em] text-slate-500 dark:bg-slate-800 dark:text-slate-400">
                                                {badge}
                                            </span>
                                            <div className="flex-1 min-w-0">
                                                <div className="text-sm font-medium text-slate-900 dark:text-slate-100">{title}</div>
                                                <div className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">{desc}</div>
                                            </div>
                                            {selectedVisibilityMode === mode && (
                                                <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" className="mt-0.5 shrink-0 text-slate-900 dark:text-slate-100">
                                                    <polyline points="20 6 9 17 4 12"></polyline>
                                                </svg>
                                            )}
                                        </button>
                                    ))}
                                </div>
                            </div>

                            {/* PIN section — shown when mode requires access control */}
                            {selectedVisibilityMode !== "public" && (
                                <div className="space-y-3 pt-3 border-t border-slate-200/50 dark:border-slate-700/50">
                                    <h4 className="text-sm font-medium text-slate-900 dark:text-slate-100">PIN</h4>
                                    {!hasExistingPin && (
                                        <p className="text-xs text-amber-600 dark:text-amber-400 bg-amber-50 dark:bg-amber-900/20 rounded-lg px-3 py-2">
                                            Este modo requer um PIN para controlar acesso.
                                        </p>
                                    )}
                                    <div>
                                        <label className="block text-xs font-medium text-slate-700 dark:text-slate-300 mb-2">
                                            {hasExistingPin ? "Novo PIN (opcional — deixe vazio para manter)" : "PIN (mín. 4 dígitos)"}
                                        </label>
                                        <div className="relative">
                                            <input
                                                type={showPassword ? "text" : "password"}
                                                value={pin}
                                                onChange={(e) => setPin(e.target.value)}
                                                placeholder={hasExistingPin ? "Novo PIN (opcional)" : "Ex: 1234"}
                                                maxLength={8}
                                                disabled={isReadOnly}
                                                className="w-full px-3 py-2.5 rounded-lg border border-slate-300 dark:border-slate-600 bg-white dark:bg-slate-700/50 focus:outline-none focus:ring-2 focus:ring-slate-400 dark:focus:ring-slate-500 focus:border-transparent text-slate-900 dark:text-slate-100 placeholder-slate-400 dark:placeholder-slate-500 text-sm transition"
                                            />
                                            <button
                                                type="button"
                                                onClick={() => setShowPassword(!showPassword)}
                                                disabled={isReadOnly}
                                                className="absolute right-3 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 transition"
                                                aria-label={showPassword ? "Ocultar PIN" : "Mostrar PIN"}
                                            >
                                                {showPassword ? (
                                                    <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M17.94 17.94A10.07 10.07 0 0112 20c-7 0-11-8-11-8a18.45 18.45 0 015.06-5.94M9.9 4.24A9.12 9.12 0 0112 4c7 0 11 8 11 8a18.5 18.5 0 01-2.16 3.19m-6.72-1.07a3 3 0 11-4.24-4.24"></path><line x1="1" y1="1" x2="23" y2="23"></line></svg>
                                                ) : (
                                                    <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"></path><circle cx="12" cy="12" r="3"></circle></svg>
                                                )}
                                            </button>
                                        </div>
                                    </div>
                                    {pin.trim() && (
                                        <div>
                                            <label className="block text-xs font-medium text-slate-700 dark:text-slate-300 mb-2">Confirmar PIN</label>
                                            <input
                                                type={showPassword ? "text" : "password"}
                                                value={confirmPin}
                                                onChange={(e) => setConfirmPin(e.target.value)}
                                                placeholder="Repita o PIN"
                                                maxLength={8}
                                                disabled={isReadOnly}
                                                className="w-full px-3 py-2.5 rounded-lg border border-slate-300 dark:border-slate-600 bg-white dark:bg-slate-700/50 focus:outline-none focus:ring-2 focus:ring-slate-400 dark:focus:ring-slate-500 focus:border-transparent text-slate-900 dark:text-slate-100 placeholder-slate-400 dark:placeholder-slate-500 text-sm transition"
                                            />
                                        </div>
                                    )}
                                </div>
                            )}

                            {/* Save button */}
                            <button
                                onClick={handleSaveSecuritySettings}
                                disabled={isReadOnly || isLoading || (selectedVisibilityMode !== "public" && !hasExistingPin && !pin.trim())}
                                className="w-full px-3 py-2.5 text-sm font-medium rounded-lg transition disabled:opacity-50 disabled:cursor-not-allowed bg-slate-900 dark:bg-slate-100 text-white dark:text-slate-900 hover:bg-slate-800 dark:hover:bg-slate-200"
                            >
                                {isLoading ? "Salvando..." : "Salvar configurações"}
                            </button>
                        </div>
                    )}

                    {/* Danger Tab */}
                    {activeTab === 'danger' && (
                        <div className="space-y-3">
                            <h3 className="text-sm font-medium text-red-600 dark:text-red-400">
                                Zona de Perigo
                            </h3>
                            <p className="text-xs text-slate-600 dark:text-slate-400">
                                Ações irreversíveis. Tenha cuidado ao usar estas opções.
                            </p>

                            {!showDeleteConfirm ? (
                                <button
                                    onClick={handleDeleteDocument}
                                    disabled={isReadOnly || isLoading || isDeleting}
                                    className="w-full px-3 py-2.5 text-sm font-medium rounded-lg border transition disabled:opacity-50 disabled:cursor-not-allowed bg-red-50 dark:bg-red-900/20 border-red-200/50 dark:border-red-800/50 text-red-700 dark:text-red-400 hover:bg-red-100 dark:hover:bg-red-900/30"
                                >
                                    Deletar nota permanentemente
                                </button>
                            ) : (
                                <div className="space-y-2 bg-red-50 dark:bg-red-900/20 border border-red-200/50 dark:border-red-800/50 rounded-lg p-3">
                                    <p className="text-xs font-medium text-red-700 dark:text-red-400">
                                            Tem certeza? Esta ação é permanente e não pode ser desfeita.
                                    </p>
                                    <div className="flex gap-2">
                                        <button
                                            onClick={() => setShowDeleteConfirm(false)}
                                            disabled={isDeleting}
                                            className="flex-1 px-3 py-2 text-xs font-medium rounded-lg border transition disabled:opacity-50 disabled:cursor-not-allowed bg-white dark:bg-slate-700 border-slate-300 dark:border-slate-600 text-slate-900 dark:text-slate-100 hover:bg-slate-50 dark:hover:bg-slate-600"
                                        >
                                            Cancelar
                                        </button>
                                        <button
                                            onClick={handleDeleteDocument}
                                            disabled={isReadOnly || isDeleting}
                                            className="flex-1 px-3 py-2 text-xs font-medium rounded-lg transition disabled:opacity-50 disabled:cursor-not-allowed bg-red-600 text-white hover:bg-red-700"
                                        >
                                            {isDeleting ? "Deletando..." : "Confirmar"}
                                        </button>
                                    </div>
                                </div>
                            )}
                        </div>
                    )}
                </div>
            </div>
        </>
    );

    if (embedded) {
        return (
            <div className="-mx-5 -my-5 flex h-full min-h-0 flex-col overflow-hidden">
                {settingsContent}
            </div>
        );
    }

    return (
        <div className="fixed inset-0 z-50 flex items-end justify-center p-0 sm:items-center sm:p-4">
            {/* Backdrop */}
            <div
                className="absolute inset-0 bg-black/30 backdrop-blur-sm"
                onClick={onClose}
                role="presentation"
            />

            {/* Modal */}
            <div className="relative h-[90dvh] w-full overflow-hidden rounded-t-[1.5rem] border border-slate-200/50 bg-white shadow-xl dark:border-slate-700/50 dark:bg-slate-800 sm:h-auto sm:max-w-lg sm:rounded-xl">
                {/* Header */}
                <div className="border-b border-slate-200/50 dark:border-slate-700/50 px-6 py-5 flex items-center justify-between">
                    <h2 className="text-base font-semibold text-slate-900 dark:text-slate-100">
                        Configurações da Nota
                    </h2>
                    <button
                        onClick={onClose}
                        className="p-1.5 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-700 transition text-slate-500 dark:text-slate-400"
                        aria-label="Fechar"
                    >
                        <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                            <line x1="18" y1="6" x2="6" y2="18"></line>
                            <line x1="6" y1="6" x2="18" y2="18"></line>
                        </svg>
                    </button>
                </div>
                {settingsContent}
            </div>
        </div>
    );
}
