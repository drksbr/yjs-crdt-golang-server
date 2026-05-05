"use client";

import { useState, useEffect } from "react";

interface LiveStat {
    value: number;
    suffix: string;
    text: string;
}

const liveStats: LiveStat[] = [
    { value: 185, suffix: "k", text: "documentos abertos neste momento" },
    { value: 212, suffix: "k", text: "pessoas colaborando em tempo real" },
    { value: 47, suffix: "k", text: "edições sendo feitas agora" },
    { value: 89, suffix: "k", text: "equipes trabalhando juntas" },
    { value: 156, suffix: "k", text: "caracteres digitados por minuto" },
];

function generateVariation(baseValue: number): number {
    const variation = baseValue * 0.08; // 8% variation
    return Math.round(baseValue + (Math.random() * variation * 2) - variation);
}

export function LiveStatsIndicator() {
    const [currentIndex, setCurrentIndex] = useState(0);
    const [displayValue, setDisplayValue] = useState(liveStats[0].value);
    const [isTransitioning, setIsTransitioning] = useState(false);

    const currentStat = liveStats[currentIndex];

    // Vary the number slightly every few seconds
    useEffect(() => {
        const interval = setInterval(() => {
            setDisplayValue(generateVariation(currentStat.value));
        }, 2000);

        return () => clearInterval(interval);
    }, [currentStat.value]);

    // Switch between stats
    useEffect(() => {
        const interval = setInterval(() => {
            setIsTransitioning(true);

            setTimeout(() => {
                setCurrentIndex((prev) => (prev + 1) % liveStats.length);
                setIsTransitioning(false);
            }, 300);
        }, 4000);

        return () => clearInterval(interval);
    }, []);

    // Update display value when stat changes
    useEffect(() => {
        setDisplayValue(generateVariation(currentStat.value));
    }, [currentStat.value]);

    return (
        <div className="mt-8 flex w-full max-w-2xl items-start gap-3 rounded-2xl border border-slate-200 bg-white/75 px-4 py-3 text-sm font-medium text-slate-600 shadow-sm dark:border-slate-700 dark:bg-slate-900/70 dark:text-slate-400 sm:inline-flex sm:w-auto sm:max-w-none sm:items-center sm:rounded-full sm:py-2">
            <span className="relative flex h-2.5 w-2.5">
                <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
                <span className="relative inline-flex rounded-full h-2.5 w-2.5 bg-emerald-500"></span>
            </span>
            <span
                className={`transition-all duration-300 ${isTransitioning ? 'opacity-0 translate-y-2' : 'opacity-100 translate-y-0'}`}
            >
                <span className="font-semibold text-slate-900 dark:text-slate-100 tabular-nums">
                    {displayValue.toLocaleString('pt-BR')}{currentStat.suffix}
                </span>
                {" "}
                <span className="text-slate-500 dark:text-slate-400">
                    {currentStat.text}
                </span>
            </span>
        </div>
    );
}
