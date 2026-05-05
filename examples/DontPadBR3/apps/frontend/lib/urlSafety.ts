const SAFE_PROTOCOLS = new Set(["http:", "https:", "mailto:", "tel:"]);

export function getSafeUrl(rawUrl: string): string | null {
  const trimmed = rawUrl.trim();
  if (!trimmed || trimmed.startsWith("//")) {
    return null;
  }

  if (trimmed.startsWith("#")) {
    return trimmed;
  }

  if (trimmed.startsWith("/")) {
    return trimmed;
  }

  try {
    const parsed = new URL(trimmed, "https://dontpad.local");
    const isRelative = !/^[a-zA-Z][a-zA-Z\d+\-.]*:/.test(trimmed);

    if (isRelative) {
      return trimmed;
    }

    if (!SAFE_PROTOCOLS.has(parsed.protocol)) {
      return null;
    }

    return parsed.toString();
  } catch {
    return null;
  }
}

export function openSafeUrl(rawUrl: string): boolean {
  const safeUrl = getSafeUrl(rawUrl);
  if (!safeUrl || typeof window === "undefined") {
    return false;
  }

  window.open(safeUrl, "_blank", "noopener,noreferrer");
  return true;
}
