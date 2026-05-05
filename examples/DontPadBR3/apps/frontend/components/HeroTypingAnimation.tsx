"use client";

import { useState, useEffect, useRef, useCallback } from "react";

interface Collaborator {
    name: string;
    lightColor: string;
    darkColor: string;
}

// Professional colors with light/dark variants for proper contrast
const collaborators: Collaborator[] = [
    { name: "Ana", lightColor: "#2563eb", darkColor: "#60a5fa" },      // Blue
    { name: "Carlos", lightColor: "#059669", darkColor: "#34d399" },   // Emerald
    { name: "Maria", lightColor: "#ea580c", darkColor: "#fb923c" },    // Orange
    { name: "João", lightColor: "#7c3aed", darkColor: "#a78bfa" },     // Violet
    { name: "Luiza", lightColor: "#db2777", darkColor: "#f472b6" },    // Pink
];

const phrases = [
    "simples e eficiente.",
    "sem complicações.",
    "em tempo real.",
    "para todos.",
    "instantânea.",
    "colaboração fácil.",
    "compartilhamento rápido.",
    "edição simultânea.",
];

function getRandomCollaborator(excludeIndex?: number): number {
    let index: number;
    do {
        index = Math.floor(Math.random() * collaborators.length);
    } while (index === excludeIndex);
    return index;
}

type ActionPhase = "typing" | "pausing" | "deleting" | "waiting";

export function HeroTypingAnimation() {
    const [displayText, setDisplayText] = useState("");
    const [phraseIndex, setPhraseIndex] = useState(0);
    const [phase, setPhase] = useState<ActionPhase>("typing");
    const [showCursor, setShowCursor] = useState(true);
    const [activeCollaboratorIndex, setActiveCollaboratorIndex] = useState(0);
    const lastTypingCollaborator = useRef<number>(0);
    const [isDarkMode, setIsDarkMode] = useState(false);

    // Inicializar com valor aleatório apenas no cliente
    useEffect(() => {
        const randomIndex = getRandomCollaborator();
        setActiveCollaboratorIndex(randomIndex);
        lastTypingCollaborator.current = randomIndex;
    }, []);

    // Detectar tema do sistema
    useEffect(() => {
        const mediaQuery = window.matchMedia("(prefers-color-scheme: dark)");
        setIsDarkMode(mediaQuery.matches);

        const handler = (e: MediaQueryListEvent) => setIsDarkMode(e.matches);
        mediaQuery.addEventListener("change", handler);
        return () => mediaQuery.removeEventListener("change", handler);
    }, []);

    // Helper para obter cor correta baseada no tema
    const getColor = useCallback((collaborator: Collaborator) => {
        return isDarkMode ? collaborator.darkColor : collaborator.lightColor;
    }, [isDarkMode]);

    const currentPhrase = phrases[phraseIndex];
    const activeCollaborator = collaborators[activeCollaboratorIndex];
    const activeColor = getColor(activeCollaborator);

    // Cursor blink
    useEffect(() => {
        const cursorInterval = setInterval(() => {
            setShowCursor(prev => !prev);
        }, 530);
        return () => clearInterval(cursorInterval);
    }, []);

    useEffect(() => {
        let timeout: NodeJS.Timeout;

        switch (phase) {
            case "typing": {
                if (displayText.length < currentPhrase.length) {
                    const delay = Math.random() * 100 + 60;
                    timeout = setTimeout(() => {
                        setDisplayText(currentPhrase.slice(0, displayText.length + 1));
                    }, delay);
                } else {
                    // Finished typing, pause
                    lastTypingCollaborator.current = activeCollaboratorIndex;
                    setPhase("pausing");
                }
                break;
            }

            case "pausing": {
                // Wait a bit before someone starts deleting
                timeout = setTimeout(() => {
                    // Random collaborator (different from the one who typed) starts deleting
                    const newCollaborator = getRandomCollaborator(lastTypingCollaborator.current);
                    setActiveCollaboratorIndex(newCollaborator);
                    setPhase("deleting");
                }, 2000 + Math.random() * 1000);
                break;
            }

            case "deleting": {
                if (displayText.length > 0) {
                    const delay = 35 + Math.random() * 25;
                    timeout = setTimeout(() => {
                        setDisplayText(displayText.slice(0, -1));
                    }, delay);
                } else {
                    // Finished deleting, wait before next phrase
                    setPhase("waiting");
                }
                break;
            }

            case "waiting": {
                // Short pause, then new collaborator starts typing new phrase
                timeout = setTimeout(() => {
                    // Pick next phrase
                    const nextPhraseIndex = (phraseIndex + 1) % phrases.length;
                    setPhraseIndex(nextPhraseIndex);

                    // Random new collaborator starts typing (can be anyone)
                    const newCollaborator = getRandomCollaborator(activeCollaboratorIndex);
                    setActiveCollaboratorIndex(newCollaborator);
                    setPhase("typing");
                }, 400 + Math.random() * 300);
                break;
            }
        }

        return () => clearTimeout(timeout);
    }, [displayText, phase, currentPhrase, phraseIndex, activeCollaboratorIndex]);

    return (
        <span className="relative inline-flex min-h-[1.2em] items-baseline">
            {/* Collaborator avatar/indicator - fixed position relative to text baseline */}
            <span className="absolute -left-5 top-[0.55em] -translate-y-1/2 flex items-center gap-1.5 transition-all duration-300 sm:-left-8">
                <span
                    className="w-2.5 h-2.5 rounded-full animate-pulse"
                    style={{ backgroundColor: activeColor }}
                />
            </span>

            {/* Text being typed - with invisible placeholder to maintain width */}
            <span
                className="transition-colors duration-200"
                style={{ color: activeColor }}
            >
                {displayText || "\u200B"}
            </span>

            {/* Cursor with floating badge */}
            <span className="relative inline-flex items-center self-stretch">
                {/* Cursor */}
                <span
                    className="inline-block w-[3px] h-[0.85em] ml-0.5 rounded-full transition-all duration-150 self-center"
                    style={{
                        backgroundColor: activeColor,
                        opacity: showCursor ? 1 : 0,
                    }}
                />

                {/* Floating collaborator name badge - follows cursor */}
                <span
                    className="absolute -top-6 left-1/2 hidden -translate-x-1/2 whitespace-nowrap rounded px-2 py-0.5 text-xs font-medium text-white shadow-sm transition-all duration-200 backdrop-blur-sm sm:inline-flex"
                    style={{ backgroundColor: `${activeColor}cc` }}
                    key={activeCollaboratorIndex}
                >
                    {activeCollaborator.name}
                    <span className="ml-1 opacity-70">
                        {phase === "deleting" ? "⌫" : phase === "typing" ? "✎" : ""}
                    </span>
                    {/* Small arrow pointing down */}
                    <span
                        className="absolute top-full left-1/2 -translate-x-1/2 w-0 h-0 border-l-4 border-r-4 border-t-4 border-l-transparent border-r-transparent"
                        style={{ borderTopColor: `${activeColor}cc` }}
                    />
                </span>
            </span>
        </span>
    );
}
