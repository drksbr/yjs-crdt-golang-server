import Script from "next/script";
import { Inter } from "next/font/google";
import { VersionLogger } from "@/components/VersionLogger";

import "./globals.css";

const inter = Inter({
  subsets: ["latin"],
  display: "swap",
});

export const metadata = {
  title: "DontPad 2.0 - Documentos Colaborativos em Tempo Real",
  description: "Crie e compartilhe documentos colaborativos instantaneamente. Edite em tempo real com subdocumentos e sincronização automática.",
  icons: {
    icon: "/favicon.svg",
  },
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html
      lang="pt-BR"
      className={inter.className}
      data-scroll-behavior="smooth"
      suppressHydrationWarning
    >
      <head>
        <Script id="system-theme" strategy="beforeInteractive">
          {`(() => {
            const root = document.documentElement;
            const mediaQuery = window.matchMedia("(prefers-color-scheme: dark)");
            const applyTheme = (isDark) => {
              root.classList.toggle("dark", isDark);
              root.style.colorScheme = isDark ? "dark" : "light";
            };

            applyTheme(mediaQuery.matches);

            const handleChange = (event) => applyTheme(event.matches);
            if (typeof mediaQuery.addEventListener === "function") {
              mediaQuery.addEventListener("change", handleChange);
            } else if (typeof mediaQuery.addListener === "function") {
              mediaQuery.addListener(handleChange);
            }
          })();`}
        </Script>
      </head>
      <body className="bg-white dark:bg-slate-900 text-gray-900 dark:text-slate-100 transition-colors">
        <VersionLogger />
        {children}
      </body>
    </html>
  );
}
