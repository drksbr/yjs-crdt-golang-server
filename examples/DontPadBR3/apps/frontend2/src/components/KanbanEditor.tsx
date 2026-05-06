'use client'

import { useMap } from '@/lib/collab/react'
import { useEffect, useState } from 'react'
import { EditorShell } from '@/components/EditorShell'
import { sanitizeDocumentId } from '@/lib/colors'
import { useDocumentSecurity } from '@/lib/documentSecurityContext'
import { getKanbanColumnsStateKey, getKanbanItemsStateKey } from '@/lib/noteStateAdapters'
import { useDebouncedLastModified } from '@/lib/useDocumentMeta'

interface KanbanEditorProps {
    documentId: string
    subdocumentName: string
    isReadOnly?: boolean
}

interface KanbanColumn {
    id: string
    title: string
    description: string
    color: string
    order: number
}

interface KanbanItem {
    id: string
    title: string
    description: string
    columnId: string
    order: number
    color?: string
    linkedDocumentId?: string
    linkedSubdocumentName?: string
}

interface ColorOption {
    label: string
    value: string
}

const KANBAN_COLOR_OPTIONS: ColorOption[] = [
    { label: 'Neutro', value: '#64748b' },
    { label: 'Vermelho', value: '#ef4444' },
    { label: 'Laranja', value: '#f97316' },
    { label: 'Amarelo', value: '#eab308' },
    { label: 'Verde', value: '#22c55e' },
    { label: 'Azul', value: '#3b82f6' },
    { label: 'Indigo', value: '#6366f1' },
    { label: 'Roxo', value: '#a855f7' },
    { label: 'Rosa', value: '#ec4899' },
]

const DEFAULT_COLUMN_COLOR = KANBAN_COLOR_OPTIONS[0].value

function generateId() {
    return `${Date.now()}-${Math.random().toString(36).slice(2, 7)}`
}

function clampChannel(value: number) {
    return Math.max(0, Math.min(255, Math.round(value)))
}

function normalizeHex(hex: string) {
    const trimmed = hex.trim()
    if (/^#[0-9a-fA-F]{6}$/.test(trimmed)) {
        return trimmed.toLowerCase()
    }
    if (/^#[0-9a-fA-F]{3}$/.test(trimmed)) {
        const a = trimmed.charAt(1)
        const b = trimmed.charAt(2)
        const c = trimmed.charAt(3)
        return `#${a}${a}${b}${b}${c}${c}`.toLowerCase()
    }
    return DEFAULT_COLUMN_COLOR
}

function hexToRgb(hex: string) {
    const normalized = normalizeHex(hex).slice(1)
    return {
        r: parseInt(normalized.slice(0, 2), 16),
        g: parseInt(normalized.slice(2, 4), 16),
        b: parseInt(normalized.slice(4, 6), 16),
    }
}

function rgbToHex(r: number, g: number, b: number) {
    return `#${[r, g, b].map(channel => clampChannel(channel).toString(16).padStart(2, '0')).join('')}`
}

function mixHex(source: string, target: string, targetWeight: number) {
    const from = hexToRgb(source)
    const to = hexToRgb(target)
    const amount = Math.max(0, Math.min(1, targetWeight))

    return rgbToHex(
        from.r + (to.r - from.r) * amount,
        from.g + (to.g - from.g) * amount,
        from.b + (to.b - from.b) * amount,
    )
}

function getRelativeLuminance(hex: string) {
    const { r, g, b } = hexToRgb(hex)
    const channels = [r, g, b].map(channel => {
        const value = channel / 255
        return value <= 0.03928 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4
    })
    return 0.2126 * channels[0] + 0.7152 * channels[1] + 0.0722 * channels[2]
}

function getReadableTextColor(background: string) {
    return getRelativeLuminance(background) > 0.36 ? '#0f172a' : '#f8fafc'
}

