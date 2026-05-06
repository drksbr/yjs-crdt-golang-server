"use client";

import { YDocProvider, useConnectionStatus } from "@/lib/collab/react";
import { useState, useEffect, useCallback } from "react";
import { DocumentSecurityContext, VisibilityMode } from "@/lib/documentSecurityContext";
import { DocumentSyncLifecycle } from "./DocumentSyncLifecycle";
import { DocumentLoadingProgress, DocumentLoadingSteps } from "./DocumentLoadingProgress";

interface SecurityStatus {
    isProtected: boolean;
    hasAccess: boolean;
    visibilityMode: VisibilityMode;
    canEdit: boolean;
}

interface DocumentTokenPayload {
    documentId: string;
    canEdit: boolean;
    persist: boolean;
    wsBaseUrl: string;
    token: string;
}

interface SecureDocumentProviderProps {
    documentId: string;
    syncDocumentId?: string;
    displayDocumentId?: string;
    children: React.ReactNode;
}

function DocumentAccessShell({
    documentId,
    badge,
    badgeClassName,
    title,
    description,
    children,
}: {
    documentId: string;
    badge: React.ReactNode;
    badgeClassName: string;
    title: string;
    description: string;
    children: React.ReactNode;
}) {
    return (
        <div className="min-h-dvh overflow-hidden bg-[var(--app-bg)] text-[var(--app-text)]">
            <div className="relative flex min-h-dvh items-center justify-center px-6 py-16">
                <div className="absolute inset-0 bg-[radial-gradient(circle_at_top,_rgba(37,99,235,0.14),_transparent_42%),radial-gradient(circle_at_bottom_right,_rgba(15,23,42,0.08),_transparent_38%)] dark:bg-[radial-gradient(circle_at_top,_rgba(96,165,250,0.18),_transparent_42%),radial-gradient(circle_at_bottom_right,_rgba(15,23,42,0.36),_transparent_38%)]" />
                <div className="relative w-full max-w-xl">
                    <div className="surface-glass p-8 md:p-10">
                        <div className={badgeClassName}>
                            {badge}
                        </div>
                        <div className="mt-6 space-y-3">
                            <h1 className="text-3xl font-semibold tracking-[-0.03em] text-slate-900 dark:text-slate-50">
                                {title}
                            </h1>
                            <p className="max-w-lg text-sm leading-6 text-slate-600 dark:text-slate-300">
                                {description}
                            </p>
                            <p className="app-code text-xs">
                                /{decodeURIComponent(documentId)}
                            </p>
                        </div>
                        {children}
                    </div>
                </div>
            </div>
        </div>
    );
}

function OpeningDocumentState({
    documentId,
    onRetry,
    error,
}: {
    documentId: string;
    onRetry: () => void;
    error?: string | null;
}) {
    return (
        <DocumentLoadingProgress
            documentId={documentId}
            stage="security"
            title="Verificando acesso e contexto"
            description="Estamos checando permissões, visibilidade e estado da nota para abrir o editor sem chamadas redundantes."
            detail="Carregando os dados iniciais do documento."
            error={error}
            onRetry={onRetry}
        />
    );
}

