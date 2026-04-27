"use client"

import {
  ArrowLeft,
  Check,
  Clock3,
  Copy,
  Link2,
  MousePointer2,
  Paperclip,
  Trash2,
  Upload,
  Users2,
} from "lucide-react"
import Link from "next/link"
import {
  startTransition,
  useDeferredValue,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react"

import { QuillCollabEditor } from "@/components/quill-collab-editor"
import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { Badge } from "@/components/ui/badge"
import { Button, buttonVariants } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { ScrollArea } from "@/components/ui/scroll-area"
import {
  BridgeProvider,
  type AttachmentRecord,
  type PresenceSnapshot,
  type ProviderStatus,
} from "@/lib/collab/bridge-provider"
import { labelFromPadSlug } from "@/lib/pad-slug"
import { cn } from "@/lib/utils"

const profileKeyName = "livepad:profile:name"
const profileKeyColor = "livepad:profile:color"

const colorPalette = [
  "#20283f",
  "#bd6b3d",
  "#35607a",
  "#7f4db1",
  "#2f8c5e",
  "#b95b74",
]

const uploadLimitLabel = "12 MB por arquivo"

export function CollaborativePad({ slug }: { slug: string }) {
  const [displayName, setDisplayName] = useState(() =>
    loadStoredValue(profileKeyName, createGuestName()),
  )
  const [color, setColor] = useState(() =>
    loadStoredValue(profileKeyColor, randomColor()),
  )
  const [status, setStatus] = useState<ProviderStatus>("connecting")
  const [presence, setPresence] = useState<PresenceSnapshot[]>([])
  const [attachments, setAttachments] = useState<AttachmentRecord[]>([])
  const [uploading, setUploading] = useState(false)
  const [uploadError, setUploadError] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)
  const copyTimerRef = useRef<number | null>(null)

  const deferredPresence = useDeferredValue(presence)
  const deferredAttachments = useDeferredValue(attachments)

  const provider = useMemo(
    () =>
      new BridgeProvider({
        room: slug,
        profile: {
          name: "Convidado",
          color: colorPalette[0] ?? "#20283f",
        },
        wsURL: resolveBridgeWebSocketURL(),
        persistOnClose: true,
      }),
    [slug],
  )

  useEffect(() => {
    window.localStorage.setItem(profileKeyName, displayName)
  }, [displayName])

  useEffect(() => {
    window.localStorage.setItem(profileKeyColor, color)
  }, [color])

  useEffect(() => {
    return () => {
      if (copyTimerRef.current !== null) {
        window.clearTimeout(copyTimerRef.current)
      }
    }
  }, [])

  useEffect(() => {
    const syncPresenceSnapshot = () => {
      startTransition(() => {
        setPresence(provider.getPresence())
      })
    }

    const syncAttachmentSnapshot = () => {
      startTransition(() => {
        setAttachments(provider.getAttachments())
      })
    }

    const unsubscribeStatus = provider.subscribeStatus((nextStatus) => {
      startTransition(() => {
        setStatus(nextStatus)
      })
    })
    const handlePresenceChange = () => syncPresenceSnapshot()
    const handleAttachmentChange = () => syncAttachmentSnapshot()

    syncPresenceSnapshot()
    syncAttachmentSnapshot()
    provider.awareness.on("change", handlePresenceChange)
    provider.getAttachmentsCollection().observe(handleAttachmentChange)
    provider.connect()

    return () => {
      unsubscribeStatus()
      provider.awareness.off("change", handlePresenceChange)
      provider.getAttachmentsCollection().unobserve(handleAttachmentChange)
      provider.destroy()
    }
  }, [provider])

  useEffect(() => {
    provider.setProfile({
      name: normalizeDisplayName(displayName),
      color,
    })
  }, [provider, displayName, color])

  const localPresence = useMemo(
    () => deferredPresence.find((person) => person.isLocal) ?? null,
    [deferredPresence],
  )

  const handleCopy = async () => {
    await navigator.clipboard.writeText(window.location.href)
    setCopied(true)

    if (copyTimerRef.current !== null) {
      window.clearTimeout(copyTimerRef.current)
    }
    copyTimerRef.current = window.setTimeout(() => {
      setCopied(false)
      copyTimerRef.current = null
    }, 1600)
  }

  const handleUpload = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(event.currentTarget.files ?? [])
    event.currentTarget.value = ""

    if (files.length === 0) {
      return
    }

    setUploadError(null)
    setUploading(true)

    try {
      for (const file of files) {
        const form = new FormData()
        form.set("file", file)

        const response = await fetch(`/api/uploads/${slug}`, {
          method: "POST",
          body: form,
        })
        const payload = (await response.json()) as
          | (AttachmentRecord & { error?: undefined })
          | { error?: string }

        if (!response.ok || !("id" in payload)) {
          throw new Error(payload.error ?? "falha ao enviar arquivo")
        }

        provider.addAttachment({
          ...payload,
          uploadedBy: normalizeDisplayName(displayName),
        })
      }
    } catch (error) {
      setUploadError(
        error instanceof Error ? error.message : "falha ao enviar arquivo",
      )
    } finally {
      setUploading(false)
    }
  }

  return (
    <main className="flex flex-1 flex-col px-4 py-5 md:px-8 md:py-8">
      <div className="pad-shell mx-auto grid w-full max-w-[1600px] gap-4 rounded-[2rem] border border-border/80 bg-card/70 p-4 backdrop-blur xl:grid-cols-[minmax(0,1fr)_360px]">
        <section className="grid gap-4">
          <Card className="rounded-[1.8rem] bg-[rgba(255,251,245,0.72)]">
            <CardHeader className="gap-4 md:grid-cols-[minmax(0,1fr)_auto] md:items-center">
              <div className="space-y-3">
                <div className="flex flex-wrap items-center gap-3">
                  <Link
                    className={cn(
                      buttonVariants({ variant: "ghost", size: "sm" }),
                      "rounded-full bg-background/70",
                    )}
                    href="/"
                  >
                    <ArrowLeft data-icon="inline-start" />
                    Voltar
                  </Link>
                  <Badge variant="secondary">/{slug}</Badge>
                  <Badge variant="outline">{formatStatus(status)}</Badge>
                </div>
                <div className="space-y-1">
                  <CardTitle className="text-3xl tracking-[-0.05em] md:text-4xl">
                    {labelFromPadSlug(slug)}
                  </CardTitle>
                  <CardDescription className="max-w-3xl text-sm leading-6">
                    Compartilhe este endereco e qualquer pessoa cai na mesma nota
                    colaborativa. Texto, cursores, selecoes, ponteiros e anexos
                    entram ao vivo.
                  </CardDescription>
                </div>
              </div>

              <div className="flex flex-col items-stretch gap-2 md:min-w-[260px]">
                <Button
                  className="rounded-[1.2rem] bg-primary"
                  onClick={handleCopy}
                  type="button"
                >
                  {copied ? (
                    <Check data-icon="inline-start" />
                  ) : (
                    <Copy data-icon="inline-start" />
                  )}
                  {copied ? "Link copiado" : "Copiar endereco"}
                </Button>
                <div className="rounded-[1rem] border border-border/70 bg-background/70 px-3 py-2 font-mono text-[11px] leading-5 tracking-[0.15em] text-muted-foreground">
                  /{slug}
                </div>
              </div>
            </CardHeader>
          </Card>

          <QuillCollabEditor provider={provider} presence={deferredPresence} />
        </section>

        <aside className="grid gap-4">
          <Card className="rounded-[1.8rem] bg-[rgba(255,251,245,0.78)]">
            <CardHeader>
              <CardTitle>Seu perfil de edicao</CardTitle>
              <CardDescription>
                O nome e a cor vao para presence, selecao remota e ponteiro.
              </CardDescription>
            </CardHeader>
            <CardContent className="grid gap-4">
              <label className="grid gap-2">
                <span className="font-mono text-[11px] uppercase tracking-[0.22em] text-muted-foreground">
                  nome visivel
                </span>
                <Input
                  value={displayName}
                  onChange={(event) => setDisplayName(event.currentTarget.value)}
                  placeholder="Ana da pauta"
                />
              </label>

              <div className="grid gap-2">
                <span className="font-mono text-[11px] uppercase tracking-[0.22em] text-muted-foreground">
                  cor de presence
                </span>
                <div className="flex flex-wrap gap-2">
                  {colorPalette.map((tone) => (
                    <button
                      key={tone}
                      aria-label={`Selecionar cor ${tone}`}
                      className={cn(
                        "size-9 rounded-full border-2 transition-transform hover:scale-105",
                        tone === color
                          ? "border-foreground shadow-[0_0_0_4px_rgba(32,40,63,0.08)]"
                          : "border-white",
                      )}
                      onClick={() => setColor(tone)}
                      style={{ backgroundColor: tone }}
                      type="button"
                    />
                  ))}
                </div>
              </div>

              <div className="rounded-[1.2rem] border border-border/70 bg-background/70 p-3">
                <div className="flex items-center gap-3">
                  <Avatar size="lg">
                    <AvatarFallback
                      className="font-mono uppercase"
                      style={{ backgroundColor: color, color: "#fffaf2" }}
                    >
                      {initialsFromName(displayName)}
                    </AvatarFallback>
                  </Avatar>
                  <div className="space-y-1">
                    <p className="text-sm font-medium">
                      {normalizeDisplayName(displayName)}
                    </p>
                    <p className="font-mono text-[11px] uppercase tracking-[0.16em] text-muted-foreground">
                      {localPresence?.selection
                        ? `selecao ${formatSelection(localPresence.selection)}`
                        : "sem selecao ativa"}
                    </p>
                  </div>
                </div>
              </div>
            </CardContent>
          </Card>

          <Card className="rounded-[1.8rem] bg-[rgba(255,251,245,0.78)]">
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <Users2 />
                Presenca
              </CardTitle>
              <CardDescription>
                {deferredPresence.length} pessoa(s) nesta rota.
              </CardDescription>
            </CardHeader>
            <CardContent className="px-0">
              <ScrollArea className="max-h-[18rem] px-4">
                <div className="grid gap-2 pb-1">
                  {deferredPresence.map((person) => (
                    <div
                      key={person.clientID}
                      className="rounded-[1.2rem] border border-border/70 bg-background/70 p-3"
                    >
                      <div className="flex items-start gap-3">
                        <Avatar>
                          <AvatarFallback
                            className="font-mono uppercase"
                            style={{
                              backgroundColor: person.color,
                              color: "#fffaf2",
                            }}
                          >
                            {initialsFromName(person.name)}
                          </AvatarFallback>
                        </Avatar>
                        <div className="min-w-0 flex-1 space-y-1">
                          <div className="flex flex-wrap items-center gap-2">
                            <p className="truncate text-sm font-medium">
                              {person.name}
                            </p>
                            {person.isLocal ? (
                              <Badge variant="secondary">voce</Badge>
                            ) : null}
                          </div>
                          <p className="font-mono text-[11px] uppercase tracking-[0.16em] text-muted-foreground">
                            {person.selection
                              ? `selecao ${formatSelection(person.selection)}`
                              : "cursor solto"}
                          </p>
                          <p className="font-mono text-[11px] uppercase tracking-[0.16em] text-muted-foreground">
                            {person.pointer
                              ? `ponteiro ${Math.round(person.pointer.x * 100)} x ${Math.round(person.pointer.y * 100)}`
                              : "ponteiro fora do editor"}
                          </p>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              </ScrollArea>
            </CardContent>
          </Card>

          <Card className="rounded-[1.8rem] bg-[rgba(255,251,245,0.78)]">
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <Paperclip />
                Anexos
              </CardTitle>
              <CardDescription>
                Upload local pelo Next. A lista sincroniza pelo mesmo documento
                Yjs.
              </CardDescription>
            </CardHeader>
            <CardContent className="grid gap-4">
              <label className="grid gap-2">
                <span className="font-mono text-[11px] uppercase tracking-[0.22em] text-muted-foreground">
                  enviar arquivo
                </span>
                <div className="rounded-[1.2rem] border border-dashed border-border bg-background/70 p-3">
                  <div className="flex flex-wrap items-center justify-between gap-3">
                    <div>
                      <p className="text-sm font-medium">
                        Arraste ou escolha arquivos
                      </p>
                      <p className="text-sm text-muted-foreground">
                        Demo local com limite de {uploadLimitLabel}.
                      </p>
                    </div>
                    <div className="relative">
                      <input
                        className="attachment-drop absolute inset-0 h-full w-full cursor-pointer opacity-0"
                        multiple
                        onChange={handleUpload}
                        type="file"
                      />
                      <Button className="rounded-full" size="sm" type="button">
                        <Upload data-icon="inline-start" />
                        {uploading ? "Enviando..." : "Selecionar"}
                      </Button>
                    </div>
                  </div>
                </div>
              </label>

              {uploadError ? (
                <div className="rounded-[1rem] border border-destructive/20 bg-destructive/8 px-3 py-2 text-sm text-destructive">
                  {uploadError}
                </div>
              ) : null}

              <ScrollArea className="max-h-[18rem] rounded-[1.2rem] border border-border/70 bg-background/70">
                <div className="grid gap-2 p-3">
                  {deferredAttachments.length === 0 ? (
                    <div className="rounded-[1rem] border border-dashed border-border/80 px-3 py-5 text-center text-sm text-muted-foreground">
                      Nada anexado ainda.
                    </div>
                  ) : null}

                  {deferredAttachments.map((attachment) => (
                    <div
                      key={attachment.id}
                      className="rounded-[1rem] border border-border/70 bg-white/75 p-3"
                    >
                      <div className="flex items-start justify-between gap-3">
                        <div className="min-w-0 space-y-1">
                          <a
                            className="truncate text-sm font-medium underline-offset-4 hover:underline"
                            href={attachment.url}
                            rel="noreferrer"
                            target="_blank"
                          >
                            {attachment.name}
                          </a>
                          <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                            <span>{formatBytes(attachment.size)}</span>
                            <span>•</span>
                            <span>{attachment.uploadedBy}</span>
                            <span>•</span>
                            <span>{formatUploadedAt(attachment.uploadedAt)}</span>
                          </div>
                        </div>
                        <Button
                          onClick={() => provider.removeAttachment(attachment.id)}
                          size="icon-xs"
                          type="button"
                          variant="ghost"
                        >
                          <Trash2 />
                        </Button>
                      </div>
                    </div>
                  ))}
                </div>
              </ScrollArea>
            </CardContent>
          </Card>

          <Card className="rounded-[1.8rem] bg-primary text-primary-foreground">
            <CardHeader>
              <CardTitle>Como esse demo conversa com o bridge</CardTitle>
              <CardDescription className="text-primary-foreground/70">
                A URL da rota vira `doc`, o `clientID` vem do Y.Doc e a
                awareness carrega nome, selecao e ponteiro.
              </CardDescription>
            </CardHeader>
            <CardContent className="grid gap-3 text-sm text-primary-foreground/88">
              <InfoLine icon={Link2} text={`/${slug} -> ?doc=${slug}`} />
              <InfoLine
                icon={MousePointer2}
                text="Ponteiro remoto e lista lateral usam o mesmo awareness."
              />
              <InfoLine
                icon={Clock3}
                text="Persistencia acontece no fechamento da conexao do bridge."
              />
            </CardContent>
          </Card>
        </aside>
      </div>
    </main>
  )
}

function InfoLine({
  icon: Icon,
  text,
}: {
  icon: typeof Link2
  text: string
}) {
  return (
    <div className="flex items-start gap-2">
      <Icon className="mt-0.5 shrink-0" />
      <p className="leading-6">{text}</p>
    </div>
  )
}

function resolveBridgeWebSocketURL() {
  const explicit = process.env.NEXT_PUBLIC_YJS_WS_URL?.trim()
  if (explicit) {
    return explicit
  }

  if (typeof window === "undefined") {
    return "ws://127.0.0.1:8080/ws"
  }

  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:"
  const host = window.location.hostname

  if (window.location.port === "3000") {
    return `${protocol}//${host}:8080/ws`
  }

  return `${protocol}//${window.location.host}/ws`
}

function formatStatus(status: ProviderStatus) {
  switch (status) {
    case "connected":
      return "conectado"
    case "connecting":
      return "conectando"
    case "disconnected":
      return "reconectando"
    case "error":
      return "erro"
    default:
      return status
  }
}

function formatSelection(selection: { start: number; end: number }) {
  return `${selection.start} -> ${selection.end}`
}

function formatBytes(size: number) {
  if (size < 1024) {
    return `${size} B`
  }
  if (size < 1024 * 1024) {
    return `${(size / 1024).toFixed(1)} KB`
  }
  return `${(size / (1024 * 1024)).toFixed(1)} MB`
}

function formatUploadedAt(value: string) {
  const date = new Date(value)
  return new Intl.DateTimeFormat("pt-BR", {
    hour: "2-digit",
    minute: "2-digit",
    day: "2-digit",
    month: "2-digit",
  }).format(date)
}

function initialsFromName(name: string) {
  const tokens = normalizeDisplayName(name).split(/\s+/g).slice(0, 2)
  return tokens.map((token) => token.charAt(0)).join("")
}

function normalizeDisplayName(value: string) {
  const trimmed = value.trim()
  return trimmed === "" ? "Convidado" : trimmed
}

function createGuestName() {
  return `Convidado ${Math.floor(10 + Math.random() * 90)}`
}

function loadStoredValue(key: string, fallback: string) {
  if (typeof window === "undefined") {
    return fallback
  }
  const value = window.localStorage.getItem(key)
  return value?.trim() ? value : fallback
}

function randomColor() {
  return (
    colorPalette[Math.floor(Math.random() * colorPalette.length)] ??
    colorPalette[0] ??
    "#20283f"
  )
}
