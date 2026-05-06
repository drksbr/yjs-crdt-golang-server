"use client";

import { useState, useCallback } from "react";

interface ToastProps {
    message: string;
    duration?: number;
}

export function useToast() {
    const [toast, setToast] = useState<ToastProps | null>(null);

    const showToast = useCallback((message: string, duration = 3000) => {
        setToast({ message, duration });
        const timer = setTimeout(() => setToast(null), duration);
        return () => clearTimeout(timer);
    }, []);

    return { toast, showToast };
}

export function Toast({ message, onClose }: { message: string; onClose: () => void }) {
    return (
        <div
            className="fixed bottom-6 right-6 bg-slate-900 dark:bg-slate-100 text-white dark:text-slate-900 px-4 py-3 rounded-lg shadow-lg flex items-center gap-2 animate-in fade-in slide-in-from-bottom-4 duration-300 z-50"
            role="alert"
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
                <polyline points="20 6 9 17 4 12"></polyline>
            </svg>
            <span className="text-sm font-medium">{message}</span>
            <button
                onClick={onClose}
                className="ml-2 text-slate-300 dark:text-slate-600 hover:text-white dark:hover:text-slate-900 transition"
                aria-label="Close"
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
                    <line x1="18" y1="6" x2="6" y2="18"></line>
                    <line x1="6" y1="6" x2="18" y2="18"></line>
                </svg>
            </button>
        </div>
    );
}
