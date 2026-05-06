"use client";

import { ReactNode } from "react";

interface SurfaceProps {
    children: ReactNode;
    className?: string;
    variant?: "default" | "muted" | "glass" | "inverted";
}

export function Surface({
    children,
    className = "",
    variant = "default",
}: SurfaceProps) {
    const variantClass =
        variant === "muted"
            ? "surface-soft"
            : variant === "glass"
                ? "surface-glass"
                : variant === "inverted"
                    ? "surface-inverted"
                    : "surface-card";

    return <div className={`${variantClass} ${className}`.trim()}>{children}</div>;
}
