"use client";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

export type DocumentLoadStage = "application" | "route" | "security" | "protected" | "sync" | "editor";

const loadSteps: Array<{
    id: DocumentLoadStage;
    label: string;
    description: string;
}> = [
    {
        id: "application",
        label: "Aplicação",
        description: "Preparando interface e recursos essenciais.",
    },
    {
        id: "route",
        label: "Rota",
        description: "Interpretando nota, subnota e tipo de documento.",
    },
    {
        id: "security",
        label: "Segurança",
        description: "Conferindo visibilidade, PIN e permissões.",
    },
    {
        id: "sync",
        label: "Sincronização",
        description: "Abrindo sessão colaborativa em tempo real.",
    },
    {
        id: "editor",
        label: "Editor",
        description: "Carregando o editor adequado para esta nota.",
    },
];

function stageIndex(stage: DocumentLoadStage) {
    if (stage === "protected") return 2;
    return Math.max(0, loadSteps.findIndex((step) => step.id === stage));
}

function stepState(stepIndex: number, activeIndex: number, stage: DocumentLoadStage) {
    if (stage === "protected" && stepIndex === activeIndex) return "waiting";
    if (stepIndex < activeIndex) return "complete";
    if (stepIndex === activeIndex) return "active";
    return "pending";
}

export function DocumentLoadingSteps({
    stage,
    compact = false,
    className,
}: {
    stage: DocumentLoadStage;
    compact?: boolean;
    className?: string;
}) {
    const activeIndex = stageIndex(stage);

    return (
        <div className={cn("flex flex-col gap-3", className)}>
            {loadSteps.map((step, index) => {
                const state = stepState(index, activeIndex, stage);
                const isComplete = state === "complete";
                const isActive = state === "active";
                const isWaiting = state === "waiting";

                return (
                    <div
                        key={step.id}
                        className={cn(
                            "flex items-center gap-3 rounded-xl border px-4 py-3 transition",
                            isActive && "border-sky-200 bg-sky-50/90 text-slate-900 dark:border-sky-900/50 dark:bg-sky-950/30 dark:text-slate-100",
                            isComplete && "border-emerald-200 bg-emerald-50/90 text-slate-900 dark:border-emerald-900/50 dark:bg-emerald-950/30 dark:text-slate-100",
                            isWaiting && "border-amber-200 bg-amber-50/90 text-slate-900 dark:border-amber-900/50 dark:bg-amber-950/30 dark:text-slate-100",
                            state === "pending" && "border-slate-200/80 bg-white/65 text-slate-500 dark:border-slate-800 dark:bg-slate-900/45 dark:text-slate-400",
                        )}
                    >
                        <span
                            className={cn(
                                "flex size-8 shrink-0 items-center justify-center rounded-full border text-xs font-semibold",
                                isActive && "border-sky-300 bg-white text-sky-600 dark:border-sky-800 dark:bg-slate-900 dark:text-sky-300",
                                isComplete && "border-emerald-300 bg-white text-emerald-600 dark:border-emerald-800 dark:bg-slate-900 dark:text-emerald-300",
                                isWaiting && "border-amber-300 bg-white text-amber-700 dark:border-amber-800 dark:bg-slate-900 dark:text-amber-300",
                                state === "pending" && "border-slate-200 bg-slate-100 text-slate-400 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-500",
                            )}
                        >
                            {isComplete ? "✓" : isActive ? (
                                <span className="inline-block size-3.5 animate-spin rounded-full border-2 border-current border-t-transparent" />
                            ) : isWaiting ? "!" : index + 1}
                        </span>
                        <span className="min-w-0 flex-1">
                            <span className="block text-sm font-semibold">
                                {step.label}
                            </span>
                            {!compact ? (
                                <span className="mt-0.5 block text-xs leading-5 opacity-75">
                                    {step.description}
                                </span>
                            ) : null}
                        </span>
                    </div>
                );
            })}
        </div>
    );
}

