import { startTransition, useDeferredValue, useEffect, useRef, useState } from 'react'
import { BridgeProvider, type PresenceSnapshot, type ProviderStatus, type UserSelection } from './collab/bridgeProvider'

const defaultRoom = 'writers-room'
const palette = ['#ef8354', '#0f8b8d', '#556cd6', '#bc6c25', '#2a9d8f', '#ff006e']

function App() {
  const [name, setName] = useState(() => loadStoredValue('signal-draft:name', createGuestName()))
  const [color, setColor] = useState(() => loadStoredValue('signal-draft:color', randomColor()))
  const [room, setRoom] = useState(() => readInitialRoom())
  const [roomDraft, setRoomDraft] = useState(() => readInitialRoom())
  const [text, setText] = useState('')
  const [status, setStatus] = useState<ProviderStatus>('connecting')
  const [presence, setPresence] = useState<PresenceSnapshot[]>([])
  const [selection, setSelection] = useState<UserSelection | null>(null)
  const [copied, setCopied] = useState(false)
  const [lastActivity, setLastActivity] = useState('aguardando primeira sincronizacao')

  const providerRef = useRef<BridgeProvider | null>(null)
  const textValueRef = useRef('')
  const deferredPresence = useDeferredValue(presence)

  useEffect(() => {
    window.localStorage.setItem('signal-draft:name', name)
  }, [name])

  useEffect(() => {
    window.localStorage.setItem('signal-draft:color', color)
  }, [color])

  useEffect(() => {
    window.localStorage.setItem('signal-draft:room', room)
    const url = new URL(window.location.href)
    url.searchParams.set('room', room)
    window.history.replaceState({}, '', url)
  }, [room])

  useEffect(() => {
    const provider = new BridgeProvider({
      room,
      profile: { name, color },
      wsPath: '/ws',
      persistOnClose: true,
    })
    providerRef.current = provider

    const sharedText = provider.getText()
    const handleTextChange = () => {
      const nextText = sharedText.toString()
      textValueRef.current = nextText
      startTransition(() => {
        setText(nextText)
        setLastActivity(formatTimestamp(new Date()))
      })
    }
    const handlePresenceChange = () => {
      startTransition(() => {
        setPresence(provider.getPresence())
      })
    }

    const unsubscribeStatus = provider.subscribeStatus((nextStatus) => {
      startTransition(() => {
        setStatus(nextStatus)
      })
    })

    sharedText.observe(handleTextChange)
    provider.awareness.on('change', handlePresenceChange)
    provider.connect()

    textValueRef.current = sharedText.toString()
    setText(textValueRef.current)
    setPresence(provider.getPresence())

    return () => {
      unsubscribeStatus()
      sharedText.unobserve(handleTextChange)
      provider.awareness.off('change', handlePresenceChange)
      provider.destroy()
      if (providerRef.current === provider) {
        providerRef.current = null
      }
    }
  }, [room])

  useEffect(() => {
    providerRef.current?.setProfile({ name, color })
  }, [name, color])

  const handleTextareaChange = (event: React.ChangeEvent<HTMLTextAreaElement>) => {
    const provider = providerRef.current
    if (!provider) {
      return
    }

    const nextText = event.currentTarget.value
    const previousText = textValueRef.current
    if (nextText === previousText) {
      return
    }

    provider.applyTextChange(previousText, nextText)
    textValueRef.current = nextText
    setText(nextText)
    setLastActivity(formatTimestamp(new Date()))
  }

  const handleSelectionChange = (event: React.SyntheticEvent<HTMLTextAreaElement>) => {
    const nextSelection = {
      start: event.currentTarget.selectionStart ?? 0,
      end: event.currentTarget.selectionEnd ?? 0,
    }
    setSelection(nextSelection)
    providerRef.current?.setSelection(nextSelection)
  }

  const handleSelectionClear = () => {
    setSelection(null)
    providerRef.current?.setSelection(null)
  }

  const handleRoomSubmit = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const normalized = normalizeRoom(roomDraft)
    setRoomDraft(normalized)
    setRoom(normalized)
  }

  const handleCopyRoom = async () => {
    const shareURL = new URL(window.location.href)
    shareURL.searchParams.set('room', room)
    await navigator.clipboard.writeText(shareURL.toString())
    setCopied(true)
    window.setTimeout(() => setCopied(false), 1600)
  }

  const charCount = text.length
  const lineCount = text === '' ? 1 : text.split('\n').length

  return (
    <main className="app-shell min-h-screen px-4 py-5 md:px-8 md:py-8">
      <div className="signal-panel mx-auto flex min-h-[calc(100vh-2.5rem)] w-full max-w-7xl flex-col overflow-hidden rounded-[2rem]">
        <div className="grain-line flex flex-col gap-4 px-6 py-5 md:flex-row md:items-end md:justify-between md:px-8">
          <div className="max-w-2xl space-y-3">
            <div className="font-mono-ui text-xs uppercase tracking-[0.32em] text-[var(--accent)]">
              Signal Draft
            </div>
            <h1 className="font-display text-4xl font-semibold tracking-[-0.06em] text-[var(--ink)] md:text-6xl">
              Texto compartilhado com sync em tempo real, awareness e persistência em Postgres.
            </h1>
            <p className="max-w-xl text-sm leading-6 text-[var(--muted)] md:text-base">
              Este demo usa Vite + React + Tailwind no frontend e Gin + yjs-go-bridge + PostgreSQL no backend.
            </p>
          </div>
          <div className="flex items-center gap-3 rounded-full border border-[var(--line)] bg-[var(--surface-strong)] px-4 py-3 font-mono-ui text-xs uppercase tracking-[0.24em] text-[var(--muted)]">
            <span className="status-dot" data-state={status} />
            {renderStatus(status)}
          </div>
        </div>

        <div className="grid flex-1 gap-0 lg:grid-cols-[320px_minmax(0,1fr)]">
          <aside className="grain-line flex flex-col gap-6 px-6 py-6 md:px-8">
            <form className="space-y-4" onSubmit={handleRoomSubmit}>
              <label className="block space-y-2">
                <span className="font-mono-ui text-xs uppercase tracking-[0.24em] text-[var(--muted)]">
                  Room
                </span>
                <div className="flex gap-2">
                  <input
                    value={roomDraft}
                    onChange={(event) => setRoomDraft(event.currentTarget.value)}
                    className="w-full rounded-2xl border border-[var(--line)] bg-white/70 px-4 py-3 text-sm outline-none transition focus:border-[var(--accent)]"
                  />
                  <button
                    type="submit"
                    className="rounded-2xl bg-[var(--ink)] px-4 py-3 text-sm font-medium text-white transition hover:bg-[#24304c]"
                  >
                    Entrar
                  </button>
                </div>
              </label>

              <label className="block space-y-2">
                <span className="font-mono-ui text-xs uppercase tracking-[0.24em] text-[var(--muted)]">
                  Seu nome
                </span>
                <input
                  value={name}
                  onChange={(event) => setName(event.currentTarget.value)}
                  className="w-full rounded-2xl border border-[var(--line)] bg-white/70 px-4 py-3 text-sm outline-none transition focus:border-[var(--accent)]"
                />
              </label>

              <div className="space-y-2">
                <span className="font-mono-ui text-xs uppercase tracking-[0.24em] text-[var(--muted)]">
                  Cor da presença
                </span>
                <div className="flex flex-wrap gap-2">
                  {palette.map((tone) => (
                    <button
                      key={tone}
                      type="button"
                      onClick={() => setColor(tone)}
                      className="h-10 w-10 rounded-full border-2 transition"
                      style={{
                        backgroundColor: tone,
                        borderColor: tone === color ? 'var(--ink)' : 'rgba(23, 32, 51, 0.12)',
                      }}
                      aria-label={`Selecionar cor ${tone}`}
                    />
                  ))}
                </div>
              </div>
            </form>

            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <h2 className="font-display text-lg font-semibold text-[var(--ink)]">
                  Awareness
                </h2>
                <span className="font-mono-ui text-xs uppercase tracking-[0.24em] text-[var(--muted)]">
                  {deferredPresence.length} online
                </span>
              </div>
              <div className="space-y-2">
                {deferredPresence.map((person) => (
                  <div key={person.clientID} className="presence-row flex items-start gap-3 rounded-2xl px-3 py-3">
                    <span className="mt-1 h-3 w-3 rounded-full" style={{ backgroundColor: person.color }} />
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
                        <span className="font-display text-sm font-medium text-[var(--ink)]">
                          {person.name}
                        </span>
                        {person.isLocal ? (
                          <span className="rounded-full bg-[var(--accent-soft)] px-2 py-1 font-mono-ui text-[10px] uppercase tracking-[0.2em] text-[var(--accent)]">
                            voce
                          </span>
                        ) : null}
                      </div>
                      <p className="font-mono-ui text-xs uppercase tracking-[0.18em] text-[var(--muted)]">
                        {formatPresenceSelection(person.selection)}
                      </p>
                    </div>
                  </div>
                ))}
              </div>
            </div>

            <div className="mt-auto space-y-2 rounded-[1.6rem] border border-[var(--line)] bg-white/55 p-4">
              <div className="font-mono-ui text-xs uppercase tracking-[0.24em] text-[var(--muted)]">
                Estado local
              </div>
              <p className="text-sm text-[var(--ink)]">Última atividade: {lastActivity}</p>
              <p className="text-sm text-[var(--ink)]">
                Sua seleção: {formatPresenceSelection(selection)}
              </p>
              <button
                type="button"
                onClick={handleCopyRoom}
                className="mt-2 rounded-full border border-[var(--line)] px-3 py-2 font-mono-ui text-xs uppercase tracking-[0.22em] text-[var(--ink)] transition hover:border-[var(--accent)] hover:text-[var(--accent)]"
              >
                {copied ? 'Link copiado' : 'Copiar sala'}
              </button>
            </div>
          </aside>

          <section className="flex flex-col gap-5 px-6 py-6 md:px-8">
            <div className="grid gap-4 md:grid-cols-3">
              <Metric label="Caracteres" value={charCount.toString()} />
              <Metric label="Linhas" value={lineCount.toString()} />
              <Metric label="Documento" value={room} />
            </div>

            <div className="signal-editor flex flex-1 flex-col rounded-[1.8rem] p-4 md:p-6">
              <div className="flex flex-wrap items-center justify-between gap-3 border-b border-[var(--line)] pb-4">
                <div>
                  <div className="font-mono-ui text-xs uppercase tracking-[0.24em] text-[var(--muted)]">
                    Editor compartilhado
                  </div>
                  <p className="mt-2 max-w-2xl text-sm leading-6 text-[var(--muted)]">
                    Escreva em duas abas ao mesmo tempo para ver o merge do `Y.Text` e a presença ao vivo no painel lateral.
                  </p>
                </div>
                <div className="rounded-full bg-[var(--accent-soft)] px-4 py-2 font-mono-ui text-xs uppercase tracking-[0.24em] text-[var(--accent)]">
                  persistência no close
                </div>
              </div>

              <textarea
                value={text}
                onChange={handleTextareaChange}
                onSelect={handleSelectionChange}
                onKeyUp={handleSelectionChange}
                onClick={handleSelectionChange}
                onBlur={handleSelectionClear}
                placeholder="Comece um rascunho compartilhado aqui..."
                className="mt-4 min-h-[420px] flex-1 bg-transparent font-mono-ui text-[15px] leading-7 text-[var(--ink)] outline-none md:min-h-[520px]"
              />
            </div>
          </section>
        </div>
      </div>
    </main>
  )
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-[1.4rem] border border-[var(--line)] bg-white/55 px-4 py-4">
      <div className="font-mono-ui text-[11px] uppercase tracking-[0.24em] text-[var(--muted)]">
        {label}
      </div>
      <div className="mt-3 font-display text-2xl font-semibold tracking-[-0.05em] text-[var(--ink)]">
        {value}
      </div>
    </div>
  )
}

