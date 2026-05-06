export function syncSystemThemeClass() {
  if (typeof window === "undefined") return () => undefined;

  const media = window.matchMedia("(prefers-color-scheme: dark)");
  const apply = () => {
    const isDark = media.matches;
    document.documentElement.classList.toggle("dark", isDark);
    document.documentElement.style.colorScheme = isDark ? "dark" : "light";
  };

  apply();
  media.addEventListener("change", apply);
  return () => media.removeEventListener("change", apply);
}