function getKanbanTone(color: string | undefined, isDark: boolean, variant: 'column' | 'card') {
    const accent = normalizeHex(color || DEFAULT_COLUMN_COLOR)
    const neutralBg = isDark ? (variant === 'column' ? '#111827' : '#0f172a') : variant === 'column' ? '#f8fafc' : '#ffffff'
    const neutralBorder = isDark ? '#334155' : '#dbe2ea'
    const background = color
        ? isDark
            ? mixHex(accent, variant === 'column' ? '#0f172a' : '#111827', variant === 'column' ? 0.8 : 0.72)
            : mixHex(accent, '#ffffff', variant === 'column' ? 0.82 : 0.9)
        : neutralBg
    const border = color
        ? isDark
            ? mixHex(accent, '#475569', variant === 'column' ? 0.48 : 0.56)
            : mixHex(accent, '#cbd5e1', variant === 'column' ? 0.42 : 0.62)
        : neutralBorder
    const title = getReadableTextColor(background)
    const body = title === '#f8fafc' ? 'rgba(226,232,240,0.84)' : '#475569'
    const muted = title === '#f8fafc' ? 'rgba(203,213,225,0.74)' : '#64748b'
    const badgeBackground = title === '#f8fafc' ? 'rgba(255,255,255,0.08)' : 'rgba(15,23,42,0.06)'

    return {
        accent,
        background,
        border,
        title,
        body,
        muted,
        badgeBackground,
    }
}

function getLinkedNoteHref(item: KanbanItem, currentDocumentId: string) {
    const linkedDocumentId = sanitizeDocumentId(item.linkedDocumentId?.trim() || '') || sanitizeDocumentId(currentDocumentId)
    const linkedSubdocumentName = item.linkedSubdocumentName?.trim() || ''

    if (!linkedDocumentId) {
        return null
    }

    if (linkedSubdocumentName) {
        return `/${encodeURIComponent(linkedDocumentId)}/${encodeURIComponent(linkedSubdocumentName)}`
    }

    if (item.linkedDocumentId?.trim()) {
        return `/${encodeURIComponent(linkedDocumentId)}`
    }

    return null
}

function getLinkedNoteLabel(item: KanbanItem, currentDocumentId: string) {
    const linkedDocumentId = item.linkedDocumentId?.trim() || currentDocumentId
    const linkedSubdocumentName = item.linkedSubdocumentName?.trim()

    if (linkedSubdocumentName) {
        return `${linkedDocumentId} / ${linkedSubdocumentName}`
    }

    if (item.linkedDocumentId?.trim()) {
        return linkedDocumentId
    }

    return null
}

function SwatchButton({
    option,
    selected,
    onSelect,
}: {
    option: ColorOption
    selected: boolean
    onSelect: () => void
}) {
    return (
        <button
            type="button"
            title={option.label}
            onClick={onSelect}
            className={`h-8 w-8 rounded-full border-2 transition ${selected ? 'scale-110 border-slate-900 dark:border-slate-100' : 'border-white/70 dark:border-slate-900/80'}`}
            style={{ backgroundColor: option.value }}
        />
    )
}

interface EditColModalProps {
    column: KanbanColumn
    onSave: (updates: Partial<KanbanColumn>) => void
    onClose: () => void
}

function EditColModal({ column, onSave, onClose }: EditColModalProps) {
    const [title, setTitle] = useState(column.title)
    const [description, setDescription] = useState(column.description)
    const [color, setColor] = useState(column.color || DEFAULT_COLUMN_COLOR)

    return (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 px-4" onClick={onClose}>
            <div className="w-full max-w-sm space-y-4 rounded-2xl bg-white p-6 shadow-xl dark:bg-slate-800" onClick={event => event.stopPropagation()}>
                <h3 className="text-sm font-semibold text-slate-900 dark:text-slate-100">Editar coluna</h3>
                <div>
                    <label className="mb-1 block text-xs font-medium text-slate-600 dark:text-slate-400">Titulo</label>
                    <input
                        autoFocus
                        value={title}
                        onChange={event => setTitle(event.target.value)}
                        className="w-full rounded-md border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 focus:outline-none focus:ring-1 focus:ring-slate-400 dark:border-slate-600 dark:bg-slate-700 dark:text-slate-100"
                    />
                </div>
                <div>
                    <label className="mb-1 block text-xs font-medium text-slate-600 dark:text-slate-400">Descricao</label>
                    <textarea
                        value={description}
                        onChange={event => setDescription(event.target.value)}
                        rows={3}
                        className="w-full resize-none rounded-md border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 focus:outline-none focus:ring-1 focus:ring-slate-400 dark:border-slate-600 dark:bg-slate-700 dark:text-slate-100"
                        placeholder="Contexto da coluna..."
                    />
                </div>
                <div>
                    <label className="mb-2 block text-xs font-medium text-slate-600 dark:text-slate-400">Cor da coluna</label>
                    <div className="flex flex-wrap gap-2">
                        {KANBAN_COLOR_OPTIONS.map(option => (
                            <SwatchButton
                                key={option.value}
                                option={option}
                                selected={color === option.value}
                                onSelect={() => setColor(option.value)}
                            />
                        ))}
                    </div>
                </div>
                <div className="flex gap-2 pt-2">
                    <button
                        type="button"
                        onClick={() => onSave({ title: title.trim(), description: description.trim(), color })}
                        disabled={!title.trim()}
                        className="flex-1 rounded-md bg-slate-900 px-3 py-2 text-sm font-medium text-white transition hover:bg-slate-800 disabled:opacity-40 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-slate-200"
                    >
                        Salvar
                    </button>
                    <button
                        type="button"
                        onClick={onClose}
                        className="rounded-md border border-slate-200 px-3 py-2 text-sm text-slate-700 transition hover:bg-slate-50 dark:border-slate-600 dark:text-slate-300 dark:hover:bg-slate-700"
                    >
                        Cancelar
                    </button>
                </div>
            </div>
        </div>
    )
}

