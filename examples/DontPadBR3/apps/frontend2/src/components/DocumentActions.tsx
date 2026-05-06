"use client";

import { ReactNode } from "react";

export type DocumentPanelId = "subdocs" | "files" | "audio";

interface DocumentActionsProps {
    activePanel: DocumentPanelId | null;
    onTogglePanel: (panel: DocumentPanelId) => void;
    onCopyLink: () => void;
    onOpenSettings: () => void;
    isSettingsOpen?: boolean;
}

interface ActionButtonProps {
    active?: boolean;
    label: string;
    icon: ReactNode;
    onClick: () => void;
    iconOnly?: boolean;
    expanded?: boolean;
}

function ActionButton({
    active = false,
    label,
    icon,
    onClick,
    iconOnly = false,
    expanded,
}: ActionButtonProps) {
    return (
        <button
            type="button"
            onClick={onClick}
            aria-label={label}
            aria-expanded={expanded}
            className={`inline-flex h-10 items-center justify-center gap-2 rounded-xl border px-3 text-sm font-medium transition ${iconOnly ? "w-10 px-0" : ""} ${active
                ? "border-slate-300 bg-slate-100 text-slate-900 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-100"
                : "border-slate-200 bg-white text-slate-600 hover:border-slate-300 hover:bg-slate-50 hover:text-slate-900 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-300 dark:hover:border-slate-600 dark:hover:bg-slate-800 dark:hover:text-slate-100"
                }`}
        >
            {icon}
            {!iconOnly && <span className="hidden md:inline-flex">{label}</span>}
        </button>
    );
}

export function DocumentActions({
    activePanel,
    onTogglePanel,
    onCopyLink,
    onOpenSettings,
    isSettingsOpen = false,
}: DocumentActionsProps) {
    return (
        <div className="flex items-center gap-2">
            <ActionButton
                active={activePanel === "subdocs"}
                label="Subdocs"
                onClick={() => onTogglePanel("subdocs")}
                icon={
                    <svg xmlns="http://www.w3.org/2000/svg" width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                        <rect x="3" y="3" width="7" height="7"></rect>
                        <rect x="14" y="3" width="7" height="7"></rect>
                        <rect x="14" y="14" width="7" height="7"></rect>
                        <rect x="3" y="14" width="7" height="7"></rect>
                    </svg>
                }
                iconOnly={false}
            />

            <ActionButton
                active={activePanel === "files"}
                label="Arquivos"
                onClick={() => onTogglePanel("files")}
                icon={
                    <svg xmlns="http://www.w3.org/2000/svg" width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                        <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path>
                        <polyline points="14 2 14 8 20 8"></polyline>
                    </svg>
                }
                iconOnly={false}
            />

            <ActionButton
                active={activePanel === "audio"}
                label="Áudio"
                onClick={() => onTogglePanel("audio")}
                icon={
                    <svg xmlns="http://www.w3.org/2000/svg" width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                        <path d="M12 2a3 3 0 0 0-3 3v7a3 3 0 0 0 6 0V5a3 3 0 0 0-3-3Z"></path>
                        <path d="M19 10v2a7 7 0 0 1-14 0v-2"></path>
                        <line x1="12" x2="12" y1="19" y2="22"></line>
                    </svg>
                }
                iconOnly={false}
            />

            <ActionButton
                label="Compartilhar"
                onClick={onCopyLink}
                iconOnly
                icon={
                    <svg xmlns="http://www.w3.org/2000/svg" width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                        <circle cx="18" cy="5" r="3"></circle>
                        <circle cx="6" cy="12" r="3"></circle>
                        <circle cx="18" cy="19" r="3"></circle>
                        <line x1="8.59" y1="13.51" x2="15.42" y2="17.49"></line>
                        <line x1="15.41" y1="6.51" x2="8.59" y2="10.49"></line>
                    </svg>
                }
            />

            <ActionButton
                active={isSettingsOpen}
                label="Configurações"
                onClick={onOpenSettings}
                iconOnly
                expanded={isSettingsOpen}
                icon={
                    <svg xmlns="http://www.w3.org/2000/svg" width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                        <path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"></path>
                        <circle cx="12" cy="12" r="3"></circle>
                    </svg>
                }
            />
        </div>
    );
}
