"use client";

import { ReactNode, useEffect } from "react";

interface ResponsivePanelProps {
    open: boolean;
    title: string;
    onClose: () => void;
    children: ReactNode;
}

function PanelFrame({
    title,
    onClose,
    children,
    className,
    mobile = false,
}: ResponsivePanelProps & { className: string; mobile?: boolean }) {
    return (
        <aside className={className}>
            {mobile && (
                <div className="flex justify-center px-4 pt-3">
                    <div className="h-1.5 w-12 rounded-full bg-slate-300 dark:bg-slate-600" />
                </div>
            )}

            <div className="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 px-5 py-4">
                <div>
                    <h2 className="text-sm font-semibold uppercase tracking-[0.16em] text-slate-900 dark:text-slate-100">
                        {title}
                    </h2>
                </div>
                <button
                    type="button"
                    onClick={onClose}
                    className="inline-flex h-9 w-9 items-center justify-center rounded-xl text-slate-500 transition hover:bg-slate-100 hover:text-slate-900 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-slate-100"
                    aria-label={`Fechar ${title.toLowerCase()}`}
                >
                    <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                        <line x1="18" y1="6" x2="6" y2="18"></line>
                        <line x1="6" y1="6" x2="18" y2="18"></line>
                    </svg>
                </button>
            </div>

            <div className="min-h-0 flex-1 overflow-hidden px-5 py-5">
                {children}
            </div>
        </aside>
    );
}

export function ResponsivePanel({
    open,
    title,
    onClose,
    children,
}: ResponsivePanelProps) {
    useEffect(() => {
        if (!open) return;

        const previousOverflow = document.body.style.overflow;
        document.body.style.overflow = "hidden";

        return () => {
            document.body.style.overflow = previousOverflow;
        };
    }, [open]);

    useEffect(() => {
        if (!open) return;

        const handleEscape = (event: KeyboardEvent) => {
            if (event.key === "Escape") {
                onClose();
            }
        };

        document.addEventListener("keydown", handleEscape);
        return () => document.removeEventListener("keydown", handleEscape);
    }, [onClose, open]);

    if (!open) return null;

    return (
        <>
            <PanelFrame
                open={open}
                title={title}
                onClose={onClose}
                className="hidden h-full min-h-0 overflow-hidden md:flex md:w-[22rem] lg:w-96 md:flex-col md:border-l md:border-slate-200 md:bg-white md:shadow-[0_0_0_1px_rgba(15,23,42,0.02),-24px_0_48px_-36px_rgba(15,23,42,0.25)] dark:md:border-slate-800 dark:md:bg-slate-900"
            >
                {children}
            </PanelFrame>

            <div
                className="fixed inset-0 z-30 bg-slate-950/35 md:hidden"
                onClick={onClose}
                role="presentation"
            />

            <PanelFrame
                open={open}
                title={title}
                onClose={onClose}
                mobile
                className="fixed inset-x-0 bottom-0 z-40 flex max-h-[85dvh] min-h-0 flex-col overflow-hidden rounded-t-[1.75rem] border border-slate-200 bg-white shadow-2xl dark:border-slate-800 dark:bg-slate-900 md:hidden"
            >
                {children}
            </PanelFrame>
        </>
    );
}
