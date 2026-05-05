'use client'

import { useEffect, useRef } from 'react'

interface ConfirmDeleteModalProps {
    isOpen: boolean
    title: string
    message: string
    itemName: string
    isLoading?: boolean
    onConfirm: () => void | Promise<void>
    onCancel: () => void
}

export function ConfirmDeleteModal({
    isOpen,
    title,
    message,
    itemName,
    isLoading = false,
    onConfirm,
    onCancel,
}: ConfirmDeleteModalProps) {
    const modalRef = useRef<HTMLDivElement>(null)

    useEffect(() => {
        if (!isOpen) return

        const handleEscape = (e: KeyboardEvent) => {
            if (e.key === 'Escape') {
                onCancel()
            }
        }

        document.addEventListener('keydown', handleEscape)
        return () => document.removeEventListener('keydown', handleEscape)
    }, [isOpen, onCancel])

    if (!isOpen) return null

    return (
        <>
            {/* Backdrop */}
            <div
                className="fixed inset-0 z-40 bg-black/50 transition-opacity"
                onClick={onCancel}
                aria-hidden="true"
            />

            {/* Modal */}
            <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
                <div
                    ref={modalRef}
                    className="w-full max-w-sm bg-white dark:bg-slate-800 rounded-lg shadow-lg border border-slate-200 dark:border-slate-700 animate-in fade-in zoom-in-95 duration-200"
                    role="dialog"
                    aria-modal="true"
                    aria-labelledby="modal-title"
                >
                    {/* Header */}
                    <div className="px-6 py-4 border-b border-slate-200 dark:border-slate-700">
                        <h2
                            id="modal-title"
                            className="text-lg font-semibold text-slate-900 dark:text-slate-100"
                        >
                            {title}
                        </h2>
                    </div>

                    {/* Content */}
                    <div className="px-6 py-4">
                        <p className="text-sm text-slate-600 dark:text-slate-400">
                            {message}
                        </p>
                        <div className="mt-3 p-3 bg-slate-50 dark:bg-slate-700/50 border border-slate-200 dark:border-slate-600 rounded-md">
                            <p className="text-sm font-medium text-slate-900 dark:text-slate-100 break-words">
                                "{itemName}"
                            </p>
                        </div>
                    </div>

                    {/* Footer */}
                    <div className="px-6 py-4 border-t border-slate-200 dark:border-slate-700 flex gap-3 justify-end">
                        <button
                            onClick={onCancel}
                            disabled={isLoading}
                            className="px-4 py-2 text-sm font-medium text-slate-900 dark:text-slate-100 bg-slate-100 dark:bg-slate-700 hover:bg-slate-200 dark:hover:bg-slate-600 rounded-md transition disabled:opacity-50 disabled:cursor-not-allowed"
                        >
                            Cancelar
                        </button>
                        <button
                            onClick={onConfirm}
                            disabled={isLoading}
                            className="px-4 py-2 text-sm font-medium text-white bg-red-600 hover:bg-red-700 rounded-md transition disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
                        >
                            {isLoading ? (
                                <>
                                    <span className="inline-block w-3 h-3 border-2 border-white border-t-transparent rounded-full animate-spin" />
                                    Deletando...
                                </>
                            ) : (
                                <>
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
                                        <path d="M3 6h18" />
                                        <path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6" />
                                        <path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2" />
                                    </svg>
                                    Deletar
                                </>
                            )}
                        </button>
                    </div>
                </div>
            </div>
        </>
    )
}
