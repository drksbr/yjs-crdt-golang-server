'use client'

import { useText, useAwareness } from '@/lib/collab/react'
import { useEffect, useRef } from 'react'
import { QuillBinding } from 'y-quill'
import 'quill/dist/quill.bubble.css'
import { EditorShell } from '@/components/EditorShell'
import { getTextStateKey } from '@/lib/noteStateAdapters'
import { useDebouncedLastModified } from '@/lib/useDocumentMeta'
import { getSessionIdentity } from '@/lib/session'
import { useDocumentSecurity } from '@/lib/documentSecurityContext'
import { openSafeUrl } from '@/lib/urlSafety'

// Register QuillCursors only once per browser session to avoid
// the "Overwriting modules/cursors" warning on every mount.
let quillCursorsRegistered = false
interface TextEditorProps {
    documentId?: string
    subdocumentName?: string
}

export function TextEditor({ documentId, subdocumentName }: TextEditorProps) {
    // Subdocuments are opened as independent Y.Docs, so they use the same
    // root key as regular documents.
    const textKey = getTextStateKey(subdocumentName)

    const yText = useText(textKey, { observe: 'none' })
    const awareness = useAwareness()
    const editorRef = useRef<HTMLDivElement | null>(null)
    const bindingRef = useRef<QuillBinding | null>(null)
    const quillRef = useRef<any>(null)
    const touchLastModified = useDebouncedLastModified(subdocumentName)
    const { isReadOnly } = useDocumentSecurity()

    // Observe only local text changes to keep lastModified in sync without
    // rebroadcasting metadata churn for remote collaborator edits.
    useEffect(() => {
        if (!yText) return
        const handler = (_event: any, transaction: { local?: boolean }) => {
            if (!transaction?.local) return
            touchLastModified()
        }
        yText.observe(handler)
        return () => yText.unobserve(handler)
    }, [touchLastModified, yText])

    useEffect(() => {
        const editor = editorRef.current
        if (!editor || !awareness || !yText) {
            return
        }

        // Set user identity so QuillBinding shows proper name/color for this client's cursor
        const { name, color } = getSessionIdentity()
        awareness.setLocalStateField('user', { name, color })

        // Wrap awareness so QuillBinding only sees text cursors from users
        // active in the same subdoc context (prevents cursor bleed-through)
        const activeSubdoc = subdocumentName ?? null
        const scopedAwareness = new Proxy(awareness, {
            get(target, prop) {
                if (prop === 'getStates') {
                    return () => {
                        const allStates = target.getStates()
                        const filtered = new Map<number, any>()
                        allStates.forEach((state, clientId) => {
                            if (
                                clientId === target.clientID ||
                                (state?.pagePresence?.activeSubdoc ?? null) === activeSubdoc
                            ) {
                                filtered.set(clientId, state)
                            }
                        })
                        return filtered
                    }
                }
                const value = (target as any)[prop]
                return typeof value === 'function' ? (value as Function).bind(target) : value
            },
        })

        // These libraries are browser-only; import lazily to avoid server-side warnings
        const Quill = require('quill').default || require('quill')
        if (!quillCursorsRegistered) {
            const QuillCursors = require('quill-cursors').default || require('quill-cursors')
            Quill.register('modules/cursors', QuillCursors)
            quillCursorsRegistered = true
        }

        editor.innerHTML = ''
        const quill = new Quill(editor, {
            theme: 'bubble',
            modules: {
                cursors: true,
                toolbar: [
                    [{ header: [1, 2, false] }],
                    ['bold', 'italic', 'underline'],
                    [{ color: [] }, { background: [] }],
                    [{ align: [] }],
                    [{ list: 'ordered' }, { list: 'bullet' }],
                    ['link'],
                ],
            },
        })

        const binding = new QuillBinding(yText, quill, scopedAwareness)
        bindingRef.current = binding
        quillRef.current = quill
        quill.enable(!isReadOnly)

        const handleLinkClick = (e: Event) => {
            const target = e.target as HTMLElement
            if (target.tagName === 'A' && target.hasAttribute('href')) {
                const href = target.getAttribute('href')
                if (href) {
                    openSafeUrl(href)
                }
            }
        }
        editor.addEventListener('click', handleLinkClick)

        return () => {
            editor.removeEventListener('click', handleLinkClick)
            binding.destroy()
            bindingRef.current = null
            quillRef.current = null
            editor.innerHTML = ''
        }
    }, [yText, awareness, subdocumentName])

    // React to isReadOnly changes (e.g. user unlocked edit in public-readonly mode)
    useEffect(() => {
        if (quillRef.current) {
            quillRef.current.enable(!isReadOnly)
        }
    }, [isReadOnly])

    return (
        <>
            <style jsx global>{`
                .ql-tooltip {
                    z-index: 50 !important;
                }
                .ql-container {
                    overflow: visible !important;
                }
                .ql-editor {
                    color: #0f172a;
                    font-size: 1rem;
                    line-height: 1.75rem;
                }
                .dark .ql-editor {
                    color: #e2e8f0;
                }
                .ql-editor.ql-blank::before {
                    left: 15px !important;
                    right: 15px !important;
                    color: #94a3b8 !important;
                    font-style: normal !important;
                    font-size: inherit !important;
                    line-height: inherit !important;
                    letter-spacing: 0 !important;
                    opacity: 1 !important;
                }
                .dark .ql-editor.ql-blank::before {
                    color: #64748b !important;
                }
                /* Hide toolbar bubble when editor is disabled (readonly) */
                .ql-disabled .ql-bubble .ql-toolbar,
                .ql-editor.ql-disabled + .ql-tooltip {
                    display: none !important;
                }
                .ql-color, .ql-background {
                    width: 28px !important;
                }
                .ql-color .ql-picker-label::before,
                .ql-background .ql-picker-label::before {
                    border-radius: 3px;
                }
                .ql-picker-options {
                    padding: 8px 4px;
                }
                .ql-color-label, .ql-background-label, .ql-align .ql-picker-label {
                    font-size: 12px;
                }
                /* Link styling */
                .ql-editor a {
                    color: #2563eb;
                    text-decoration: underline;
                    cursor: pointer;
                    transition: color 0.2s ease;
                }
                .ql-editor a::after {
                    content: ' ↗';
                    font-size: 0.75em;
                    opacity: 0.6;
                    transition: opacity 0.2s ease;
                }
                .ql-editor a:hover {
                    color: #1d4ed8;
                }
                .ql-editor a:hover::after {
                    opacity: 1;
                }
                .dark .ql-editor a {
                    color: #60a5fa;
                }
                .dark .ql-editor a:hover {
                    color: #93c5fd;
                }
                /* In dark mode, force dark text over any highlight background so
                   colours applied in light mode remain readable */
                .dark .ql-editor [style*="background-color"] {
                    color: #0f172a !important;
                }
                /* But if the user also explicitly set a text colour, respect it by
                   overriding only when the element carries no inline color itself.
                   We achieve this by restoring the colour on a narrower selector
                   that matches both attributes together. */
                .dark .ql-editor [style*="background-color"][style*="color"] {
                    color: revert !important;
                }
            `}</style>
            <EditorShell
                variant="narrow"
                frameClassName="overflow-visible bg-white dark:bg-slate-900"
                contentClassName="overflow-visible rounded-[inherit] bg-white dark:bg-slate-900"
            >
                <div
                    ref={editorRef}
                    data-command-palette-scope="editor"
                    className="h-full overflow-auto px-4 py-5 sm:px-6 sm:py-6"
                />
            </EditorShell>
        </>
    )
}
