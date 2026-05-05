'use client'

import { useMap } from '@/lib/collab/react'
import { useEffect, useRef, useState } from 'react'
import { ChecklistComposer } from '@/components/checklist/ChecklistComposer'
import { ChecklistTree } from '@/components/checklist/ChecklistTree'
import { EditorShell } from '@/components/EditorShell'
import { getChecklistStateKey } from '@/lib/noteStateAdapters'
import {
    addChecklistChildItem,
    addChecklistRootItem,
    addChecklistSiblingItem,
    deleteChecklistItem,
    ensureChecklistV2,
    indentChecklistItem,
    outdentChecklistItem,
    readChecklistItems,
    removeChecklistItemIfBlank,
    toggleChecklistCollapsed,
    toggleChecklistItem,
    updateChecklistItemText,
} from '@/lib/checklistStorage'
import { ChecklistTreeNode, buildChecklistTree, getChecklistCheckedState, getChecklistStats } from '@/lib/checklistTree'
import { useDocumentSecurity } from '@/lib/documentSecurityContext'
import { useDebouncedLastModified } from '@/lib/useDocumentMeta'

interface ChecklistEditorProps {
    subdocumentName: string
    isReadOnly?: boolean
}

export function ChecklistEditor({ subdocumentName, isReadOnly: isReadOnlyProp }: ChecklistEditorProps) {
    const { isReadOnly: ctxReadOnly } = useDocumentSecurity()
    const isReadOnly = isReadOnlyProp ?? ctxReadOnly
    const mapKey = getChecklistStateKey(subdocumentName)
    const itemsMap = useMap(mapKey)
    const touchLastModified = useDebouncedLastModified(subdocumentName)
    const [newText, setNewText] = useState('')
    const [editingId, setEditingId] = useState<string | null>(null)
    const [editText, setEditText] = useState('')
    const [editingInitialText, setEditingInitialText] = useState('')
    const commitLockRef = useRef<string | null>(null)

    useEffect(() => {
        if (itemsMap) {
            ensureChecklistV2(itemsMap)
        }
    }, [itemsMap])

    useEffect(() => {
        if (!itemsMap) return
        const handler = (_event: any, transaction: { local?: boolean }) => {
            if (!transaction?.local) return
            touchLastModified()
        }
        itemsMap.observe(handler)
        return () => itemsMap.unobserve(handler)
    }, [itemsMap, touchLastModified])

    const items = readChecklistItems(itemsMap)
    const tree = buildChecklistTree(items)
    const stats = getChecklistStats(items)
    const checkedStates = Object.fromEntries(
        items.map((item) => [item.id, getChecklistCheckedState(items, item.id)]),
    )

    useEffect(() => {
        if (editingId && !items.some((item) => item.id === editingId)) {
            setEditingId(null)
            setEditText('')
            setEditingInitialText('')
        }
    }, [editingId, items])

    const startEditing = (node: ChecklistTreeNode) => {
        setEditingId(node.id)
        setEditText(node.text)
        setEditingInitialText(node.text)
    }

    const finishEditing = (
        node: ChecklistTreeNode,
        options?: { createSibling?: boolean; createChild?: boolean },
    ) => {
        if (!itemsMap || commitLockRef.current === node.id) {
            return
        }

        commitLockRef.current = node.id

        const trimmed = editText.trim()
        if (trimmed.length === 0) {
            removeChecklistItemIfBlank(itemsMap, node.id)
            setEditingId(null)
            setEditText('')
            setEditingInitialText('')
            window.setTimeout(() => {
                if (commitLockRef.current === node.id) {
                    commitLockRef.current = null
                }
            }, 0)
            return
        }

        updateChecklistItemText(itemsMap, node.id, trimmed)
        setEditingId(null)
        setEditText('')
        setEditingInitialText('')

        const nextItem = options?.createChild
            ? addChecklistChildItem(itemsMap, node.id, '')
            : options?.createSibling
                ? addChecklistSiblingItem(itemsMap, node.id, '')
                : null

        if (nextItem) {
            setEditingId(nextItem.id)
            setEditText(nextItem.text)
            setEditingInitialText(nextItem.text)
        }

        window.setTimeout(() => {
            if (commitLockRef.current === node.id) {
                commitLockRef.current = null
            }
        }, 0)
    }

    const cancelEditing = () => {
        if (itemsMap && editingId && editingInitialText.trim().length === 0) {
            removeChecklistItemIfBlank(itemsMap, editingId)
        }
        setEditingId(null)
        setEditText('')
        setEditingInitialText('')
    }

    const handleAddRoot = () => {
        if (!itemsMap || !newText.trim()) return
        addChecklistRootItem(itemsMap, newText.trim())
        setNewText('')
    }

    const handleAddChild = (node: ChecklistTreeNode) => {
        if (!itemsMap || isReadOnly) return
        const nextItem = addChecklistChildItem(itemsMap, node.id, '')
        if (nextItem) {
            setEditingId(nextItem.id)
            setEditText(nextItem.text)
            setEditingInitialText(nextItem.text)
        }
    }

    const handleAddSibling = (node: ChecklistTreeNode) => {
        if (!itemsMap || isReadOnly) return
        const nextItem = addChecklistSiblingItem(itemsMap, node.id, '')
        if (nextItem) {
            setEditingId(nextItem.id)
            setEditText(nextItem.text)
            setEditingInitialText(nextItem.text)
        }
    }

    const toolbar = (
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div>
                <span className="text-xs font-medium uppercase tracking-[0.14em] text-slate-500 dark:text-slate-400">
                    {stats.total === 0 ? 'Nenhuma tarefa' : `${stats.completed} de ${stats.total} concluída${stats.completed !== 1 ? 's' : ''}`}
                </span>
                <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">
                    Use subtarefas, indentação e colapso para organizar a execução.
                </p>
            </div>
            <div className="flex items-center gap-3">
                <div className="h-2 w-full min-w-24 overflow-hidden rounded-full bg-slate-200 dark:bg-slate-700 sm:w-40">
                    <div
                        className="h-full bg-emerald-500 transition-all duration-300"
                        style={{ width: `${stats.percentage}%` }}
                    />
                </div>
                <span className="text-xs font-medium text-slate-500 dark:text-slate-400">
                    {stats.percentage}%
                </span>
            </div>
        </div>
    )

    return (
        <EditorShell
            variant="medium"
            toolbar={toolbar}
            frameClassName="overflow-hidden bg-white dark:bg-slate-900"
            contentClassName="overflow-hidden bg-white dark:bg-slate-900"
        >
            <div className="flex h-full min-h-0 flex-col">
                <div className="flex-1 overflow-auto px-4 py-4 sm:px-5 sm:py-5">
                    {tree.length === 0 ? (
                        <div className="py-12 text-center">
                            <p className="text-sm text-slate-400 dark:text-slate-500">
                                Nenhuma tarefa ainda. Crie a primeira para começar.
                            </p>
                        </div>
                    ) : (
                        <ChecklistTree
                            nodes={tree}
                            checkedStates={checkedStates}
                            editingId={editingId}
                            editText={editText}
                            isReadOnly={isReadOnly}
                            onStartEdit={startEditing}
                            onEditChange={setEditText}
                            onCommitEdit={finishEditing}
                            onCancelEdit={cancelEditing}
                            onToggleCheck={(node) => {
                                if (!itemsMap || isReadOnly) return
                                const nextChecked = checkedStates[node.id] !== 'checked'
                                toggleChecklistItem(itemsMap, node.id, nextChecked)
                            }}
                            onToggleCollapsed={(node) => {
                                if (!itemsMap || isReadOnly) return
                                toggleChecklistCollapsed(itemsMap, node.id)
                            }}
                            onAddChild={handleAddChild}
                            onAddSibling={handleAddSibling}
                            onIndent={(node) => {
                                if (!itemsMap || isReadOnly) return
                                indentChecklistItem(itemsMap, node.id)
                            }}
                            onOutdent={(node) => {
                                if (!itemsMap || isReadOnly) return
                                outdentChecklistItem(itemsMap, node.id)
                            }}
                            onDelete={(node) => {
                                if (!itemsMap || isReadOnly) return
                                deleteChecklistItem(itemsMap, node.id)
                                if (editingId === node.id) {
                                    setEditingId(null)
                                    setEditText('')
                                    setEditingInitialText('')
                                }
                            }}
                        />
                    )}
                </div>

                {!isReadOnly && (
                    <div className="border-t border-slate-200/80 px-4 py-4 dark:border-slate-700/80 sm:px-5">
                        <ChecklistComposer
                            value={newText}
                            onChange={setNewText}
                            onSubmit={handleAddRoot}
                        />
                    </div>
                )}
            </div>
        </EditorShell>
    )
}
