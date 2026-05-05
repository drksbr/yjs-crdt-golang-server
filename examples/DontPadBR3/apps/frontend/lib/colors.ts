const COLLABORATOR_COLORS = [
  { light: "#2563eb", dark: "#60a5fa" }, // Blue
  { light: "#059669", dark: "#34d399" }, // Emerald
  { light: "#7c3aed", dark: "#a78bfa" }, // Violet
  { light: "#db2777", dark: "#f472b6" }, // Pink
  { light: "#ea580c", dark: "#fb923c" }, // Orange
  { light: "#0891b2", dark: "#22d3ee" }, // Cyan
  { light: "#4f46e5", dark: "#818cf8" }, // Indigo
  { light: "#be123c", dark: "#fb7185" }, // Rose
  { light: "#0d9488", dark: "#2dd4bf" }, // Teal
  { light: "#9333ea", dark: "#c084fc" }, // Purple
];

const ANIMALS = [
  "Leão",
  "Tigre",
  "Urso",
  "Raposa",
  "Lobo",
  "Águia",
  "Golfinho",
  "Panda",
];
const ADJECTIVES = [
  "Azul",
  "Verde",
  "Vermelho",
  "Roxo",
  "Laranja",
  "Rosa",
  "Dourado",
  "Prateado",
];

function stableHash(seed: string): number {
  let hash = 0;
  for (let i = 0; i < seed.length; i++) {
    hash = (hash << 5) - hash + seed.charCodeAt(i);
    hash |= 0;
  }
  return Math.abs(hash);
}

export function getCollaboratorColorFromSeed(
  seed: string,
  isDarkMode: boolean = false,
): string {
  const pair =
    COLLABORATOR_COLORS[stableHash(seed) % COLLABORATOR_COLORS.length];
  return isDarkMode ? pair.dark : pair.light;
}

export function generateCollaboratorName(seed: string): string {
  const h = stableHash(seed);
  return `${ANIMALS[h % ANIMALS.length]} ${ADJECTIVES[Math.floor(h / ANIMALS.length) % ADJECTIVES.length]}`;
}

/**
 * Sanitizes a document ID for use with Y-Sweet
 * Y-Sweet only accepts alphanumeric characters and hyphens
 */
export function sanitizeDocumentId(id: string): string {
  return id
    .toLowerCase()
    .trim()
    .replace(/[^\w\s-]/g, "") // Remove special characters
    .replace(/\s+/g, "-") // Replace spaces with hyphens
    .replace(/-+/g, "-") // Replace multiple hyphens with single hyphen
    .replace(/^-+|-+$/g, ""); // Remove leading/trailing hyphens
}
