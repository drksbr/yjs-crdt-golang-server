"use client";

import { ReactNode } from "react";
import { ChecklistCheckedState } from "@/lib/checklistModel";
import { ChecklistTreeNode } from "@/lib/checklistTree";

interface ChecklistRowProps {
    node: ChecklistTreeNode;
    checkedState: ChecklistCheckedState;
    isEditing: boolean;
    editText: string;
    isReadOnly: boolean;
    onStartEdit: (node: ChecklistTreeNode) => void;
    onEditChange: (value: string) => void;
    onCommitEdit: (node: ChecklistTreeNode, options?: { createSibling?: boolean; createChild?: boolean }) => void;
    onCancelEdit: () => void;
    onToggleCheck: (node: ChecklistTreeNode) => void;
    onToggleCollapsed: (node: ChecklistTreeNode) => void;
    onAddChild: (node: ChecklistTreeNode) => void;
    onAddSibling: (node: ChecklistTreeNode) => void;
    onIndent: (node: ChecklistTreeNode) => void;
    onOutdent: (node: ChecklistTreeNode) => void;
    onDelete: (node: ChecklistTreeNode) => void;
}

function ActionButton({
    label,
    disabled,
    onClick,
    children,
}: {
    label: string;
    disabled?: boolean;
    onClick: () => void;
    children: ReactNode;
}) {
    return (
        <button
            type="button"
            onClick={onClick}
            disabled={disabled}
            aria-label={label}
            title={label}
            className="rounded-lg p-1.5 text-slate-400 transition hover:bg-slate-100 hover:text-slate-700 disabled:cursor-not-allowed disabled:opacity-40 dark:text-slate-500 dark:hover:bg-slate-700 dark:hover:text-slate-300"
        >
            {children}
        </button>
    );
}

