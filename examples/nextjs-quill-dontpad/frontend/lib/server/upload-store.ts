import { randomUUID } from "node:crypto"
import { mkdir, readFile, stat, writeFile } from "node:fs/promises"
import path from "node:path"

import { normalizePadSlug } from "@/lib/pad-slug"

const uploadRoot = path.join(process.cwd(), ".uploads")
const maxUploadBytes = 12 * 1024 * 1024

export type StoredUpload = {
  id: string
  name: string
  size: number
  contentType: string
  url: string
  uploadedAt: string
}

export async function saveUploadForPad(slug: string, file: File) {
  const normalizedSlug = normalizePadSlug(slug)
  if (file.size <= 0) {
    throw new Error("arquivo vazio")
  }
  if (file.size > maxUploadBytes) {
    throw new Error("arquivo excede o limite de 12 MB")
  }

  const fileName = sanitizeFileName(file.name)
  const storedName = buildStoredName(fileName)
  const targetDir = path.join(uploadRoot, normalizedSlug)
  const targetFile = path.join(targetDir, storedName)

  await mkdir(targetDir, { recursive: true })
  await writeFile(targetFile, Buffer.from(await file.arrayBuffer()))

  return {
    id: storedName,
    name: file.name,
    size: file.size,
    contentType: file.type || "application/octet-stream",
    url: `/api/uploads/${encodeURIComponent(normalizedSlug)}/${encodeURIComponent(storedName)}`,
    uploadedAt: new Date().toISOString(),
  } satisfies StoredUpload
}

export async function readUploadForPad(slug: string, storedName: string) {
  const normalizedSlug = normalizePadSlug(slug)
  const safeStoredName = sanitizeStoredName(storedName)
  const filePath = path.join(uploadRoot, normalizedSlug, safeStoredName)
  const file = await readFile(filePath)
  const fileInfo = await stat(filePath)

  return {
    file,
    size: fileInfo.size,
    downloadName: extractOriginalName(safeStoredName),
  }
}

function buildStoredName(fileName: string) {
  const parsed = path.parse(fileName)
  const safeBase = sanitizeBaseName(parsed.name)
  const safeExt = sanitizeExtension(parsed.ext)
  return `${Date.now()}-${randomUUID()}-${safeBase}${safeExt}`
}

function sanitizeFileName(fileName: string) {
  const trimmed = fileName.trim()
  if (trimmed === "") {
    return "arquivo.bin"
  }
  return trimmed
}

function sanitizeBaseName(value: string) {
  const sanitized = value
    .normalize("NFD")
    .replace(/\p{Diacritic}/gu, "")
    .replace(/[^a-zA-Z0-9_-]+/g, "-")
    .replace(/-+/g, "-")
    .replace(/^[-_]+|[-_]+$/g, "")

  return sanitized || "arquivo"
}

function sanitizeExtension(value: string) {
  return value.toLowerCase().replace(/[^a-z0-9.]/g, "").slice(0, 12)
}

function sanitizeStoredName(value: string) {
  const trimmed = path.basename(value).trim()
  if (trimmed === "" || trimmed.includes("..")) {
    throw new Error("nome de arquivo invalido")
  }
  return trimmed.replace(/[^a-zA-Z0-9._-]+/g, "-")
}

function extractOriginalName(storedName: string) {
  const segments = storedName.split("-")
  if (segments.length < 3) {
    return storedName
  }
  return segments.slice(2).join("-")
}
