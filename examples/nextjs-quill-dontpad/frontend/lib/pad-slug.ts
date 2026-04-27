const slugJoiner = "-"

const adjectivePool = [
  "caderno",
  "linha",
  "rascunho",
  "papel",
  "memo",
  "bloco",
]

const nounPool = [
  "quente",
  "publico",
  "agora",
  "compartilhado",
  "direto",
  "vivo",
]

export function normalizePadSlug(value: string) {
  const normalized = value
    .trim()
    .toLowerCase()
    .normalize("NFD")
    .replace(/\p{Diacritic}/gu, "")
    .replace(/[^a-z0-9_-]+/g, slugJoiner)
    .replace(/-+/g, slugJoiner)
    .replace(/^[-_]+|[-_]+$/g, "")

  return normalized || randomPadSlug()
}

export function randomPadSlug() {
  const adjective =
    adjectivePool[Math.floor(Math.random() * adjectivePool.length)] ?? "bloco"
  const noun = nounPool[Math.floor(Math.random() * nounPool.length)] ?? "vivo"
  const suffix = Math.floor(100 + Math.random() * 900)
  return [adjective, noun, suffix].join(slugJoiner)
}

export function labelFromPadSlug(slug: string) {
  return slug
    .split(/[-_]+/g)
    .filter(Boolean)
    .map((token) => token.charAt(0).toUpperCase() + token.slice(1))
    .join(" ")
}
