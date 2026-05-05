'use client'

import '@uiw/react-md-editor/markdown-editor.css'
import { useText } from '@/lib/collab/react'
import { useEffect, useRef, useState } from 'react'
import dynamic from 'next/dynamic'
import * as Y from 'yjs'
import { EditorShell } from '@/components/EditorShell'
import { getMarkdownStateKey } from '@/lib/noteStateAdapters'
import { useDocumentSecurity } from '@/lib/documentSecurityContext'
import { useDebouncedLastModified } from '@/lib/useDocumentMeta'
import { getSafeUrl } from '@/lib/urlSafety'

const MDEditor = dynamic(() => import('@uiw/react-md-editor'), { ssr: false })
const MDPreview = dynamic(() => import('@uiw/react-md-editor').then(m => m.default.Markdown), { ssr: false })

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
    const prevValueRef = useRef('')
    const isLocalChangeRef = useRef(false)

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

    return (
        <EditorShell
            variant="narrow"
            frameClassName="overflow-hidden bg-white dark:bg-slate-900"
            contentClassName="overflow-hidden bg-white dark:bg-slate-900"
            className="min-h-0"
        >
            <div
                className="h-full min-h-0"
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
                    <div className="h-full min-h-0 overflow-hidden [&_.w-md-editor]:h-full [&_.w-md-editor]:border-0 [&_.w-md-editor]:bg-transparent [&_.w-md-editor]:shadow-none [&_.w-md-editor-content]:h-full [&_.w-md-editor-preview]:h-full [&_.w-md-editor-preview]:overflow-auto">
                        <MDEditor
                            value={value}
                            onChange={handleChange}
                            visibleDragbar={false}
                            preview="live"
                            previewOptions={{ urlTransform: safeUrlTransform }}
                            style={{ height: '100%', border: 'none', borderRadius: 0, backgroundColor: 'transparent' }}
                        />
                    </div>
                )}
            </div>
        </EditorShell>
    )
}