function ProtectedDocumentState({
    documentId,
    pin,
    error,
    isVerifying,
    showPassword,
    onPinChange,
    onTogglePassword,
    onSubmit,
}: {
    documentId: string;
    pin: string;
    error: string;
    isVerifying: boolean;
    showPassword: boolean;
    onPinChange: (value: string) => void;
    onTogglePassword: () => void;
    onSubmit: (event: React.FormEvent) => void;
}) {
    return (
        <div className="relative">
            <DocumentAccessShell
                documentId={documentId}
                badgeClassName="eyebrow border-amber-200/80 bg-white/70 text-amber-700 dark:border-amber-900/60 dark:bg-slate-900/65 dark:text-amber-300"
                badge={(
                    <>
                        <span className="inline-flex h-5 w-5 items-center justify-center rounded-full bg-amber-100 text-amber-700 dark:bg-amber-950/60 dark:text-amber-300">
                            <svg xmlns="http://www.w3.org/2000/svg" width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                                <rect x="3" y="11" width="18" height="11" rx="2" ry="2" />
                                <path d="M7 11V7a5 5 0 0 1 10 0v4" />
                            </svg>
                        </span>
                        Nota protegida
                    </>
                )}
                title="Aguardando autorizacao"
                description="A nota foi localizada, mas o editor continua bloqueado ate a validacao do PIN. Depois disso a sessao colaborativa continua automaticamente."
            >
                <div className="pointer-events-none mt-8 space-y-8 opacity-55 saturate-75">
                    <DocumentLoadingSteps stage={isVerifying ? "security" : "protected"} />
                    <div className="flex items-center justify-between gap-4 rounded-2xl border border-slate-200/80 bg-white/70 px-4 py-3 text-sm text-slate-500 dark:border-slate-800 dark:bg-slate-900/45 dark:text-slate-400">
                        <span>
                            Aguardando a confirmacao de acesso para continuar.
                        </span>
                        <span className="inline-flex items-center gap-2 font-medium text-slate-700 dark:text-slate-200">
                            <span className="inline-block h-2.5 w-2.5 rounded-full bg-amber-500 animate-pulse" />
                            Protegido
                        </span>
                    </div>
                </div>
            </DocumentAccessShell>

            <div className="pointer-events-none fixed inset-0 z-20 flex items-center justify-center px-6 py-10">
                <div className="absolute inset-0 bg-white/18 backdrop-blur-md dark:bg-slate-950/40" />
                <div
                    role="dialog"
                    aria-modal="true"
                    aria-labelledby="protected-document-pin-title"
                    className="pointer-events-auto relative w-full max-w-md rounded-[28px] border border-white/65 bg-white/72 p-6 shadow-[0_32px_80px_-34px_rgba(15,23,42,0.42)] backdrop-blur-2xl dark:border-slate-700/70 dark:bg-slate-900/76 sm:p-7"
                >
                    <div className="flex items-start justify-between gap-4">
                        <div className="space-y-2">
                            <div className="eyebrow border-white/70 bg-white/55 text-amber-700 dark:border-slate-700/80 dark:bg-slate-950/45 dark:text-amber-300">
                                <span className="inline-flex h-5 w-5 items-center justify-center rounded-full bg-amber-100 text-amber-700 dark:bg-amber-950/60 dark:text-amber-300">
                                    <svg xmlns="http://www.w3.org/2000/svg" width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                                        <rect x="3" y="11" width="18" height="11" rx="2" ry="2" />
                                        <path d="M7 11V7a5 5 0 0 1 10 0v4" />
                                    </svg>
                                </span>
                                Inserir PIN
                            </div>
                            <div>
                                <h2
                                    id="protected-document-pin-title"
                                    className="text-2xl font-semibold text-slate-900 dark:text-slate-50"
                                >
                                    Digite o PIN para abrir a nota
                                </h2>
                                <p className="mt-2 text-sm leading-6 text-slate-600 dark:text-slate-300">
                                    O acesso desta rota esta protegido. Assim que o PIN for aceito, o editor abre na mesma sequencia.
                                </p>
                            </div>
                        </div>
                        <p className="app-code shrink-0 text-[11px]">
                            /{decodeURIComponent(documentId)}
                        </p>
                    </div>

                    <form onSubmit={onSubmit} className="mt-6 space-y-4">
                        {error ? (
                            <div className="rounded-2xl border border-red-200/90 bg-red-50/90 p-4 dark:border-red-900/40 dark:bg-red-950/40">
                                <p className="text-sm text-red-700 dark:text-red-300">
                                    {error}
                                </p>
                            </div>
                        ) : null}
                        <div className="relative">
                            <input
                                type={showPassword ? "text" : "password"}
                                value={pin}
                                onChange={(event) => onPinChange(event.target.value)}
                                placeholder="••••••"
                                autoFocus
                                disabled={isVerifying}
                                className="w-full rounded-2xl border border-white/70 bg-white/78 px-5 py-4 pr-14 text-center text-2xl font-medium tracking-[0.32em] text-slate-900 outline-none transition placeholder:text-slate-400 focus:border-sky-400 focus:ring-4 focus:ring-sky-100 disabled:opacity-50 dark:border-slate-700/80 dark:bg-slate-950/55 dark:text-white dark:placeholder:text-slate-600 dark:focus:border-sky-500 dark:focus:ring-sky-950/50"
                            />
                            <button
                                type="button"
                                onClick={onTogglePassword}
                                disabled={isVerifying}
                                className="absolute right-4 top-1/2 -translate-y-1/2 p-1.5 text-slate-400 transition hover:text-slate-600 disabled:opacity-50 dark:hover:text-slate-300"
                                aria-label={showPassword ? "Ocultar PIN" : "Mostrar PIN"}
                            >
                                {showPassword ? (
                                    <svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"></path><line x1="1" y1="1" x2="23" y2="23"></line></svg>
                                ) : (
                                    <svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"></path><circle cx="12" cy="12" r="3"></circle></svg>
                                )}
                            </button>
                        </div>
                        <div className="flex flex-col gap-3 sm:flex-row">
                            <button
                                type="submit"
                                disabled={!pin.trim() || isVerifying}
                                className="inline-flex flex-1 items-center justify-center rounded-2xl bg-slate-900 px-4 py-3 text-sm font-semibold text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-slate-200"
                            >
                                {isVerifying ? (
                                    <span className="flex items-center gap-2">
                                        <span className="inline-block h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent dark:border-slate-900 dark:border-t-transparent" />
                                        Validando
                                    </span>
                                ) : "Desbloquear nota"}
                            </button>
                            <a
                                href="/"
                                className="inline-flex items-center justify-center rounded-2xl border border-white/70 bg-white/68 px-4 py-3 text-sm font-medium text-slate-700 no-underline transition hover:bg-white/88 dark:border-slate-700/80 dark:bg-slate-950/40 dark:text-slate-300 dark:hover:bg-slate-900/80"
                            >
                                Voltar ao inicio
                            </a>
                        </div>
                    </form>
                </div>
            </div>
        </div>
    );
}

