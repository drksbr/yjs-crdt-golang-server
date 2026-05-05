'use client'

import '@excalidraw/excalidraw/index.css'
import { useMap } from '@/lib/collab/react'
import dynamic from 'next/dynamic'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { EditorShell } from '@/components/EditorShell'
import {
    createDrawingSceneSnapshot,
    DrawingSceneSnapshot,
    getDrawingSceneComparableSignature,
    hasPersistedDrawingContent,
    readDrawingSceneFromMap,
    writeDrawingSceneToMap,
} from '@/lib/drawingScene'
import { getDrawingStateKey } from '@/lib/noteStateAdapters'
import { useDocumentSecurity } from '@/lib/documentSecurityContext'
import { useDebouncedLastModified } from '@/lib/useDocumentMeta'

const ExcalidrawComp = dynamic(
    () => import('@excalidraw/excalidraw').then(m => ({ default: m.Excalidraw })),
    {
        ssr: false,
        loading: () => (
            <div className="flex-1 flex items-center justify-center bg-white dark:bg-slate-900">
                <div className="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-slate-900 dark:border-slate-100" />
            </div>
        ),
    }
)

interface DrawingEditorProps {
    subdocumentName: string
}

export function DrawingEditor({ subdocumentName }: DrawingEditorProps) {
    const { isReadOnly } = useDocumentSecurity()
    const yMap = useMap(getDrawingStateKey(subdocumentName))
    const touchLastModified = useDebouncedLastModified(subdocumentName)

    // Detect system dark mode for Excalidraw theme
    const [isDark, setIsDark] = useState(false)
    useEffect(() => {
        const mq = window.matchMedia('(prefers-color-scheme: dark)')
        setIsDark(mq.matches)
        const handler = (e: MediaQueryListEvent) => setIsDark(e.matches)
        mq.addEventListener('change', handler)
        return () => mq.removeEventListener('change', handler)
    }, [])
    const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)
    const applyFrameRef = useRef<number | null>(null)
    const apiRef = useRef<any>(null)
    const lastAppliedSceneSigRef = useRef<string>('')

    const initialData = useMemo(() => yMap ? readDrawingSceneFromMap(yMap) : undefined, [yMap])

    const toolbar = (
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
            <span className="text-xs font-medium uppercase tracking-[0.14em] text-slate-500 dark:text-slate-400">
                desenho colaborativo
            </span>
            <span className="text-xs text-slate-500 dark:text-slate-400">
                {isReadOnly ? 'Modo leitura' : 'Quadro sincronizado em tempo real'}
            </span>
        </div>
    )

    const applySceneToCanvas = useCallback((scene: DrawingSceneSnapshot) => {
        const api = apiRef.current
        if (!api) return

        const signature = getDrawingSceneComparableSignature(scene)
        if (signature === lastAppliedSceneSigRef.current) {
            return
        }

        lastAppliedSceneSigRef.current = signature
        api.updateScene({
            elements: scene.elements,
            appState: scene.appState,
            files: scene.files,
        })
    }, [])

    const scheduleSceneApply = useCallback((scene: DrawingSceneSnapshot) => {
        if (applyFrameRef.current !== null) {
            window.cancelAnimationFrame(applyFrameRef.current)
        }
        applyFrameRef.current = window.requestAnimationFrame(() => {
            applyFrameRef.current = null
            applySceneToCanvas(scene)
        })
    }, [applySceneToCanvas])

    // Store the Excalidraw API once ready. Scene loading is intentionally
    // deferred; calling updateScene inside this callback can run before
    // Excalidraw's internal React tree has finished mounting.
    const handleAPI = useCallback((api: any) => {
        apiRef.current = api
        if (!yMap) return

        const scene = readDrawingSceneFromMap(yMap)
        if (hasPersistedDrawingContent(scene)) {
            scheduleSceneApply(scene)
        }
    }, [scheduleSceneApply, yMap])

    // Remote changes (Y.Map observer) → update Excalidraw scene
    useEffect(() => {
        if (!yMap) return
        const handler = (_event: any, transaction: { local?: boolean }) => {
            if (transaction?.local) {
                touchLastModified()
            }
            const scene = readDrawingSceneFromMap(yMap)
            scheduleSceneApply(scene)
        }
        yMap.observe(handler)
        return () => yMap.unobserve(handler)
    }, [scheduleSceneApply, touchLastModified, yMap])

    useEffect(() => {
        return () => {
            if (debounceRef.current) {
                clearTimeout(debounceRef.current)
            }
            if (applyFrameRef.current !== null) {
                window.cancelAnimationFrame(applyFrameRef.current)
                applyFrameRef.current = null
            }
            apiRef.current = null
        }
    }, [])

    // Local Excalidraw changes → write to Y.Map (debounced 80ms)
    const handleChange = useCallback((elements: readonly any[], appState: any, files: Record<string, any>) => {
        if (!yMap) return
        const scene = createDrawingSceneSnapshot(elements, appState, files)
        const signature = getDrawingSceneComparableSignature(scene)
        if (signature === lastAppliedSceneSigRef.current) return

        if (debounceRef.current) clearTimeout(debounceRef.current)
        debounceRef.current = setTimeout(() => {
            lastAppliedSceneSigRef.current = signature
            writeDrawingSceneToMap(yMap, elements, appState, files)
        }, 80)
    }, [yMap])

    return (
        <EditorShell
            variant="full"
            toolbar={toolbar}
            frameClassName="overflow-hidden bg-white dark:bg-slate-950"
            contentClassName="flex min-h-0 overflow-hidden bg-white dark:bg-slate-950"
        >
            {/* position:relative + absolute inset child gives Excalidraw a real
                computed height regardless of how the flex parent resolves. */}
            <div className="relative min-h-0 flex-1">
                <div className="absolute inset-0">
                    <ExcalidrawComp
                        excalidrawAPI={handleAPI}
                        initialData={initialData}
                        onChange={!isReadOnly ? handleChange : undefined}
                        viewModeEnabled={isReadOnly}
                        theme={isDark ? 'dark' : 'light'}
                        UIOptions={{
                            canvasActions: {
                                saveToActiveFile: false,
                                loadScene: false,
                                export: { saveFileToDisk: false },
                            },
                        }}
                    />
                </div>
            </div>
        </EditorShell>
    )
}
