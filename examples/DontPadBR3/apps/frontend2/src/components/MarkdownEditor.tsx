'use client'

import '@uiw/react-md-editor/markdown-editor.css'
import { useText } from '@/lib/collab/react'
import { useEffect, useMemo, useRef, useState } from 'react'
import dynamic from 'next/dynamic'
import * as Y from 'yjs'
import { EditorShell } from '@/components/EditorShell'
import { getMarkdownStateKey } from '@/lib/noteStateAdapters'
import { useDocumentSecurity } from '@/lib/documentSecurityContext'
import { useDebouncedLastModified } from '@/lib/useDocumentMeta'
import { getSafeUrl } from '@/lib/urlSafety'

const MDEditor = dynamic(() => import('@uiw/react-md-editor/nohighlight'), { ssr: false })
const MDPreview = dynamic(() => import('@uiw/react-markdown-preview/nohighlight'), { ssr: false })
type MarkdownViewMode = 'edit' | 'live' | 'preview'

interface MarkdownEditorProps {
    subdocumentName: string
    isReadOnly?: boolean
}

// Minimal diff-based Y.Text sync (avoids full replace on each keystroke)
function applyTextDiff(yText: Y.Text, oldVal: string, newVal: string) {
    if (oldVal === newVal) return
    let start = 0
    while (start < oldVal.length && start < newVal.length && oldVal[start] === newVal[start]) start++
    let endOld = oldVal.length
    let endNew = newVal.length
    while (endOld > start && endNew > start && oldVal[endOld - 1] === newVal[endNew - 1]) {
        endOld--
        endNew--
    }
    const deleteLen = endOld - start
    const insertStr = newVal.slice(start, endNew)
    yText.doc?.transact(() => {
        if (deleteLen > 0) yText.delete(start, deleteLen)
        if (insertStr) yText.insert(start, insertStr)
    })
}

