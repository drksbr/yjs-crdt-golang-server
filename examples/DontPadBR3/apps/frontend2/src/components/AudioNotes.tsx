'use client'

import { useState, useRef, useEffect } from 'react'
import { useArray } from '@/lib/collab/react'
import { AudioNote } from '@/lib/types'
import { AudioNoteRecorder } from './AudioNoteRecorder'
import { useDocumentSecurity } from '@/lib/documentSecurityContext'

interface AudioNotesProps {
    documentId: string
    subdocumentId?: string
    useRootState?: boolean
    embedded?: boolean
}

function getAudioNotesStateKey(documentId: string, subdocumentId?: string, useRootState = false) {
    const scope = subdocumentId && !useRootState ? `${documentId}.${subdocumentId}` : documentId
    return `audioNotes.${scope}`
}

function isAudioNoteInScope(note: AudioNote, documentId: string, subdocumentId?: string, useRootState = false) {
    if (!note?.id) return false
    if (note.documentId !== documentId) return false
    const expectedSubdocumentId = subdocumentId && !useRootState ? subdocumentId : null
    return (note.subdocumentId ?? null) === expectedSubdocumentId
}

export function AudioNotes({
    documentId,
    subdocumentId,
    useRootState = false,
    embedded = false,
}: AudioNotesProps) {
    const [showRecorder, setShowRecorder] = useState(false)
    const [playingId, setPlayingId] = useState<string | null>(null)
    const [loading, setLoading] = useState(false)
    const [error, setError] = useState<string | null>(null)
    const { isReadOnly } = useDocumentSecurity()

    // Ref para guardar a instância de áudio em execução
    const currentAudioRef = useRef<HTMLAudioElement | null>(null)

    // Keep audio metadata scoped by document. The legacy root key `audioNotes`
    // is intentionally not reused because it can leak notes across documents.
    const audioNotesArray = useArray(getAudioNotesStateKey(documentId, subdocumentId, useRootState))

    // Converter array Y-Sweet para array normal para o render
    const audioNotes = audioNotesArray
        ? (audioNotesArray.toArray() as AudioNote[])
            .filter((note) => isAudioNoteInScope(note, documentId, subdocumentId, useRootState))
        : []

    useEffect(() => {
        if (!audioNotesArray) return

        let cancelled = false
        const queryParams = new URLSearchParams({
            documentId,
            ...(subdocumentId && !useRootState ? { subdocumentId } : {}),
        })

        fetch(`/api/audio-notes?${queryParams.toString()}`, {
            cache: 'no-store',
        })
            .then((response) => {
                if (!response.ok) throw new Error('Falha ao carregar notas de áudio')
                return response.json()
            })
            .then((serverNotes: AudioNote[]) => {
                if (cancelled || !Array.isArray(serverNotes)) return
                for (let index = audioNotesArray.length - 1; index >= 0; index--) {
                    const item = audioNotesArray.get(index) as AudioNote | undefined
                    if (!item?.id || !isAudioNoteInScope(item, documentId, subdocumentId, useRootState)) {
                        audioNotesArray.delete(index)
                    }
                }
                const existingIds = new Set(
                    Array.from(audioNotesArray)
                        .filter((item: any) => isAudioNoteInScope(item, documentId, subdocumentId, useRootState))
                        .map((item: any) => item?.id)
                        .filter(Boolean),
                )
                const missing = serverNotes
                    .filter((note) => isAudioNoteInScope(note, documentId, subdocumentId, useRootState))
                    .filter((note) => !existingIds.has(note.id))
                if (missing.length > 0) {
                    audioNotesArray.push(missing)
                }
            })
            .catch(() => {
                // O Yjs continua exibindo as notas já sincronizadas mesmo sem a lista remota.
            })

        return () => {
            cancelled = true
        }
    }, [audioNotesArray, documentId, subdocumentId, useRootState])

    const handleSaveAudio = async (audioBlob: Blob, duration: number) => {
        if (isReadOnly) {
            return
        }

        try {
            setLoading(true)
            setError(null)

            const formData = new FormData()
            formData.append('audio', audioBlob)
            formData.append('duration', duration.toString())
            formData.append('documentId', documentId)
            if (subdocumentId) {
                formData.append('subdocumentId', subdocumentId)
            }

            const response = await fetch('/api/audio-notes', {
                method: 'POST',
                body: formData,
            })

            if (!response.ok) {
                throw new Error('Falha ao salvar nota de áudio')
            }

            const newNote = await response.json()

            // Add to Y-Sweet array
            if (audioNotesArray) {
                audioNotesArray.push([{
                    ...newNote,
                    documentId,
                    subdocumentId: subdocumentId ?? null,
                }])
            }

            setShowRecorder(false)
        } catch (err) {
            console.error('[AudioNotes] Erro ao salvar áudio:', err)
            setError(err instanceof Error ? err.message : 'Erro ao salvar nota de áudio')
        } finally {
            setLoading(false)
        }
    }

    const handleDeleteAudio = async (noteId: string) => {
        if (isReadOnly) {
            return
        }

        try {
            setLoading(true)
            setError(null)

            const response = await fetch('/api/audio-notes', {
                method: 'DELETE',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    documentId,
                    noteId,
                    subdocumentId,
                }),
            })

            if (!response.ok) {
                throw new Error('Falha ao deletar nota de áudio')
            }

            // Remove from Y-Sweet array
            if (audioNotesArray) {
                const index = audioNotes.findIndex(n => n.id === noteId)
                if (index !== -1) {
                    audioNotesArray.delete(index)
                }
            }
        } catch (err) {
            console.error('[AudioNotes] Erro ao deletar áudio:', err)
            setError(err instanceof Error ? err.message : 'Erro ao deletar nota de áudio')
        } finally {
            setLoading(false)
        }
    }

    const playAudio = async (noteId: string) => {
        try {
            // Se o mesmo áudio está tocando, parar
            if (playingId === noteId) {
                if (currentAudioRef.current) {
                    currentAudioRef.current.pause()
                    currentAudioRef.current.currentTime = 0
                }
                setPlayingId(null)
                return
            }

            // Se outro áudio está tocando, parar primeiro
            if (playingId !== null && currentAudioRef.current) {
                currentAudioRef.current.pause()
                currentAudioRef.current.currentTime = 0
            }

            setPlayingId(noteId)

            const response = await fetch(`/api/audio-notes/${noteId}`, {
                headers: {
                    'X-Document-Id': documentId,
                    'X-Subdocument-Id': subdocumentId || '',
                },
            })

            if (!response.ok) {
                throw new Error('Falha ao carregar áudio')
            }

            const audioBlob = await response.blob()
            const audioUrl = URL.createObjectURL(audioBlob)
            const audio = new Audio(audioUrl)
            currentAudioRef.current = audio

            audio.onended = () => {
                setPlayingId(null)
                URL.revokeObjectURL(audioUrl)
                currentAudioRef.current = null
            }

            audio.play().catch(err => {
                console.error('[AudioNotes] Erro ao reproduzir áudio:', err)
                setPlayingId(null)
                currentAudioRef.current = null
            })
        } catch (err) {
            console.error('[AudioNotes] Erro ao reproduzir áudio:', err)
            setError('Erro ao reproduzir áudio')
            setPlayingId(null)
            currentAudioRef.current = null
        }
    }

    const formatDuration = (seconds: number) => {
        const mins = Math.floor(seconds / 60)
        const secs = Math.floor(seconds % 60)
        return `${mins}:${secs.toString().padStart(2, '0')}`
    }

    const formatDate = (timestamp: number) => {
        return new Date(timestamp).toLocaleDateString('pt-BR', {
            year: 'numeric',
            month: 'short',
            day: 'numeric',
            hour: '2-digit',
            minute: '2-digit',
        })
    }

    return (
        <div className="flex flex-col gap-4 h-full">
            {/* Header with Add Button */}
            <div className={`flex items-center gap-3 ${embedded ? "justify-end pt-0" : "justify-between"}`}>
                {!embedded && (
                    <h3 className="font-semibold text-slate-900 dark:text-slate-100 text-base">
                        Notas de Áudio
                    </h3>
                )}
                <button
                    onClick={() => setShowRecorder(true)}
                    disabled={loading || isReadOnly}
                    className="flex items-center gap-1.5 px-3 py-1.5 text-sm bg-slate-900 dark:bg-slate-100 hover:bg-slate-700 dark:hover:bg-slate-300 disabled:opacity-50 disabled:cursor-not-allowed text-white dark:text-slate-900 font-medium rounded-md transition"
                    title="Gravar nova nota de áudio"
                    aria-label="Gravar nova nota de áudio"
                >
                    <svg
                        xmlns="http://www.w3.org/2000/svg"
                        width="16"
                        height="16"
                        viewBox="0 0 24 24"
                        fill="none"
                        stroke="currentColor"
                        strokeWidth="2"
                        strokeLinecap="round"
                        strokeLinejoin="round"
                    >
                        <circle cx="12" cy="12" r="10" />
                        <line x1="12" x2="12" y1="8" y2="16" />
                        <line x1="8" x2="16" y1="12" y2="12" />
                    </svg>
                    {isReadOnly ? 'Somente leitura' : 'Nova'}
                </button>
            </div>

            {/* Error Message */}
            {error && (
                <div className="p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 text-red-700 dark:text-red-300 text-sm rounded-md flex items-start gap-2">
                    <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="flex-shrink-0 mt-0.5">
                        <circle cx="12" cy="12" r="10" />
                        <line x1="12" x2="12" y1="8" y2="12" />
                        <line x1="12" x2="12.01" y1="16" y2="16" />
                    </svg>
                    <span>{error}</span>
                </div>
            )}

            {/* Audio Notes List */}
            <div className="flex-1 overflow-y-auto space-y-2">
                {audioNotes.length === 0 ? (
                    <div className="flex flex-col items-center justify-center py-16 text-center">
                        <svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" className="text-slate-300 dark:text-slate-600 mb-3">
                            <path d="M12 2a3 3 0 0 0-3 3v7a3 3 0 0 0 6 0V5a3 3 0 0 0-3-3Z" />
                            <path d="M19 10v2a7 7 0 0 1-14 0v-2" />
                            <line x1="12" x2="12" y1="19" y2="22" />
                        </svg>
                        <p className="text-slate-500 dark:text-slate-400 text-sm font-medium">
                            Nenhuma nota gravada
                        </p>
                        <p className="text-slate-400 dark:text-slate-500 text-xs mt-1">
                            Clique em Nova para começar
                        </p>
                    </div>
                ) : (
                    audioNotes.map(note => (
                        <div
                            key={note.id}
                            className="flex items-center gap-3 p-3 bg-white dark:bg-slate-800 rounded-md border border-slate-200 dark:border-slate-700 hover:border-slate-300 dark:hover:border-slate-600 transition group"
                        >
                            <button
                                onClick={() => playAudio(note.id)}
                                disabled={playingId !== null && playingId !== note.id}
                                className="flex-shrink-0 w-9 h-9 rounded-full bg-slate-900 dark:bg-slate-100 hover:bg-slate-700 dark:hover:bg-slate-300 disabled:opacity-50 disabled:cursor-not-allowed text-white dark:text-slate-900 flex items-center justify-center transition"
                                title={playingId === note.id ? "Parar" : "Reproduzir"}
                                aria-label="Reproduzir"
                            >
                                {playingId === note.id ? (
                                    <svg
                                        xmlns="http://www.w3.org/2000/svg"
                                        width="14"
                                        height="14"
                                        viewBox="0 0 24 24"
                                        fill="currentColor"
                                    >
                                        <rect x="6" y="4" width="4" height="16" />
                                        <rect x="14" y="4" width="4" height="16" />
                                    </svg>
                                ) : (
                                    <svg
                                        xmlns="http://www.w3.org/2000/svg"
                                        width="14"
                                        height="14"
                                        viewBox="0 0 24 24"
                                        fill="currentColor"
                                    >
                                        <polygon points="5 3 19 12 5 21 5 3" />
                                    </svg>
                                )}
                            </button>

                            <div className="flex-1 min-w-0">
                                <div className="text-sm font-medium text-slate-900 dark:text-slate-100 truncate">
                                    {formatDuration(note.duration)}
                                </div>
                                <div className="text-xs text-slate-500 dark:text-slate-400">
                                    {formatDate(note.createdAt)}
                                </div>
                            </div>

                            <button
                                onClick={() => handleDeleteAudio(note.id)}
                                disabled={loading || isReadOnly}
                                className="flex-shrink-0 w-8 h-8 rounded text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-700 transition md:opacity-0 md:group-hover:opacity-100 disabled:cursor-not-allowed flex items-center justify-center opacity-100"
                                title="Deletar"
                                aria-label="Deletar"
                            >
                                <svg
                                    xmlns="http://www.w3.org/2000/svg"
                                    width="16"
                                    height="16"
                                    viewBox="0 0 24 24"
                                    fill="none"
                                    stroke="currentColor"
                                    strokeWidth="2"
                                    strokeLinecap="round"
                                    strokeLinejoin="round"
                                >
                                    <polyline points="3 6 5 6 21 6" />
                                    <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" />
                                    <line x1="10" y1="11" x2="10" y2="17" />
                                    <line x1="14" y1="11" x2="14" y2="17" />
                                </svg>
                            </button>
                        </div>
                    ))
                )}
            </div>

            {/* Recorder Modal */}
            {showRecorder && (
                <AudioNoteRecorder
                    documentId={documentId}
                    subdocumentId={subdocumentId}
                    onSave={handleSaveAudio}
                    onClose={() => setShowRecorder(false)}
                />
            )}
        </div>
    )
}