export function ChecklistRow({
    node,
    checkedState,
    isEditing,
    editText,
    isReadOnly,
    onStartEdit,
    onEditChange,
    onCommitEdit,
    onCancelEdit,
    onToggleCheck,
    onToggleCollapsed,
    onAddChild,
    onAddSibling,
    onIndent,
    onOutdent,
    onDelete,
}: ChecklistRowProps) {
    const hasChildren = node.children.length > 0;
    const indentPadding = Math.min(node.depth, 6) * 20;

    return (
        <div style={{ paddingLeft: indentPadding }}>
            <div
                className={`group flex items-start gap-2 rounded-2xl border p-3 transition ${checkedState === "checked"
                    ? "border-slate-100 bg-slate-50 dark:border-slate-800 dark:bg-slate-800/30"
                    : "border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-900 hover:border-slate-300 dark:hover:border-slate-600"
                    }`}
            >
                <button
                    type="button"
                    onClick={() => hasChildren && onToggleCollapsed(node)}
                    disabled={!hasChildren}
                    aria-label={node.collapsed ? "Expandir subtarefas" : "Recolher subtarefas"}
                    className={`mt-0.5 flex h-5 w-5 flex-shrink-0 items-center justify-center rounded-md text-slate-400 transition ${hasChildren
                        ? "hover:bg-slate-100 hover:text-slate-700 dark:hover:bg-slate-700 dark:hover:text-slate-200"
                        : "opacity-0"
                        }`}
                >
                    {hasChildren ? (
                        <svg
                            xmlns="http://www.w3.org/2000/svg"
                            width="14"
                            height="14"
                            viewBox="0 0 24 24"
                            fill="none"
                            stroke="currentColor"
                            strokeWidth="2"
                            strokeLinecap="round"
                            strokeLinejoin="round"
                            className={`transition-transform ${node.collapsed ? "-rotate-90" : "rotate-0"}`}
                        >
                            <polyline points="6 9 12 15 18 9" />
                        </svg>
                    ) : null}
                </button>

                <button
                    type="button"
                    onClick={() => onToggleCheck(node)}
                    disabled={isReadOnly}
                    aria-label={checkedState === "checked" ? "Desmarcar tarefa" : "Marcar tarefa"}
                    className={`mt-0.5 flex h-5 w-5 flex-shrink-0 items-center justify-center rounded border-2 transition ${checkedState === "checked"
                        ? "border-emerald-500 bg-emerald-500"
                        : checkedState === "mixed"
                            ? "border-emerald-500 bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300"
                            : "border-slate-300 hover:border-emerald-500 dark:border-slate-600"
                        }`}
                >
                    {checkedState === "checked" ? (
                        <svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="white" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round">
                            <polyline points="20 6 9 17 4 12" />
                        </svg>
                    ) : checkedState === "mixed" ? (
                        <svg xmlns="http://www.w3.org/2000/svg" width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round">
                            <line x1="5" y1="12" x2="19" y2="12" />
                        </svg>
                    ) : null}
                </button>

                <div className="min-w-0 flex-1">
                    {isEditing ? (
                        <input
                            autoFocus
                            value={editText}
                            onChange={(event) => onEditChange(event.target.value)}
                            onBlur={() => onCommitEdit(node)}
                            onKeyDown={(event) => {
                                if (event.key === "Enter") {
                                    event.preventDefault();
                                    onCommitEdit(node, { createSibling: true });
                                }
                                if ((event.metaKey || event.ctrlKey) && event.key === "Enter") {
                                    event.preventDefault();
                                    onCommitEdit(node, { createChild: true });
                                }
                                if (event.key === "Tab") {
                                    event.preventDefault();
                                    if (event.shiftKey) {
                                        onOutdent(node);
                                    } else {
                                        onIndent(node);
                                    }
                                }
                                if (event.key === "Escape") {
                                    event.preventDefault();
                                    onCancelEdit();
                                }
                            }}
                            className="w-full border-b border-slate-400 bg-transparent pb-1 text-sm text-slate-900 outline-none dark:border-slate-500 dark:text-slate-100"
                        />
                    ) : (
                        <button
                            type="button"
                            onClick={() => onStartEdit(node)}
                            disabled={isReadOnly}
                            className={`w-full text-left text-sm transition ${checkedState === "checked"
                                ? "text-slate-400 line-through dark:text-slate-500"
                                : "text-slate-900 dark:text-slate-100"
                                }`}
                        >
                            {node.text.trim().length > 0 ? node.text : "Nova tarefa"}
                        </button>
                    )}
                </div>

                <div className="flex items-center gap-1 opacity-100 transition md:opacity-0 md:group-hover:opacity-100">
                    <ActionButton label="Adicionar subtarefa" disabled={isReadOnly} onClick={() => onAddChild(node)}>
                        <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                            <path d="M12 5v10" />
                            <path d="M7 10h10" />
                            <path d="M7 19h6" />
                            <path d="m11 15 2 2 4-4" />
                        </svg>
                    </ActionButton>
                    <ActionButton label="Adicionar tarefa abaixo" disabled={isReadOnly} onClick={() => onAddSibling(node)}>
                        <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                            <line x1="12" y1="5" x2="12" y2="19" />
                            <line x1="5" y1="12" x2="19" y2="12" />
                        </svg>
                    </ActionButton>
                    <ActionButton label="Indentar" disabled={isReadOnly} onClick={() => onIndent(node)}>
                        <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                            <path d="M3 7h11" />
                            <path d="M3 12h11" />
                            <path d="M3 17h11" />
                            <path d="m14 9 3 3-3 3" />
                        </svg>
                    </ActionButton>
                    <ActionButton label="Desindentar" disabled={isReadOnly || node.parentId === null} onClick={() => onOutdent(node)}>
                        <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                            <path d="M10 9 7 12l3 3" />
                            <path d="M21 7H10" />
                            <path d="M21 12H3" />
                            <path d="M21 17H10" />
                        </svg>
                    </ActionButton>
                    <ActionButton label="Excluir tarefa" disabled={isReadOnly} onClick={() => onDelete(node)}>
                        <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                            <path d="M3 6h18" />
                            <path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6" />
                            <path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2" />
                        </svg>
                    </ActionButton>
                </div>
            </div>
        </div>
    );
}