export function MarkdownEditor({ subdocumentName, isReadOnly: isReadOnlyProp }: MarkdownEditorProps) {
    const { isReadOnly: ctxReadOnly } = useDocumentSecurity()
    const isReadOnly = isReadOnlyProp ?? ctxReadOnly

    const textKey = getMarkdownStateKey(subdocumentName)
    const yText = useText(textKey, { observe: 'none' })
    const touchLastModified = useDebouncedLastModified(subdocumentName)

    const [value, setValue] = useState('')
    const [viewMode, setViewMode] = useState<MarkdownViewMode>('live')
    const prevValueRef = useRef('')
    const isLocalChangeRef = useRef(false)
    const stats = useMemo(() => {
        const words = value.trim().length > 0 ? value.trim().split(/\s+/).length : 0
        const wordLabel = words === 1 ? 'palavra' : 'palavras'
        const charLabel = value.length === 1 ? 'caractere' : 'caracteres'
        return {
            chars: value.length,
            words,
            label: `${words} ${wordLabel} · ${value.length} ${charLabel}`,
        }
    }, [value])

    // Sync Y.Text → state
    useEffect(() => {
        if (!yText) return
        const initial = yText.toString()
        setValue(initial)
        prevValueRef.current = initial

        const handler = () => {
            if (isLocalChangeRef.current) return
            const current = yText.toString()
            prevValueRef.current = current
            setValue(current)
        }
        yText.observe(handler)
        return () => yText.unobserve(handler)
    }, [yText])

    useEffect(() => {
        if (!yText) return
        const handler = (_event: any, transaction: { local?: boolean }) => {
            if (!transaction?.local) return
            touchLastModified()
        }
        yText.observe(handler)
        return () => yText.unobserve(handler)
    }, [touchLastModified, yText])

    const handleChange = (newValue?: string) => {
        const text = newValue ?? ''
        setValue(text)
        if (!yText) return
        isLocalChangeRef.current = true
        applyTextDiff(yText, prevValueRef.current, text)
        prevValueRef.current = text
        isLocalChangeRef.current = false
    }

    const safeUrlTransform = (url: string) => getSafeUrl(url) ?? ''
    const toolbar = (
        <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
            <div className="flex items-center gap-2">
                <span className="h-2 w-2 rounded-full bg-sky-500 shadow-[0_0_0_4px_rgba(14,165,233,0.12)] dark:bg-sky-400" />
                <span className="text-sm font-semibold text-slate-800 dark:text-slate-100">
                    Markdown
                </span>
                <span className="hidden text-xs text-slate-400 dark:text-slate-500 sm:inline">
                    {stats.label}
                </span>
            </div>
            {!isReadOnly ? (
                <div className="inline-flex w-fit rounded-lg border border-slate-200 bg-slate-50 p-1 dark:border-slate-700 dark:bg-slate-950">
                    {[
                        ['edit', 'Editar'],
                        ['live', 'Dividir'],
                        ['preview', 'Preview'],
                    ].map(([mode, label]) => (
                        <button
                            key={mode}
                            type="button"
                            onClick={() => setViewMode(mode as MarkdownViewMode)}
                            className={`rounded-md px-3 py-1.5 text-xs font-medium transition ${viewMode === mode
                                ? 'bg-white text-slate-950 shadow-sm dark:bg-slate-800 dark:text-slate-50'
                                : 'text-slate-500 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-100'
                                }`}
                        >
                            {label}
                        </button>
                    ))}
                </div>
            ) : null}
        </div>
    )

    return (
        <EditorShell
            variant="narrow"
            surfaceVariant="glass"
            toolbar={toolbar}
            frameClassName="overflow-hidden rounded-2xl bg-white/95 dark:bg-slate-950/95"
            contentClassName="overflow-hidden bg-white dark:bg-slate-950"
            className="min-h-0"
        >
            <style>{`
                .dontpad-markdown-editor {
                    --md-bg: #ffffff;
                    --md-panel: #f8fafc;
                    --md-panel-strong: #f1f5f9;
                    --md-border: rgba(148, 163, 184, 0.28);
                    --md-border-strong: rgba(100, 116, 139, 0.34);
                    --md-text: #0f172a;
                    --md-soft: #64748b;
                    --md-code-bg: #f1f5f9;
                    --md-code-text: #0f172a;
                    --md-accent: #2563eb;
                }
                .dark .dontpad-markdown-editor {
                    --md-bg: #020617;
                    --md-panel: #0f172a;
                    --md-panel-strong: #111827;
                    --md-border: rgba(71, 85, 105, 0.58);
                    --md-border-strong: rgba(100, 116, 139, 0.72);
                    --md-text: #e2e8f0;
                    --md-soft: #94a3b8;
                    --md-code-bg: rgba(15, 23, 42, 0.9);
                    --md-code-text: #dbeafe;
                    --md-accent: #60a5fa;
                }
                @media (prefers-color-scheme: dark) {
                    .dontpad-markdown-editor {
                        --md-bg: #020617;
                        --md-panel: #0f172a;
                        --md-panel-strong: #111827;
                        --md-border: rgba(71, 85, 105, 0.58);
                        --md-border-strong: rgba(100, 116, 139, 0.72);
                        --md-text: #e2e8f0;
                        --md-soft: #94a3b8;
                        --md-code-bg: rgba(15, 23, 42, 0.9);
                        --md-code-text: #dbeafe;
                        --md-accent: #60a5fa;
                    }
                }
                .dontpad-markdown-editor .w-md-editor {
                    height: 100%;
                    min-height: 0;
                    display: flex;
                    flex-direction: column;
                    border: 0;
                    border-radius: 0;
                    background: var(--md-bg);
                    color: var(--md-text);
                    box-shadow: none;
                }
                .dontpad-markdown-editor .w-md-editor-toolbar {
                    min-height: 46px;
                    border-bottom: 1px solid var(--md-border);
                    background: linear-gradient(180deg, var(--md-panel), var(--md-bg));
                    padding: 7px 12px;
                }
                .dontpad-markdown-editor .w-md-editor-toolbar ul {
                    display: flex;
                    gap: 3px;
                }
                .dontpad-markdown-editor .w-md-editor-toolbar li > button {
                    height: 30px;
                    min-width: 30px;
                    border-radius: 7px;
                    color: var(--md-soft);
                    transition: background-color 140ms ease, color 140ms ease;
                }
                .dontpad-markdown-editor .w-md-editor-toolbar li > button:hover,
                .dontpad-markdown-editor .w-md-editor-toolbar li.active > button {
                    background: var(--md-panel-strong);
                    color: var(--md-text);
                }
                .dontpad-markdown-editor .w-md-editor-content {
                    flex: 1 1 auto;
                    min-height: 0;
                    height: auto !important;
                    background: var(--md-bg);
                }
                .dontpad-markdown-editor .w-md-editor-area {
                    min-height: 0;
                    height: 100%;
                    border-right: 1px solid var(--md-border);
                    background:
                        linear-gradient(90deg, rgba(148, 163, 184, 0.05) 0, transparent 56px),
                        var(--md-bg);
                }
                .dontpad-markdown-editor .w-md-editor-input,
                .dontpad-markdown-editor .w-md-editor-text,
                .dontpad-markdown-editor .w-md-editor-text-input,
                .dontpad-markdown-editor .w-md-editor-text-pre {
                    min-height: 100%;
                }
                .dontpad-markdown-editor .w-md-editor-input,
                .dontpad-markdown-editor .w-md-editor-text,
                .dontpad-markdown-editor .w-md-editor-text-input,
                .dontpad-markdown-editor .w-md-editor-text-pre > code {
                    color: var(--md-text) !important;
                    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace !important;
                    font-size: 14px !important;
                    line-height: 1.78 !important;
                    letter-spacing: 0 !important;
                }
                .dontpad-markdown-editor .w-md-editor-text-input,
                .dontpad-markdown-editor .w-md-editor-text-pre {
                    padding: 24px 28px !important;
                }
                .dontpad-markdown-editor .w-md-editor-text-input::placeholder {
                    color: var(--md-soft);
                }
                .dontpad-markdown-editor .w-md-editor-preview {
                    min-height: 0;
                    height: 100%;
                    background: var(--md-bg);
                    box-shadow: none;
                    padding: 0 !important;
                }
                .dontpad-markdown-editor .wmde-markdown {
                    width: min(100%, 880px);
                    max-width: 880px;
                    margin: 0;
                    padding: 34px 30px 56px;
                    background: transparent;
                    color: var(--md-text);
                    font-size: 16px;
                    line-height: 1.78;
                    letter-spacing: 0;
                    text-align: left;
                }
                .dontpad-markdown-editor .wmde-markdown h1,
                .dontpad-markdown-editor .wmde-markdown h2,
                .dontpad-markdown-editor .wmde-markdown h3 {
                    color: var(--md-text);
                    font-weight: 720;
                    letter-spacing: 0;
                    line-height: 1.18;
                    border-bottom: 0;
                }
                .dontpad-markdown-editor .wmde-markdown h1 {
                    margin: 0 0 1rem;
                    font-size: 2.1rem;
                }
                .dontpad-markdown-editor .wmde-markdown h2 {
                    margin: 2rem 0 0.85rem;
                    font-size: 1.45rem;
                    padding-bottom: 0.45rem;
                    border-bottom: 1px solid var(--md-border);
                }
                .dontpad-markdown-editor .wmde-markdown h3 {
                    margin: 1.6rem 0 0.65rem;
                    font-size: 1.12rem;
                }
                .dontpad-markdown-editor .wmde-markdown p,
                .dontpad-markdown-editor .wmde-markdown li {
                    color: var(--md-text);
                }
                .dontpad-markdown-editor .wmde-markdown ul,
                .dontpad-markdown-editor .wmde-markdown ol {
                    margin: 0.85rem 0 1.15rem;
                    padding-left: 1.55rem;
                }
                .dontpad-markdown-editor .wmde-markdown ul {
                    list-style: disc outside;
                }
                .dontpad-markdown-editor .wmde-markdown ol {
                    list-style: decimal outside;
                }
                .dontpad-markdown-editor .wmde-markdown ul ul {
                    list-style-type: circle;
                    margin-top: 0.35rem;
                    margin-bottom: 0.35rem;
                }
                .dontpad-markdown-editor .wmde-markdown ul ul ul {
                    list-style-type: square;
                }
                .dontpad-markdown-editor .wmde-markdown ol ol {
                    list-style-type: lower-alpha;
                    margin-top: 0.35rem;
                    margin-bottom: 0.35rem;
                }
                .dontpad-markdown-editor .wmde-markdown li {
                    padding-left: 0.2rem;
                    margin: 0.25rem 0;
                }
                .dontpad-markdown-editor .wmde-markdown li::marker {
                    color: var(--md-accent);
                    font-weight: 650;
                }
                .dontpad-markdown-editor .wmde-markdown ul.contains-task-list,
                .dontpad-markdown-editor .wmde-markdown li.task-list-item {
                    list-style: none;
                }
                .dontpad-markdown-editor .wmde-markdown li.task-list-item {
                    padding-left: 0;
                }
                .dontpad-markdown-editor .wmde-markdown li.task-list-item input[type="checkbox"] {
                    margin: 0 0.55rem 0 -1.35rem;
                    vertical-align: middle;
                    accent-color: var(--md-accent);
                }
                .dontpad-markdown-editor .wmde-markdown a {
                    color: var(--md-accent);
                    text-decoration: underline;
                    text-decoration-thickness: 1px;
                    text-underline-offset: 4px;
                }
                .dontpad-markdown-editor .wmde-markdown blockquote {
                    margin: 1.35rem 0;
                    border-left: 4px solid var(--md-accent);
                    border-radius: 0 8px 8px 0;
                    background: var(--md-panel);
                    color: var(--md-soft);
                    padding: 0.85rem 1rem;
                }
                .dontpad-markdown-editor .wmde-markdown code {
                    border: 1px solid var(--md-border);
                    border-radius: 6px;
                    background: var(--md-code-bg);
                    color: var(--md-code-text);
                    padding: 0.14rem 0.38rem;
                    font-size: 0.88em;
                }
                .dontpad-markdown-editor .wmde-markdown pre {
                    border: 1px solid var(--md-border);
                    border-radius: 10px;
                    background: var(--md-code-bg);
                    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04);
                    padding: 1rem;
                }
                .dontpad-markdown-editor .wmde-markdown pre code {
                    border: 0;
                    background: transparent;
                    padding: 0;
                    color: var(--md-code-text);
                }
                .dontpad-markdown-editor .wmde-markdown table {
                    overflow: hidden;
                    border: 1px solid var(--md-border);
                    border-radius: 10px;
                    border-collapse: separate;
                    border-spacing: 0;
                }
                .dontpad-markdown-editor .wmde-markdown table th {
                    background: var(--md-panel);
                    color: var(--md-text);
                    font-weight: 650;
                }
                .dontpad-markdown-editor .wmde-markdown table th,
                .dontpad-markdown-editor .wmde-markdown table td {
                    border-color: var(--md-border);
                    padding: 0.7rem 0.85rem;
                }
                .dontpad-markdown-editor .wmde-markdown hr {
                    height: 1px;
                    border: 0;
                    background: var(--md-border-strong);
                    margin: 2rem 0;
                }
                @media (max-width: 760px) {
                    .dontpad-markdown-editor .w-md-editor-toolbar {
                        overflow-x: auto;
                    }
                    .dontpad-markdown-editor .w-md-editor-area {
                        border-right: 0;
                    }
                    .dontpad-markdown-editor .w-md-editor-text-input,
                    .dontpad-markdown-editor .w-md-editor-text-pre {
                        padding: 18px 16px !important;
                    }
                    .dontpad-markdown-editor .wmde-markdown {
                        padding: 24px 18px 42px;
                        font-size: 15px;
                    }
                    .dontpad-markdown-editor .wmde-markdown h1 {
                        font-size: 1.7rem;
                    }
                }
            `}</style>
            <div
                className="dontpad-markdown-editor h-full min-h-0"
                data-color-mode="auto"
            >
                {isReadOnly ? (
                    <div className="h-full overflow-auto px-4 py-5 sm:px-6 sm:py-6">
                        <MDPreview
                            source={value}
                            className="wmde-markdown"
                            urlTransform={safeUrlTransform}
                        />
                    </div>
                ) : (
                    <div className="h-full min-h-0 overflow-hidden [&_.w-md-editor]:h-full [&_.w-md-editor]:min-h-0 [&_.w-md-editor]:border-0 [&_.w-md-editor]:bg-transparent [&_.w-md-editor]:shadow-none [&_.w-md-editor-content]:min-h-0 [&_.w-md-editor-preview]:h-full [&_.w-md-editor-preview]:overflow-auto">
                        <MDEditor
                            value={value}
                            onChange={handleChange}
                            height="100%"
                            highlightEnable={false}
                            visibleDragbar={false}
                            preview={viewMode}
                            previewOptions={{ urlTransform: safeUrlTransform }}
                            textareaProps={{
                                placeholder: '# Título\n\nEscreva sua nota em Markdown.',
                            }}
                            style={{ height: '100%', border: 'none', borderRadius: 0, backgroundColor: 'transparent' }}
                        />
                    </div>
                )}
            </div>
        </EditorShell>
    )
}