function SyncedDocumentContent({
    documentId,
    children,
}: {
    documentId: string;
    children: React.ReactNode;
}) {
    const connectionStatus = useConnectionStatus();
    const [hasOpened, setHasOpened] = useState(false);

    useEffect(() => {
        if (connectionStatus === "connected") {
            setHasOpened(true);
        }
    }, [connectionStatus]);

    if (!hasOpened) {
        const detail =
            connectionStatus === "offline"
                ? "Sem conexao no navegador. A nota abre assim que a rede voltar."
                : connectionStatus === "error"
                    ? "Nao foi possivel abrir o canal em tempo real. Tentando reconectar."
                    : connectionStatus === "handshaking"
                        ? "Canal aberto. Trocando o estado inicial do documento."
                        : "Solicitando token e abrindo o canal em tempo real.";

        return (
            <DocumentLoadingProgress
                documentId={documentId}
                stage="sync"
                title="Sincronizando a nota"
                description="A seguranca ja foi validada. Agora estamos conectando o documento ao backend em tempo real."
                detail={detail}
            />
        );
    }

    return <>{children}</>;
}

/**
 * Provider seguro que verifica autenticação antes de carregar o documento Y-Sweet.
 * Suporta 3 modos de visibilidade:
 *   "public"          → leitura e escrita sem autenticação
 *   "public-readonly" → leitura pública, edição requer PIN
 *   "private"         → acesso completo requer PIN
 */
