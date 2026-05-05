"use client";

interface ChecklistComposerProps {
    value: string;
    placeholder?: string;
    onChange: (value: string) => void;
    onSubmit: () => void;
    disabled?: boolean;
}

export function ChecklistComposer({
    value,
    placeholder = "Nova tarefa...",
    onChange,
    onSubmit,
    disabled = false,
}: ChecklistComposerProps) {
    return (
        <div className="flex flex-col gap-2 sm:flex-row">
            <input
                value={value}
                onChange={(event) => onChange(event.target.value)}
                onKeyDown={(event) => {
                    if (event.key === "Enter") {
                        event.preventDefault();
                        onSubmit();
                    }
                }}
                placeholder={placeholder}
                disabled={disabled}
                className="flex-1 rounded-xl border border-slate-200 bg-white px-3 py-2.5 text-sm text-slate-900 transition placeholder-slate-400 focus:outline-none focus:ring-1 focus:ring-slate-400 disabled:cursor-not-allowed disabled:opacity-60 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100 dark:placeholder-slate-500 dark:focus:ring-slate-500"
            />
            <button
                type="button"
                onClick={onSubmit}
                disabled={disabled || !value.trim()}
                className="inline-flex items-center justify-center rounded-xl bg-slate-950 px-4 py-2.5 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-40 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-slate-200"
            >
                Adicionar
            </button>
        </div>
    );
}
