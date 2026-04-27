"use client"

import type Quill from "quill"
import { LoaderCircle } from "lucide-react"
import { useEffect, useRef, useState } from "react"

import {
  BridgeProvider,
  type PresenceSnapshot,
} from "@/lib/collab/bridge-provider"
import { cn } from "@/lib/utils"

type QuillRange = {
  index: number
  length: number
}

export function QuillCollabEditor({
  provider,
  presence,
  className,
}: {
  provider: BridgeProvider
  presence: PresenceSnapshot[]
  className?: string
}) {
  const shellRef = useRef<HTMLDivElement | null>(null)
  const mountRef = useRef<HTMLDivElement | null>(null)
  const quillRef = useRef<Quill | null>(null)
  const lastPointerSentAtRef = useRef(0)
  const [ready, setReady] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!mountRef.current) {
      return
    }

    let disposed = false
    let binding: { destroy?: () => void } | undefined
    let selectionHandler: ((range: QuillRange | null) => void) | undefined

    const mount = mountRef.current
    setReady(false)
    setError(null)

    async function setup() {
      try {
        const [
          { default: QuillCtor },
          { default: QuillCursors },
          { QuillBinding },
        ] = await Promise.all([
          import("quill"),
          import("quill-cursors"),
          import("y-quill"),
        ])

        if (disposed) {
          return
        }

        const quillWithImports = QuillCtor as typeof QuillCtor & {
          imports?: Record<string, unknown>
        }
        if (!quillWithImports.imports?.["modules/cursors"]) {
          QuillCtor.register("modules/cursors", QuillCursors)
        }

        mount.innerHTML = ""
        const editorNode = document.createElement("div")
        mount.appendChild(editorNode)

        const quill = new QuillCtor(editorNode, {
          theme: "snow",
          placeholder:
            "Escreva livremente. Quem abrir este mesmo endereco vai cair dentro da mesma nota.",
          modules: {
            cursors: {
              transformOnTextChange: true,
            },
            history: {
              userOnly: true,
            },
            toolbar: [
              [{ header: [1, 2, 3, false] }],
              ["bold", "italic", "underline", "strike"],
              [{ list: "ordered" }, { list: "bullet" }],
              ["blockquote", "code-block", "link"],
              ["clean"],
            ],
          },
        })

        selectionHandler = (range) => {
          provider.setSelection(
            range
              ? {
                  start: range.index,
                  end: range.index + range.length,
                }
              : null,
          )
        }

        quill.on("selection-change", selectionHandler)
        quillRef.current = quill
        binding = new QuillBinding(provider.getText(), quill, provider.awareness)
        setReady(true)
      } catch (setupError) {
        setError(
          setupError instanceof Error
            ? setupError.message
            : "falha ao preparar o editor",
        )
      }
    }

    void setup()

    return () => {
      disposed = true
      provider.setPointer(null)
      provider.setSelection(null)
      if (quillRef.current && selectionHandler) {
        quillRef.current.off("selection-change", selectionHandler)
      }
      binding?.destroy?.()
      quillRef.current = null
      mount.innerHTML = ""
    }
  }, [provider])

  const remotePointers = presence.filter(
    (person) => !person.isLocal && person.pointer !== null,
  )

  return (
    <div
      ref={shellRef}
      className={cn(
        "paper-panel relative overflow-hidden rounded-[1.8rem] border border-white/75 bg-[rgba(255,251,245,0.82)]",
        className,
      )}
      onPointerLeave={() => provider.setPointer(null)}
      onPointerMove={(event) => {
        const now = performance.now()
        if (now - lastPointerSentAtRef.current < 40) {
          return
        }
        if (!shellRef.current) {
          return
        }

        const rect = shellRef.current.getBoundingClientRect()
        if (rect.width <= 0 || rect.height <= 0) {
          return
        }

        lastPointerSentAtRef.current = now
        provider.setPointer({
          x: clamp((event.clientX - rect.left) / rect.width),
          y: clamp((event.clientY - rect.top) / rect.height),
        })
      }}
    >
      <div className="flex items-center justify-between border-b border-border/70 px-4 py-3">
        <div>
          <p className="font-mono text-[11px] uppercase tracking-[0.24em] text-muted-foreground">
            editor colaborativo
          </p>
          <p className="text-sm text-muted-foreground">
            Selecao e cursores remotos entram via awareness.
          </p>
        </div>
        {!ready ? (
          <div className="flex items-center gap-2 font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground">
            <LoaderCircle className="animate-spin" />
            preparando
          </div>
        ) : null}
      </div>

      <div className="relative">
        <div ref={mountRef} className="min-h-[32rem]" />
        {error ? (
          <div className="absolute inset-0 flex items-center justify-center bg-background/90 px-6 text-center text-sm text-destructive">
            {error}
          </div>
        ) : null}
        {remotePointers.map((person) => {
          if (!person.pointer) {
            return null
          }
          return (
            <div
              key={person.clientID}
              className="remote-pointer"
              style={{
                left: `${person.pointer.x * 100}%`,
                top: `${person.pointer.y * 100}%`,
              }}
            >
              <span
                className="remote-pointer-mark"
                style={{ backgroundColor: person.color }}
              />
              <span className="remote-pointer-label">{person.name}</span>
            </div>
          )
        })}
      </div>
    </div>
  )
}

function clamp(value: number) {
  return Math.max(0, Math.min(1, value))
}