export function SecureDocumentProvider({
    documentId,
    syncDocumentId,
    displayDocumentId,
    children,
}: SecureDocumentProviderProps) {
    const [status, setStatus] = useState<SecurityStatus | null>(null);
    const [isLoading, setIsLoading] = useState(true);
    const [securityError, setSecurityError] = useState<string | null>(null);

    // PIN para acesso (documentos privados) ou para desbloquear edição (public-readonly)
    const [pin, setPin] = useState("");
    const [error, setError] = useState("");
    const [isVerifying, setIsVerifying] = useState(false);
    const [showPassword, setShowPassword] = useState(false);

    // Modal de desbloqueio de edição (public-readonly)
    const [showEditModal, setShowEditModal] = useState(false);
    const [editPin, setEditPin] = useState("");
    const [editError, setEditError] = useState("");
    const [isVerifyingEdit, setIsVerifyingEdit] = useState(false);
    const [showEditPassword, setShowEditPassword] = useState(false);
    const providerDocumentId = syncDocumentId ?? documentId;
    const visibleDocumentId = displayDocumentId ?? documentId;

    const authEndpoint = async (): Promise<DocumentTokenPayload> => {
        const res = await fetch(`/api/documents/${encodeURIComponent(providerDocumentId)}/token`, {
            cache: "no-store",
            credentials: "same-origin",
        });
        if (!res.ok) {
            if (res.status === 401) throw new Error("Unauthorized");
            throw new Error("Failed to get document token");
        }
        return res.json();
    };

    const checkSecurity = useCallback(async () => {
        try {
            setSecurityError(null);

            const response = await fetch(`/api/documents/${encodeURIComponent(documentId)}/security`, {
                cache: "no-store",
                credentials: "same-origin",
            });
            if (response.ok) {
                const data: SecurityStatus = await response.json();
                setStatus(data);
            } else {
                setStatus(null);
                setSecurityError(`Falha ao verificar seguranca da nota (${response.status}).`);
            }
        } catch (err) {
            console.error("[SecureDocumentProvider] Erro ao verificar segurança:", err);
            setStatus(null);
            setSecurityError("Nao foi possivel verificar a seguranca da nota.");
        } finally {
            setIsLoading(false);
        }
    }, [documentId]);

    useEffect(() => {
        checkSecurity();
    }, [checkSecurity]);

    // Verificar PIN (para documentos privados)
    const handleVerifyPin = async (e: React.FormEvent) => {
        e.preventDefault();
        setError("");
        setIsVerifying(true);
        try {
            const response = await fetch(`/api/documents/${encodeURIComponent(documentId)}/verify-pin`, {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                cache: "no-store",
                credentials: "same-origin",
                body: JSON.stringify({ pin }),
            });
            const data = await response.json();
            if (data.success) {
                await checkSecurity();
            } else {
                setError(data.error || "PIN incorreto");
                setPin("");
            }
        } catch {
            setError("Erro ao verificar PIN");
        } finally {
            setIsVerifying(false);
        }
    };

    // Verificar PIN para desbloquear edição (public-readonly)
    const handleVerifyEditPin = async (e: React.FormEvent) => {
        e.preventDefault();
        setEditError("");
        setIsVerifyingEdit(true);
        try {
            const response = await fetch(`/api/documents/${encodeURIComponent(documentId)}/verify-pin`, {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                cache: "no-store",
                credentials: "same-origin",
                body: JSON.stringify({ pin: editPin }),
            });
            const data = await response.json();
            if (data.success) {
                setShowEditModal(false);
                setEditPin("");
                // re-fetch para pegar canEdit: true com o JWT agora setado
                await checkSecurity();
            } else {
                setEditError(data.error || "PIN incorreto");
                setEditPin("");
            }
        } catch {
            setEditError("Erro ao verificar PIN");
        } finally {
            setIsVerifyingEdit(false);
        }
    };

    // Loading
    if (isLoading) {
        return (
            <OpeningDocumentState
                documentId={visibleDocumentId}
                error={securityError}
                onRetry={() => {
                    setIsLoading(true);
                    void checkSecurity();
                }}
            />
        );
    }

    if (securityError) {
        return (
            <OpeningDocumentState
                documentId={visibleDocumentId}
                error={securityError}
                onRetry={() => {
                    setIsLoading(true);
                    void checkSecurity();
                }}
            />
        );
    }

    // Documento privado sem acesso: tela de PIN de acesso
    if (status?.isProtected && !status?.hasAccess) {
        return (
            <ProtectedDocumentState
                documentId={visibleDocumentId}
                pin={pin}
                error={error}
                isVerifying={isVerifying}
                showPassword={showPassword}
                onPinChange={setPin}
                onTogglePassword={() => setShowPassword((value) => !value)}
                onSubmit={handleVerifyPin}
            />
        );
    }

    const visibilityMode = status?.visibilityMode ?? "public";
    const canEdit = status?.canEdit ?? true;
    const isReadOnly = !canEdit;

    return (
        <DocumentSecurityContext.Provider value={{
            visibilityMode,
            isReadOnly,
            requestEdit: () => {
                setEditPin("");
                setEditError("");
                setShowEditModal(true);
            },
        }}>
            <YDocProvider
                docId={providerDocumentId}
                authEndpoint={authEndpoint}
                showDebuggerLink={false}
                warnOnClose={true}
            >
                <SyncedDocumentContent documentId={visibleDocumentId}>
                    <DocumentSyncLifecycle documentId={providerDocumentId} />
                    {children}

                    {/* Modal de desbloqueio de edição (public-readonly) */}
                    {showEditModal && (
                        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm" onClick={() => setShowEditModal(false)}>
                            <div className="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl shadow-xl p-6 w-full max-w-sm mx-4" onClick={e => e.stopPropagation()}>
                                <div className="flex justify-center mb-4">
                                    <div className="w-12 h-12 bg-amber-50 dark:bg-amber-900/30 rounded-full flex items-center justify-center">
                                        <svg xmlns="http://www.w3.org/2000/svg" width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="text-amber-600 dark:text-amber-400">
                                            <rect x="3" y="11" width="18" height="11" rx="2" ry="2"></rect>
                                            <path d="M7 11V7a5 5 0 0110 0v4"></path>
                                        </svg>
                                    </div>
                                </div>
                                <h2 className="text-base font-semibold text-slate-900 dark:text-slate-100 text-center mb-1">Desbloquear edição</h2>
                                <p className="text-sm text-slate-500 dark:text-slate-400 text-center mb-5">Digite o PIN para editar este documento</p>
                                <form onSubmit={handleVerifyEditPin} className="space-y-3">
                                    {editError && (
                                        <div className="p-2.5 rounded-lg bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800/50">
                                            <p className="text-sm text-red-600 dark:text-red-400 text-center">{editError}</p>
                                        </div>
                                    )}
                                    <div className="relative">
                                        <input
                                            type={showEditPassword ? "text" : "password"}
                                            value={editPin}
                                            onChange={e => setEditPin(e.target.value)}
                                            placeholder="••••••"
                                            autoFocus
                                            disabled={isVerifyingEdit}
                                            className="w-full px-4 py-3 text-center text-xl font-medium tracking-[0.3em] rounded-lg border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-800 focus:outline-none focus:ring-2 focus:ring-slate-900 dark:focus:ring-slate-100 focus:border-transparent text-slate-900 dark:text-white placeholder-slate-300 dark:placeholder:text-slate-600 transition"
                                        />
                                        <button type="button" onClick={() => setShowEditPassword(v => !v)} className="absolute right-3 top-1/2 -translate-y-1/2 p-1.5 text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 transition" aria-label={showEditPassword ? "Ocultar PIN" : "Mostrar PIN"}>
                                            {showEditPassword ? (
                                                <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M17.94 17.94A10.07 10.07 0 0112 20c-7 0-11-8-11-8a18.45 18.45 0 015.06-5.94M9.9 4.24A9.12 9.12 0 0112 4c7 0 11 8 11 8a18.5 18.5 0 01-2.16 3.19m-6.72-1.07a3 3 0 11-4.24-4.24"></path><line x1="1" y1="1" x2="23" y2="23"></line></svg>
                                            ) : (
                                                <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"></path><circle cx="12" cy="12" r="3"></circle></svg>
                                            )}
                                        </button>
                                    </div>
                                    <div className="flex gap-2">
                                        <button type="button" onClick={() => setShowEditModal(false)} className="flex-1 py-2.5 text-sm rounded-lg border border-slate-200 dark:border-slate-700 text-slate-700 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-800 transition">Cancelar</button>
                                        <button type="submit" disabled={!editPin.trim() || isVerifyingEdit} className="flex-1 py-2.5 text-sm font-semibold rounded-lg bg-slate-900 dark:bg-slate-100 text-white dark:text-slate-900 hover:bg-slate-800 dark:hover:bg-slate-200 disabled:opacity-50 disabled:cursor-not-allowed transition">
                                            {isVerifyingEdit ? (
                                                <span className="flex items-center justify-center gap-2">
                                                    <span className="inline-block w-4 h-4 border-2 border-white dark:border-slate-900 border-t-transparent rounded-full animate-spin" />
                                                    Verificando...
                                                </span>
                                            ) : "Desbloquear"}
                                        </button>
                                    </div>
                                </form>
                            </div>
                        </div>
                    )}
                </SyncedDocumentContent>
            </YDocProvider>
        </DocumentSecurityContext.Provider>
    );
}