export function DocumentLoadingProgress({
    stage,
    documentId,
    title,
    description,
    detail,
    error,
    onRetry,
    mode = "screen",
    className,
}: {
    stage: DocumentLoadStage;
    documentId?: string;
    title?: string;
    description?: string;
    detail?: string;
    error?: string | null;
    onRetry?: () => void;
    mode?: "screen" | "panel";
    className?: string;
}) {
    const activeIndex = stageIndex(stage);
    const progress = Math.round(((activeIndex + 1) / loadSteps.length) * 100);
    const activeStep = loadSteps[activeIndex] ?? loadSteps[0];

    return (
        <div
            className={cn(
                "flex min-h-0 flex-1 items-center justify-center bg-[var(--app-bg)] px-6 py-8 text-[var(--app-text)] dark:bg-slate-950",
                mode === "screen" && "min-h-dvh",
                className,
            )}
        >
            <div className="w-full max-w-2xl">
                <div className="surface-glass p-6 md:p-8">
                    <div className="flex flex-col gap-6">
                        <div className="flex flex-col gap-4">
                            <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                                <div className="eyebrow border-slate-200/80 bg-white/70 text-slate-600 dark:border-slate-700/70 dark:bg-slate-900/65 dark:text-slate-300">
                                    <span className="inline-block size-2 rounded-full bg-sky-500" />
                                    Abrindo nota
                                </div>
                                {documentId ? (
                                    <p className="app-code max-w-full truncate text-xs">
                                        /{decodeURIComponent(documentId)}
                                    </p>
                                ) : null}
                            </div>

                            <div className="flex flex-col gap-2">
                                <h1 className="text-2xl font-semibold text-slate-900 dark:text-slate-50 md:text-3xl">
                                    {title ?? activeStep.label}
                                </h1>
                                <p className="max-w-xl text-sm leading-6 text-slate-600 dark:text-slate-300">
                                    {description ?? activeStep.description}
                                </p>
                            </div>
                        </div>

                        <div className="flex flex-col gap-3">
                            <div className="flex items-center justify-between text-xs font-medium uppercase tracking-[0.14em] text-slate-500 dark:text-slate-400">
                                <span>Progresso</span>
                                <span>{progress}%</span>
                            </div>
                            <div className="h-2 overflow-hidden rounded-full bg-slate-200/80 dark:bg-slate-800">
                                <div
                                    className="h-full rounded-full bg-slate-900 transition-[width] duration-500 dark:bg-slate-100"
                                    style={{ width: `${progress}%` }}
                                />
                            </div>
                        </div>

                        <DocumentLoadingSteps stage={stage} compact={mode === "panel"} />

                        {error ? (
                            <div className="rounded-xl border border-red-200 bg-red-50/90 p-4 dark:border-red-900/40 dark:bg-red-950/40">
                                <p className="text-sm font-medium text-red-700 dark:text-red-300">
                                    Nao foi possivel concluir esta etapa.
                                </p>
                                <p className="mt-2 text-sm leading-6 text-red-700/90 dark:text-red-200/90">
                                    {error}
                                </p>
                            </div>
                        ) : (
                            <div className="flex flex-col gap-3 rounded-xl border border-slate-200/80 bg-white/70 px-4 py-3 text-sm text-slate-500 dark:border-slate-800 dark:bg-slate-900/45 dark:text-slate-400 sm:flex-row sm:items-center sm:justify-between">
                                <span>{detail ?? activeStep.description}</span>
                                <span className="inline-flex items-center gap-2 font-medium text-slate-700 dark:text-slate-200">
                                    <span className="inline-block size-2.5 rounded-full bg-sky-500 animate-pulse" />
                                    Em andamento
                                </span>
                            </div>
                        )}

                        {onRetry ? (
                            <div>
                                <Button type="button" variant="outline" onClick={onRetry}>
                                    Tentar novamente
                                </Button>
                            </div>
                        ) : null}
                    </div>
                </div>
            </div>
        </div>
    );
}