interface EditItemModalProps {
    currentDocumentId: string
    item: KanbanItem
    onSave: (updates: Partial<KanbanItem>) => void
    onClose: () => void
}

function EditItemModal({ currentDocumentId, item, onSave, onClose }: EditItemModalProps) {
    const [title, setTitle] = useState(item.title)
    const [description, setDescription] = useState(item.description)
    const [color, setColor] = useState(item.color || '')
    const [linkedDocumentId, setLinkedDocumentId] = useState(item.linkedDocumentId || '')
    const [linkedSubdocumentName, setLinkedSubdocumentName] = useState(item.linkedSubdocumentName || '')

    return (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 px-4" onClick={onClose}>
            <div className="w-full max-w-md space-y-4 rounded-2xl bg-white p-6 shadow-xl dark:bg-slate-800" onClick={event => event.stopPropagation()}>
                <h3 className="text-sm font-semibold text-slate-900 dark:text-slate-100">Editar item</h3>
                <div>
                    <label className="mb-1 block text-xs font-medium text-slate-600 dark:text-slate-400">Titulo</label>
                    <input
                        autoFocus
                        value={title}
                        onChange={event => setTitle(event.target.value)}
                        className="w-full rounded-md border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 focus:outline-none focus:ring-1 focus:ring-slate-400 dark:border-slate-600 dark:bg-slate-700 dark:text-slate-100"
                    />
                </div>
                <div>
                    <label className="mb-1 block text-xs font-medium text-slate-600 dark:text-slate-400">Descricao</label>
                    <textarea
                        value={description}
                        onChange={event => setDescription(event.target.value)}
                        rows={3}
                        className="w-full resize-none rounded-md border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 focus:outline-none focus:ring-1 focus:ring-slate-400 dark:border-slate-600 dark:bg-slate-700 dark:text-slate-100"
                        placeholder="Detalhes do item..."
                    />
                </div>
                <div>
                    <label className="mb-2 block text-xs font-medium text-slate-600 dark:text-slate-400">Cor do cartao</label>
                    <div className="flex flex-wrap gap-2">
                        <button
                            type="button"
                            onClick={() => setColor('')}
                            className={`inline-flex h-8 items-center rounded-full border px-3 text-xs font-medium transition ${!color ? 'border-slate-900 bg-slate-900 text-white dark:border-slate-100 dark:bg-slate-100 dark:text-slate-900' : 'border-slate-200 bg-white text-slate-600 hover:border-slate-300 dark:border-slate-600 dark:bg-slate-700 dark:text-slate-300'}`}
                        >
                            Padrao
                        </button>
                        {KANBAN_COLOR_OPTIONS.map(option => (
                            <SwatchButton
                                key={option.value}
                                option={option}
                                selected={color === option.value}
                                onSelect={() => setColor(option.value)}
                            />
                        ))}
                    </div>
                </div>
                <div className="space-y-3 rounded-xl border border-slate-200/80 bg-slate-50/90 p-3 dark:border-slate-700/80 dark:bg-slate-900/60">
                    <div>
                        <label className="mb-1 block text-xs font-medium text-slate-600 dark:text-slate-400">Documento vinculado</label>
                        <input
                            value={linkedDocumentId}
                            onChange={event => setLinkedDocumentId(event.target.value)}
                            className="w-full rounded-md border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 focus:outline-none focus:ring-1 focus:ring-slate-400 dark:border-slate-600 dark:bg-slate-700 dark:text-slate-100"
                            placeholder={currentDocumentId}
                        />
                        <p className="mt-1 text-[11px] text-slate-500 dark:text-slate-400">Se ficar vazio, o item abre uma nota deste mesmo documento.</p>
                    </div>
                    <div>
                        <label className="mb-1 block text-xs font-medium text-slate-600 dark:text-slate-400">Subdocumento vinculado</label>
                        <input
                            value={linkedSubdocumentName}
                            onChange={event => setLinkedSubdocumentName(event.target.value)}
                            className="w-full rounded-md border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 focus:outline-none focus:ring-1 focus:ring-slate-400 dark:border-slate-600 dark:bg-slate-700 dark:text-slate-100"
                            placeholder="Ex.: backlog da semana"
                        />
                    </div>
                </div>
                <div className="flex gap-2 pt-2">
                    <button
                        type="button"
                        onClick={() => onSave({
                            title: title.trim(),
                            description: description.trim(),
                            color,
                            linkedDocumentId: sanitizeDocumentId(linkedDocumentId.trim()),
                            linkedSubdocumentName: linkedSubdocumentName.trim(),
                        })}
                        disabled={!title.trim()}
                        className="flex-1 rounded-md bg-slate-900 px-3 py-2 text-sm font-medium text-white transition hover:bg-slate-800 disabled:opacity-40 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-slate-200"
                    >
                        Salvar
                    </button>
                    <button
                        type="button"
                        onClick={onClose}
                        className="rounded-md border border-slate-200 px-3 py-2 text-sm text-slate-700 transition hover:bg-slate-50 dark:border-slate-600 dark:text-slate-300 dark:hover:bg-slate-700"
                    >
                        Cancelar
                    </button>
                </div>
            </div>
        </div>
    )
}

