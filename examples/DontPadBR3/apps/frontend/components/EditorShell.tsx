"use client";

import { ReactNode } from "react";
import { Surface } from "./ui/Surface";

type EditorShellVariant = "narrow" | "medium" | "full";
type EditorShellSurface = "default" | "muted" | "glass" | "inverted";

interface EditorShellProps {
    children: ReactNode;
    toolbar?: ReactNode;
    variant?: EditorShellVariant;
    surfaceVariant?: EditorShellSurface;
    className?: string;
    frameClassName?: string;
    contentClassName?: string;
}

export function EditorShell({
    children,
    toolbar,
    variant = "medium",
    surfaceVariant = "default",
    className = "",
    frameClassName = "",
    contentClassName = "",
}: EditorShellProps) {
    const widthClass =
        variant === "narrow"
            ? "max-w-none sm:max-w-[min(88rem,calc(100vw-clamp(2rem,11vw,18rem)))]"
            : variant === "medium"
                ? "max-w-none sm:max-w-[min(104rem,calc(100vw-clamp(1.5rem,8vw,12rem)))]"
                : "max-w-none sm:max-w-[min(100%,calc(100vw-clamp(1rem,4vw,5rem)))]";

    const spacingClass =
        variant === "full"
            ? "px-0 py-0 sm:px-3 sm:py-3 lg:px-4 lg:py-4"
            : "px-0 py-0 sm:px-4 sm:py-4 lg:px-6 lg:py-5 2xl:px-8";

    return (
        <div data-command-palette-scope="editor" className={`flex h-full min-h-0 w-full flex-col ${className}`.trim()}>
            <div className={`mx-auto flex h-full min-h-0 w-full flex-1 flex-col ${spacingClass} ${widthClass}`.trim()}>
                <Surface variant={surfaceVariant} className={`flex h-full min-h-0 flex-col ${frameClassName}`.trim()}>
                    {toolbar ? (
                        <div className="border-b border-slate-200/80 px-4 py-3 dark:border-slate-700/80 sm:px-5">
                            {toolbar}
                        </div>
                    ) : null}
                    <div className={`min-h-0 flex-1 ${contentClassName}`.trim()}>{children}</div>
                </Surface>
            </div>
        </div>
    );
}