function renderStatus(status: ProviderStatus) {
  switch (status) {
    case 'connected':
      return 'Conectado'
    case 'connecting':
      return 'Conectando'
    case 'disconnected':
      return 'Desconectado'
    default:
      return 'Erro'
  }
}

function formatPresenceSelection(selection: UserSelection | null) {
  if (!selection) {
    return 'sem seleção ativa'
  }
  return `${selection.start} → ${selection.end}`
}

function formatTimestamp(date: Date) {
  return new Intl.DateTimeFormat('pt-BR', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  }).format(date)
}

function readInitialRoom() {
  if (typeof window === 'undefined') {
    return defaultRoom
  }
  const fromURL = new URL(window.location.href).searchParams.get('room')
  if (fromURL) {
    return normalizeRoom(fromURL)
  }
  return loadStoredValue('signal-draft:room', defaultRoom)
}

function normalizeRoom(value: string) {
  const normalized = value.trim().toLowerCase().replace(/\s+/g, '-')
  return normalized === '' ? defaultRoom : normalized
}

function loadStoredValue(key: string, fallback: string) {
  if (typeof window === 'undefined') {
    return fallback
  }
  const value = window.localStorage.getItem(key)
  return value && value.trim() !== '' ? value : fallback
}

function createGuestName() {
  return `writer-${Math.floor(Math.random() * 900 + 100)}`
}

function randomColor() {
  return palette[Math.floor(Math.random() * palette.length)]
}

export default App
