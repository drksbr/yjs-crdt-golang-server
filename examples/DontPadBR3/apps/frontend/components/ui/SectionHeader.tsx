"use client";

interface SectionHeaderProps {
    eyebrow?: string;
    title: string;
    description?: string;
    align?: "left" | "center";
    theme?: "light" | "dark";
    className?: string;
}

export function SectionHeader({
    eyebrow,
    title,
    description,
    align = "center",
    theme = "light",
    className = "",
}: SectionHeaderProps) {
    const isDark = theme === "dark";
    const alignment = align === "center" ? "text-center mx-auto" : "text-left";

    return (
        <div className={`${alignment} ${className}`.trim()}>
            {eyebrow && (
                <span
                    className={`eyebrow ${isDark
                        ? "border-white/10 bg-white/5 text-slate-300"
                        : "border-slate-200/80 bg-white/80 text-slate-500"
                        }`}
                >
                    {eyebrow}
                </span>
            )}
            <h2
                className={`mt-4 text-3xl font-semibold tracking-tight sm:text-4xl ${isDark ? "text-white" : "text-slate-950 dark:text-slate-100"
                    }`}
            >
                {title}
            </h2>
            {description && (
                <p
                    className={`mt-4 max-w-2xl text-base leading-7 sm:text-lg ${isDark ? "text-slate-300" : "text-slate-600 dark:text-slate-400"
                        } ${align === "center" ? "mx-auto" : ""}`}
                >
                    {description}
                </p>
            )}
        </div>
    );
}
