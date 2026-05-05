"use client";

import { Surface } from "./ui/Surface";

const collaborators = [
    { name: "Ana", role: "Produto", color: "bg-blue-500" },
    { name: "Carlos", role: "Dev", color: "bg-emerald-500" },
    { name: "Maria", role: "Design", color: "bg-orange-500" },
];

const noteLines = [
    "Ajustar prioridades do plantão da semana.",
    "Separar notas por paciente e manter checklist operacional.",
    "Anexar arquivos de referência e áudios rápidos no mesmo fluxo.",
];

const checklistItems = [
    { label: "Liberar subnota da triagem", done: true },
    { label: "Atualizar quadro do turno", done: true },
    { label: "Revisar pendências da alta", done: false },
];

const kanbanColumns = [
    { title: "Agora", count: 3, tone: "bg-blue-50 text-blue-700 dark:bg-blue-950/40 dark:text-blue-300" },
    { title: "Hoje", count: 5, tone: "bg-amber-50 text-amber-700 dark:bg-amber-950/40 dark:text-amber-300" },
    { title: "Depois", count: 2, tone: "bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300" },
];

export function CollaborationDemo() {
    return (
        <div className="w-full">
            <Surface className="overflow-hidden">
                <div className="flex flex-col gap-3 border-b border-slate-200 bg-slate-50 px-5 py-4 dark:border-slate-800 dark:bg-slate-900/80 sm:flex-row sm:items-center sm:justify-between">
                    <div className="min-w-0">
                        <div className="flex items-center gap-2">
                            <div className="flex gap-1.5">
                                <span className="h-2.5 w-2.5 rounded-full bg-slate-300 dark:bg-slate-600" />
                                <span className="h-2.5 w-2.5 rounded-full bg-slate-300 dark:bg-slate-600" />
                                <span className="h-2.5 w-2.5 rounded-full bg-slate-300 dark:bg-slate-600" />
                            </div>
                            <span className="truncate font-mono text-xs text-slate-500 dark:text-slate-400">
                                /plantao-noturno/operacao
                            </span>
                        </div>
                        <p className="mt-2 text-sm font-medium text-slate-900 dark:text-slate-100">
                            Nota principal com subdocumentos, checklist, kanban e anexos.
                        </p>
                    </div>

                    <div className="flex flex-wrap gap-2">
                        {collaborators.map((person) => (
                            <span
                                key={person.name}
                                className="inline-flex items-center gap-2 rounded-full border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-600 dark:border-slate-700 dark:bg-slate-950 dark:text-slate-300"
                            >
                                <span className={`h-2 w-2 rounded-full ${person.color}`} />
                                {person.name}
                                <span className="text-slate-400 dark:text-slate-500">· {person.role}</span>
                            </span>
                        ))}
                    </div>
                </div>

                <div className="grid gap-4 p-5 lg:grid-cols-[minmax(0,1.15fr)_minmax(17rem,0.85fr)]">
                    <div className="rounded-3xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-950">
                        <div className="flex items-start justify-between gap-4">
                            <div>
                                <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500 dark:text-slate-400">
                                    nota ativa
                                </p>
                                <h3 className="mt-2 text-lg font-semibold text-slate-950 dark:text-slate-100">
                                    Coordenação do turno e comunicação rápida
                                </h3>
                            </div>
                            <span className="rounded-full bg-[var(--app-accent-soft)] px-3 py-1 text-xs font-semibold text-[var(--app-accent)]">
                                sync online
                            </span>
                        </div>

                        <div className="mt-5 space-y-3">
                            {noteLines.map((line) => (
                                <div
                                    key={line}
                                    className="rounded-2xl border border-slate-200 bg-slate-50 px-4 py-3 text-sm leading-6 text-slate-700 dark:border-slate-800 dark:bg-slate-900 dark:text-slate-300"
                                >
                                    {line}
                                </div>
                            ))}
                        </div>

                        <div className="mt-5 flex flex-wrap gap-2">
                            {["Arquivos", "Áudio", "PIN", "Compartilhar"].map((item) => (
                                <span
                                    key={item}
                                    className="rounded-full border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-600 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-300"
                                >
                                    {item}
                                </span>
                            ))}
                        </div>
                    </div>

                    <div className="grid gap-4">
                        <div className="rounded-3xl border border-slate-200 bg-slate-50 p-4 dark:border-slate-800 dark:bg-slate-900">
                            <div className="flex items-center justify-between gap-3">
                                <div>
                                    <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500 dark:text-slate-400">
                                        checklist
                                    </p>
                                    <p className="mt-1 text-sm font-medium text-slate-900 dark:text-slate-100">
                                        Pendências do plantão
                                    </p>
                                </div>
                                <span className="rounded-full bg-white px-2.5 py-1 text-xs font-semibold text-slate-600 dark:bg-slate-950 dark:text-slate-300">
                                    2/3
                                </span>
                            </div>

                            <div className="mt-4 space-y-2.5">
                                {checklistItems.map((item) => (
                                    <div
                                        key={item.label}
                                        className="flex items-center gap-3 rounded-2xl bg-white px-3 py-2.5 text-sm text-slate-700 dark:bg-slate-950 dark:text-slate-300"
                                    >
                                        <span
                                            className={`inline-flex h-5 w-5 items-center justify-center rounded-md border text-[11px] ${item.done
                                                ? "border-slate-900 bg-slate-900 text-white dark:border-slate-100 dark:bg-slate-100 dark:text-slate-900"
                                                : "border-slate-300 text-transparent dark:border-slate-600"
                                                }`}
                                        >
                                            ✓
                                        </span>
                                        <span className={item.done ? "line-through opacity-70" : ""}>{item.label}</span>
                                    </div>
                                ))}
                            </div>
                        </div>

                        <div className="rounded-3xl border border-slate-200 bg-slate-50 p-4 dark:border-slate-800 dark:bg-slate-900">
                            <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500 dark:text-slate-400">
                                quadro kanban
                            </p>
                            <div className="mt-4 grid gap-2 sm:grid-cols-3">
                                {kanbanColumns.map((column) => (
                                    <div
                                        key={column.title}
                                        className="rounded-2xl border border-slate-200 bg-white p-3 dark:border-slate-800 dark:bg-slate-950"
                                    >
                                        <div className={`inline-flex rounded-full px-2.5 py-1 text-xs font-semibold ${column.tone}`}>
                                            {column.title}
                                        </div>
                                        <p className="mt-3 text-2xl font-semibold tracking-tight text-slate-950 dark:text-slate-100">
                                            {column.count}
                                        </p>
                                        <p className="mt-1 text-xs text-slate-500 dark:text-slate-400">
                                            cartões vinculados
                                        </p>
                                    </div>
                                ))}
                            </div>
                        </div>
                    </div>
                </div>
            </Surface>

            <p className="mt-4 text-center text-sm text-slate-500 dark:text-slate-400">
                Uma visualização única para nota principal, subdocumentos e trabalho em andamento.
            </p>
        </div>
    );
}