export function KanbanEditor({ documentId, subdocumentName, isReadOnly: isReadOnlyProp }: KanbanEditorProps) {
    const { isReadOnly: ctxReadOnly } = useDocumentSecurity()
    const isReadOnly = isReadOnlyProp ?? ctxReadOnly
    const colsMap = useMap(getKanbanColumnsStateKey(subdocumentName))
    const itemsMap = useMap(getKanbanItemsStateKey(subdocumentName))
    const touchLastModified = useDebouncedLastModified(subdocumentName)

    const [isDark, setIsDark] = useState(false)
    const [editingCol, setEditingCol] = useState<KanbanColumn | null>(null)
    const [editingItem, setEditingItem] = useState<KanbanItem | null>(null)
    const [addingItemColId, setAddingItemColId] = useState<string | null>(null)
    const [newItemTitle, setNewItemTitle] = useState('')
    const [draggedItemId, setDraggedItemId] = useState<string | null>(null)
    const [dragOverColId, setDragOverColId] = useState<string | null>(null)
    const [dragOverItemId, setDragOverItemId] = useState<string | null>(null)

    useEffect(() => {
        const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
        setIsDark(mediaQuery.matches)
        const handleChange = (event: MediaQueryListEvent) => setIsDark(event.matches)
        mediaQuery.addEventListener('change', handleChange)
        return () => mediaQuery.removeEventListener('change', handleChange)
    }, [])

    const columns: KanbanColumn[] = colsMap
        ? Array.from(colsMap.values() as Iterable<KanbanColumn>).sort((a, b) => a.order - b.order)
        : []
    const allItems: KanbanItem[] = itemsMap
        ? Array.from(itemsMap.values() as Iterable<KanbanItem>).sort((a, b) => a.order - b.order)
        : []

    useEffect(() => {
        const observeLocalChanges = (map: typeof colsMap | typeof itemsMap) => {
            if (!map) return () => undefined
            const handler = (_event: any, transaction: { local?: boolean }) => {
                if (!transaction?.local) return
                touchLastModified()
            }
            map.observe(handler)
            return () => map.unobserve(handler)
        }

        const cleanupColumns = observeLocalChanges(colsMap)
        const cleanupItems = observeLocalChanges(itemsMap)

        return () => {
            cleanupColumns()
            cleanupItems()
        }
    }, [colsMap, itemsMap, touchLastModified])

    const handleAddColumn = () => {
        if (!colsMap) return
        const id = `col-${generateId()}`
        colsMap.set(id, {
            id,
            title: 'Nova coluna',
            description: '',
            color: DEFAULT_COLUMN_COLOR,
            order: (columns.length + 1) * 1000,
        })
    }

    const handleSaveCol = (updates: Partial<KanbanColumn>) => {
        if (!colsMap || !editingCol) return
        colsMap.set(editingCol.id, { ...editingCol, ...updates })
        setEditingCol(null)
    }

    const handleDeleteCol = (columnId: string) => {
        if (!colsMap || !itemsMap) return
        colsMap.delete(columnId)
        for (const item of allItems) {
            if (item.columnId === columnId) {
                itemsMap.delete(item.id)
            }
        }
    }

    const handleAddItem = (columnId: string) => {
        if (!itemsMap || !newItemTitle.trim()) return
        const id = `item-${generateId()}`
        const columnItems = allItems.filter(item => item.columnId === columnId)
        itemsMap.set(id, {
            id,
            title: newItemTitle.trim(),
            description: '',
            columnId,
            order: (columnItems.length + 1) * 1000,
            color: '',
            linkedDocumentId: '',
            linkedSubdocumentName: '',
        })
        setNewItemTitle('')
        setAddingItemColId(null)
    }

    const handleSaveItem = (updates: Partial<KanbanItem>) => {
        if (!itemsMap || !editingItem) return
        itemsMap.set(editingItem.id, { ...editingItem, ...updates })
        setEditingItem(null)
    }

    const handleDeleteItem = (itemId: string) => {
        if (!itemsMap) return
        itemsMap.delete(itemId)
    }

    const handleDragStart = (event: React.DragEvent, itemId: string) => {
        setDraggedItemId(itemId)
        event.dataTransfer.effectAllowed = 'move'
        setTimeout(() => {
            const element = document.getElementById(`kanban-item-${itemId}`)
            if (element) {
                element.classList.add('opacity-45')
            }
        }, 0)
    }

    const handleDragEnd = (itemId: string) => {
        const element = document.getElementById(`kanban-item-${itemId}`)
        if (element) {
            element.classList.remove('opacity-45')
        }
        setDraggedItemId(null)
        setDragOverColId(null)
        setDragOverItemId(null)
    }

    const handleDropOnColumn = (event: React.DragEvent, columnId: string) => {
        event.preventDefault()
        if (!draggedItemId || !itemsMap) return

        const draggedItem = itemsMap.get(draggedItemId) as KanbanItem | undefined
        if (!draggedItem) return

        if (dragOverItemId) {
            const targetItem = itemsMap.get(dragOverItemId) as KanbanItem | undefined
            if (targetItem) {
                const columnItems = allItems.filter(item => item.columnId === columnId).sort((a, b) => a.order - b.order)
                const targetIndex = columnItems.findIndex(item => item.id === dragOverItemId)
                const previousOrder = targetIndex > 0 ? columnItems[targetIndex - 1].order : 0
                const nextOrder = (previousOrder + targetItem.order) / 2
                itemsMap.set(draggedItemId, { ...draggedItem, columnId, order: nextOrder })
            }
        } else {
            const columnItems = allItems.filter(item => item.columnId === columnId)
            const maxOrder = columnItems.reduce((highest, item) => Math.max(highest, item.order), 0)
            itemsMap.set(draggedItemId, { ...draggedItem, columnId, order: maxOrder + 1000 })
        }

        setDraggedItemId(null)
        setDragOverColId(null)
        setDragOverItemId(null)
    }

    const toolbar = (
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <span className="text-xs font-medium uppercase tracking-[0.14em] text-slate-500 dark:text-slate-400">
                {columns.length} coluna{columns.length !== 1 ? 's' : ''}
                {' · '}
                {allItems.length} item{allItems.length !== 1 ? 's' : ''}
            </span>
            <button
                type="button"
                onClick={handleAddColumn}
                disabled={isReadOnly}
                className="inline-flex items-center justify-center gap-1.5 rounded-xl bg-slate-950 px-3.5 py-2 text-xs font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-40 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-slate-200"
            >
                <svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                    <line x1="12" y1="5" x2="12" y2="19" />
                    <line x1="5" y1="12" x2="19" y2="12" />
                </svg>
                Nova coluna
            </button>
        </div>
    )

    return (
        <EditorShell
            variant="full"
            toolbar={toolbar}
            surfaceVariant="muted"
            frameClassName="overflow-hidden bg-slate-100 dark:bg-slate-950"
            contentClassName="overflow-hidden"
        >
            {columns.length === 0 ? (
                <div className="flex h-full items-center justify-center px-6">
                    <div className="space-y-3 text-center">
                        <svg xmlns="http://www.w3.org/2000/svg" width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" className="mx-auto text-slate-300 dark:text-slate-600">
                            <rect x="3" y="3" width="7" height="7" />
                            <rect x="14" y="3" width="7" height="7" />
                            <rect x="14" y="14" width="7" height="7" />
                            <rect x="3" y="14" width="7" height="7" />
                        </svg>
                        <p className="text-sm text-slate-400 dark:text-slate-500">Nenhuma coluna ainda</p>
                        <button
                            type="button"
                            onClick={handleAddColumn}
                            className="inline-flex items-center justify-center rounded-xl bg-slate-950 px-4 py-2 text-sm font-medium text-white transition hover:bg-slate-800 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-slate-200"
                        >
                            + Criar primeira coluna
                        </button>
                    </div>
                </div>
            ) : (
                <div className="h-full overflow-x-auto overflow-y-hidden">
                    <div className="flex h-full items-start gap-4 p-4 sm:p-5">
                        {columns.map(column => {
                            const columnItems = allItems.filter(item => item.columnId === column.id)
                            const columnTone = getKanbanTone(column.color, isDark, 'column')
                            const isOver = dragOverColId === column.id

                            return (
                                <div
                                    key={column.id}
                                    className={`flex h-full w-72 flex-shrink-0 flex-col rounded-2xl border transition-shadow ${isOver ? 'shadow-xl ring-2 ring-slate-400/70 dark:ring-slate-500/70' : 'shadow-sm'}`}
                                    style={{ backgroundColor: columnTone.background, borderColor: columnTone.border }}
                                    onDragOver={event => {
                                        if (isReadOnly) return
                                        event.preventDefault()
                                        setDragOverColId(column.id)
                                    }}
                                    onDragLeave={event => {
                                        if (!(event.currentTarget as HTMLElement).contains(event.relatedTarget as Node)) {
                                            setDragOverColId(null)
                                        }
                                    }}
                                    onDrop={event => handleDropOnColumn(event, column.id)}
                                >
                                    <div className="flex items-start justify-between gap-3 p-3 pb-2">
                                        <div className="min-w-0 flex-1">
                                            <div className="flex items-center gap-2">
                                                <span className="h-2.5 w-2.5 rounded-full" style={{ backgroundColor: columnTone.accent }} />
                                                <h3 className="truncate text-sm font-semibold" style={{ color: columnTone.title }}>{column.title}</h3>
                                            </div>
                                            {column.description ? (
                                                <p className="mt-1 text-xs whitespace-pre-wrap break-words" style={{ color: columnTone.body }}>{column.description}</p>
                                            ) : null}
                                        </div>
                                        <div className="ml-2 flex items-center gap-1">
                                            <span
                                                className="inline-flex min-w-7 items-center justify-center rounded-full px-2 py-1 text-[11px] font-medium"
                                                style={{ backgroundColor: columnTone.badgeBackground, color: columnTone.muted }}
                                            >
                                                {columnItems.length}
                                            </span>
                                            <button
                                                type="button"
                                                onClick={() => setEditingCol(column)}
                                                disabled={isReadOnly}
                                                className="rounded-lg p-1.5 transition hover:bg-black/10 disabled:opacity-0 dark:hover:bg-white/10"
                                                style={{ color: columnTone.muted }}
                                                title="Editar coluna"
                                            >
                                                <svg xmlns="http://www.w3.org/2000/svg" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                                                    <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7" />
                                                    <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z" />
                                                </svg>
                                            </button>
                                            <button
                                                type="button"
                                                onClick={() => handleDeleteCol(column.id)}
                                                disabled={isReadOnly}
                                                className="rounded-lg p-1.5 text-slate-400 transition hover:bg-red-100/80 hover:text-red-600 disabled:opacity-0 dark:hover:bg-red-900/30 dark:hover:text-red-400"
                                                title="Excluir coluna"
                                            >
                                                <svg xmlns="http://www.w3.org/2000/svg" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                                                    <path d="M3 6h18" />
                                                    <path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6" />
                                                    <path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2" />
                                                </svg>
                                            </button>
                                        </div>
                                    </div>

                                    <div className="min-h-[2rem] flex-1 space-y-2 overflow-y-auto px-3 pb-2">
                                        {columnItems.map(item => {
                                            const itemTone = getKanbanTone(item.color, isDark, 'card')
                                            const linkedNoteHref = getLinkedNoteHref(item, documentId)
                                            const linkedNoteLabel = getLinkedNoteLabel(item, documentId)

                                            return (
                                                <div
                                                    key={item.id}
                                                    id={`kanban-item-${item.id}`}
                                                    draggable={!isReadOnly}
                                                    onDragStart={event => !isReadOnly && handleDragStart(event, item.id)}
                                                    onDragEnd={() => handleDragEnd(item.id)}
                                                    onDragOver={event => {
                                                        event.preventDefault()
                                                        event.stopPropagation()
                                                        setDragOverItemId(item.id)
                                                    }}
                                                    className={`group rounded-xl border p-3 shadow-sm transition hover:shadow-md ${dragOverItemId === item.id && draggedItemId !== item.id ? 'border-t-2 border-t-blue-400' : ''}`}
                                                    style={{ backgroundColor: itemTone.background, borderColor: itemTone.border }}
                                                >
                                                    <div className="flex items-start justify-between gap-2">
                                                        <button
                                                            type="button"
                                                            onClick={() => linkedNoteHref && window.location.assign(linkedNoteHref)}
                                                            disabled={!linkedNoteHref}
                                                            className={`min-w-0 flex-1 text-left disabled:cursor-default disabled:opacity-100 ${linkedNoteHref ? 'cursor-pointer' : ''}`}
                                                        >
                                                            <div className="flex items-center gap-2">
                                                                {item.color ? (
                                                                    <span className="h-2.5 w-2.5 rounded-full" style={{ backgroundColor: itemTone.accent }} />
                                                                ) : null}
                                                                <p className="text-sm font-medium leading-snug" style={{ color: itemTone.title }}>{item.title}</p>
                                                            </div>
                                                            {item.description ? (
                                                                <p className="mt-1 text-xs leading-snug whitespace-pre-wrap break-words" style={{ color: itemTone.body }}>{item.description}</p>
                                                            ) : null}
                                                            {linkedNoteLabel ? (
                                                                <span
                                                                    className="mt-3 inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-[11px] font-medium"
                                                                    style={{ backgroundColor: itemTone.badgeBackground, color: itemTone.muted }}
                                                                >
                                                                    <svg xmlns="http://www.w3.org/2000/svg" width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                                                                        <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07L11 5" />
                                                                        <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07L13 19" />
                                                                    </svg>
                                                                    {linkedNoteLabel}
                                                                </span>
                                                            ) : null}
                                                        </button>
                                                        <div className="flex flex-shrink-0 items-center gap-0.5 opacity-100 transition md:opacity-0 md:group-hover:opacity-100">
                                                            <button
                                                                type="button"
                                                                onClick={() => setEditingItem(item)}
                                                                disabled={isReadOnly}
                                                                className="rounded-lg p-1.5 text-slate-400 transition hover:bg-slate-100 hover:text-slate-600 disabled:opacity-0 dark:hover:bg-slate-700 dark:hover:text-slate-300"
                                                                title="Editar item"
                                                            >
                                                                <svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                                                                    <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7" />
                                                                    <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z" />
                                                                </svg>
                                                            </button>
                                                            <button
                                                                type="button"
                                                                onClick={() => handleDeleteItem(item.id)}
                                                                disabled={isReadOnly}
                                                                className="rounded-lg p-1.5 text-slate-400 transition hover:bg-red-50 hover:text-red-500 disabled:opacity-0 dark:hover:bg-red-900/20"
                                                                title="Excluir item"
                                                            >
                                                                <svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                                                                    <path d="M3 6h18" />
                                                                    <path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6" />
                                                                    <path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2" />
                                                                </svg>
                                                            </button>
                                                        </div>
                                                    </div>
                                                </div>
                                            )
                                        })}
                                    </div>

                                    <div className="px-3 pb-3">
                                        {!isReadOnly && addingItemColId === column.id ? (
                                            <div className="space-y-2">
                                                <input
                                                    autoFocus
                                                    value={newItemTitle}
                                                    onChange={event => setNewItemTitle(event.target.value)}
                                                    onKeyDown={event => {
                                                        if (event.key === 'Enter') handleAddItem(column.id)
                                                        if (event.key === 'Escape') {
                                                            setAddingItemColId(null)
                                                            setNewItemTitle('')
                                                        }
                                                    }}
                                                    placeholder="Titulo do item..."
                                                    className="w-full rounded-md border border-slate-200 bg-white px-2.5 py-2 text-sm text-slate-900 placeholder-slate-400 focus:outline-none focus:ring-1 focus:ring-slate-400 dark:border-slate-600 dark:bg-slate-700 dark:text-slate-100 dark:placeholder-slate-500"
                                                />
                                                <div className="flex gap-1.5">
                                                    <button
                                                        type="button"
                                                        onClick={() => handleAddItem(column.id)}
                                                        disabled={!newItemTitle.trim()}
                                                        className="flex-1 rounded-md bg-slate-900 py-1.5 text-xs font-medium text-white transition hover:bg-slate-800 disabled:opacity-40 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-slate-200"
                                                    >
                                                        Adicionar
                                                    </button>
                                                    <button
                                                        type="button"
                                                        onClick={() => {
                                                            setAddingItemColId(null)
                                                            setNewItemTitle('')
                                                        }}
                                                        className="rounded-md border border-slate-200 px-3 py-1.5 text-xs text-slate-600 transition hover:bg-black/5 dark:border-slate-600 dark:text-slate-400 dark:hover:bg-white/5"
                                                    >
                                                        Cancelar
                                                    </button>
                                                </div>
                                            </div>
                                        ) : !isReadOnly ? (
                                            <button
                                                type="button"
                                                onClick={() => {
                                                    setAddingItemColId(column.id)
                                                    setNewItemTitle('')
                                                }}
                                                className="flex w-full items-center gap-1.5 rounded-md px-2 py-1.5 text-xs text-slate-500 transition hover:bg-black/5 dark:text-slate-400 dark:hover:bg-white/5"
                                            >
                                                <svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                                                    <line x1="12" y1="5" x2="12" y2="19" />
                                                    <line x1="5" y1="12" x2="19" y2="12" />
                                                </svg>
                                                Adicionar item
                                            </button>
                                        ) : null}
                                    </div>
                                </div>
                            )
                        })}

                        {!isReadOnly ? (
                            <button
                                type="button"
                                onClick={handleAddColumn}
                                className="flex h-16 w-72 flex-shrink-0 items-center justify-center gap-2 rounded-xl border-2 border-dashed border-slate-300 text-sm text-slate-400 transition hover:border-slate-400 hover:text-slate-600 dark:border-slate-700 dark:text-slate-500 dark:hover:border-slate-600 dark:hover:text-slate-400"
                            >
                                <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                                    <line x1="12" y1="5" x2="12" y2="19" />
                                    <line x1="5" y1="12" x2="19" y2="12" />
                                </svg>
                                Nova coluna
                            </button>
                        ) : null}
                    </div>
                </div>
            )}

            {editingCol ? <EditColModal column={editingCol} onSave={handleSaveCol} onClose={() => setEditingCol(null)} /> : null}
            {editingItem ? (
                <EditItemModal
                    currentDocumentId={documentId}
                    item={editingItem}
                    onSave={handleSaveItem}
                    onClose={() => setEditingItem(null)}
                />
            ) : null}
        </EditorShell>
    )
}
