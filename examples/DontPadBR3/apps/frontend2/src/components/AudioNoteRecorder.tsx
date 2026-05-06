'use client'

import { useState, useRef, useEffect } from 'react'

interface AudioNoteRecorderProps {
    documentId: string
    subdocumentId?: string
    onSave: (audioBlob: Blob, duration: number) => Promise<void>
    onClose: () => void
}

export function AudioNoteRecorder({ documentId, subdocumentId, onSave, onClose }: AudioNoteRecorderProps) {
    const [isRecording, setIsRecording] = useState(false)
    const [isPaused, setIsPaused] = useState(false)
    const [duration, setDuration] = useState(0)
    const [isSaving, setIsSaving] = useState(false)
    const [hasAudio, setHasAudio] = useState(false)
    const mediaRecorderRef = useRef<MediaRecorder | null>(null)
    const audioChunksRef = useRef<Blob[]>([])
    const durationIntervalRef = useRef<NodeJS.Timeout | null>(null)
    const streamRef = useRef<MediaStream | null>(null)

    useEffect(() => {
        return () => {
            // Cleanup on unmount
            if (durationIntervalRef.current) {
                clearInterval(durationIntervalRef.current)
            }
            if (streamRef.current) {
                streamRef.current.getTracks().forEach(track => track.stop())
            }
        }
    }, [])

    const startRecording = async () => {
        try {
            const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
            streamRef.current = stream

            const mimeType = MediaRecorder.isTypeSupported('audio/webm')
                ? 'audio/webm'
                : 'audio/mp4'

            const mediaRecorder = new MediaRecorder(stream, { mimeType })
            mediaRecorderRef.current = mediaRecorder
            audioChunksRef.current = []

            mediaRecorder.ondataavailable = (event) => {
                if (event.data.size > 0) {
                    audioChunksRef.current.push(event.data)
                }
            }

            mediaRecorder.start()
            setIsRecording(true)
            setDuration(0)

            // Update duration every 100ms
            durationIntervalRef.current = setInterval(() => {
                setDuration(prev => prev + 0.1)
            }, 100)
        } catch (err) {
            console.error('Erro ao acessar microfone:', err)
            alert('Não foi possível acessar o microfone. Verifique as permissões.')
        }
    }

    const stopRecording = () => {
        if (mediaRecorderRef.current && isRecording) {
            mediaRecorderRef.current.stop()
            setIsRecording(false)
            setIsPaused(false)
            setHasAudio(true)

            if (durationIntervalRef.current) {
                clearInterval(durationIntervalRef.current)
            }

            // Stop all tracks
            if (streamRef.current) {
                streamRef.current.getTracks().forEach(track => track.stop())
            }
        }
    }

    const pauseRecording = () => {
        if (mediaRecorderRef.current && isRecording) {
            mediaRecorderRef.current.pause()
            setIsPaused(true)
            if (durationIntervalRef.current) {
                clearInterval(durationIntervalRef.current)
            }
        }
    }

    const resumeRecording = () => {
        if (mediaRecorderRef.current && isRecording && isPaused) {
            mediaRecorderRef.current.resume()
            setIsPaused(false)
            durationIntervalRef.current = setInterval(() => {
                setDuration(prev => prev + 0.1)
            }, 100)
        }
    }

    const handleSave = async () => {
        if (audioChunksRef.current.length === 0) {
            alert('Nenhum áudio foi gravado')
            return
        }

        try {
            setIsSaving(true)
            const audioBlob = new Blob(audioChunksRef.current, { type: 'audio/webm' })
            await onSave(audioBlob, Math.round(duration * 10) / 10)
            onClose()
        } catch (err) {
            console.error('Erro ao salvar áudio:', err)
            alert('Erro ao salvar a nota de áudio')
        } finally {
            setIsSaving(false)
        }
    }

    const formatTime = (seconds: number) => {
        const mins = Math.floor(seconds / 60)
        const secs = Math.floor(seconds % 60)
        return `${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`
    }

    return (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 backdrop-blur-sm px-4">
            <div className="bg-white dark:bg-slate-900 rounded-lg shadow-xl w-full max-w-md border border-slate-200 dark:border-slate-700">
                {/* Header */}
                <div className="px-6 py-4 border-b border-slate-200 dark:border-slate-700">
                    <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100">Nova Nota de Áudio</h2>
                    <p className="text-slate-600 dark:text-slate-400 text-sm mt-1">Grave sua nota de voz</p>
                </div>

                {/* Content */}
                <div className="p-6">
                    {/* Duration Display */}
                    <div className="bg-slate-50 dark:bg-slate-800 rounded-lg p-6 mb-6 border border-slate-200 dark:border-slate-700">
                        <div className="text-center">
                            <p className="text-slate-500 dark:text-slate-400 text-xs uppercase tracking-wide mb-2">Tempo</p>
                            <div className="text-4xl font-mono font-bold text-slate-900 dark:text-slate-100 tracking-tight">
                                {formatTime(duration)}
                            </div>
                        </div>
                    </div>

                    {/* Recording Status */}
                    {isRecording && (
                        <div className="mb-6 flex items-center justify-center gap-2">
                            <span className="w-2 h-2 bg-red-500 rounded-full animate-pulse"></span>
                            <span className="text-slate-700 dark:text-slate-300 text-sm">{isPaused ? 'Pausado' : 'Gravando'}</span>
                        </div>
                    )}

                    {/* Recording Controls */}
                    <div className="flex justify-center gap-4 mb-6">
                        {!isRecording ? (
                            <button
                                onClick={startRecording}
                                className="w-20 h-20 rounded-full bg-slate-900 dark:bg-slate-100 hover:bg-slate-700 dark:hover:bg-slate-300 text-white dark:text-slate-900 shadow-lg flex items-center justify-center transition transform hover:scale-105"
                                title="Iniciar gravação"
                                aria-label="Iniciar gravação"
                            >
                                <svg
                                    xmlns="http://www.w3.org/2000/svg"
                                    width="32"
                                    height="32"
                                    viewBox="0 0 24 24"
                                    fill="none"
                                    stroke="currentColor"
                                    strokeWidth="2"
                                    strokeLinecap="round"
                                    strokeLinejoin="round"
                                >
                                    <path d="M12 2a3 3 0 0 0-3 3v7a3 3 0 0 0 6 0V5a3 3 0 0 0-3-3Z" />
                                    <path d="M19 10v2a7 7 0 0 1-14 0v-2" />
                                    <line x1="12" x2="12" y1="19" y2="22" />
                                </svg>
                            </button>
                        ) : (
                            <>
                                <button
                                    onClick={pauseRecording}
                                    disabled={isPaused}
                                    className="w-14 h-14 rounded-full bg-slate-200 dark:bg-slate-700 hover:bg-slate-300 dark:hover:bg-slate-600 disabled:opacity-50 disabled:cursor-not-allowed text-slate-900 dark:text-slate-100 flex items-center justify-center transition"
                                    title="Pausar"
                                    aria-label="Pausar"
                                >
                                    <svg
                                        xmlns="http://www.w3.org/2000/svg"
                                        width="20"
                                        height="20"
                                        viewBox="0 0 24 24"
                                        fill="currentColor"
                                    >
                                        <rect x="6" y="4" width="4" height="16" />
                                        <rect x="14" y="4" width="4" height="16" />
                                    </svg>
                                </button>

                                {isPaused && (
                                    <button
                                        onClick={resumeRecording}
                                        className="w-14 h-14 rounded-full bg-slate-900 dark:bg-slate-100 hover:bg-slate-700 dark:hover:bg-slate-300 text-white dark:text-slate-900 flex items-center justify-center transition"
                                        title="Retomar"
                                        aria-label="Retomar"
                                    >
                                        <svg
                                            xmlns="http://www.w3.org/2000/svg"
                                            width="20"
                                            height="20"
                                            viewBox="0 0 24 24"
                                            fill="currentColor"
                                        >
                                            <polygon points="5 3 19 12 5 21 5 3" />
                                        </svg>
                                    </button>
                                )}

                                <button
                                    onClick={stopRecording}
                                    className="w-14 h-14 rounded-full border-2 border-slate-900 dark:border-slate-100 hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-900 dark:text-slate-100 flex items-center justify-center transition"
                                    title="Parar"
                                    aria-label="Parar"
                                >
                                    <svg
                                        xmlns="http://www.w3.org/2000/svg"
                                        width="16"
                                        height="16"
                                        viewBox="0 0 24 24"
                                        fill="currentColor"
                                    >
                                        <rect x="4" y="4" width="16" height="16" />
                                    </svg>
                                </button>
                            </>
                        )}
                    </div>

                    {/* Action Buttons */}
                    <div className="flex gap-3">
                        {hasAudio && !isRecording && (
                            <button
                                onClick={handleSave}
                                disabled={isSaving}
                                className="flex-1 px-4 py-2.5 bg-slate-900 dark:bg-slate-100 hover:bg-slate-700 dark:hover:bg-slate-300 disabled:opacity-50 disabled:cursor-not-allowed text-white dark:text-slate-900 font-medium rounded-md transition text-sm"
                            >
                                {isSaving ? 'Salvando...' : 'Salvar'}
                            </button>
                        )}
                        <button
                            onClick={onClose}
                            className="flex-1 px-4 py-2.5 border border-slate-300 dark:border-slate-600 hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-700 dark:text-slate-300 font-medium rounded-md transition text-sm"
                        >
                            Cancelar
                        </button>
                    </div>
                </div>
            </div>
        </div>
    )
}
