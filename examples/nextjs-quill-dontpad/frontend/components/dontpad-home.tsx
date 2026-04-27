"use client"

import { ArrowRight, Link2, MousePointer2, Paperclip, Users2 } from "lucide-react"
import Link from "next/link"
import { useRouter } from "next/navigation"
import { FormEvent, useState } from "react"

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
import { cn } from "@/lib/utils"
import { normalizePadSlug, randomPadSlug } from "@/lib/pad-slug"

const samplePads = ["roteiro-reuniao", "apresentacao-viva", "sala-dos-autores"]

export function DontpadHome() {
  const router = useRouter()
  const [draft, setDraft] = useState(() => randomPadSlug())

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    router.push(`/${normalizePadSlug(draft)}`)
  }

  return (
    <main className="flex flex-1 flex-col px-4 py-5 md:px-8 md:py-8">
      <section className="pad-shell mx-auto grid w-full max-w-7xl gap-6 rounded-[2rem] border border-border/80 bg-card/70 p-4 backdrop-blur md:grid-cols-[1.3fr_0.9fr] md:p-6">
        <div className="paper-panel rounded-[1.7rem] border border-white/70 bg-[rgba(255,252,246,0.7)] p-6 md:p-8">
          <div className="flex flex-col gap-6">
            <div className="flex items-center gap-3">
              <Badge variant="secondary">Next.js + Yjs + Quill</Badge>
              <Badge variant="outline">demo dontpad colaborativo</Badge>
            </div>
            <div className="space-y-4">
              <p className="font-mono text-xs uppercase tracking-[0.28em] text-muted-foreground">
                compartilhe a URL e edite junto
              </p>
              <h1 className="max-w-3xl text-4xl font-semibold tracking-[-0.06em] text-balance md:text-6xl">
                Abra um slug, escreva agora e veja pessoas, selecoes e ponteiros
                em tempo real.
              </h1>
              <p className="max-w-2xl text-base leading-7 text-muted-foreground md:text-lg">
                Cada rota vira uma nota viva. O texto fica sincronizado via Yjs,
                o editor usa Quill, anexos entram pelo proprio app e a presenca
                aparece sem recarregar nada.
              </p>
            </div>

            <form className="flex flex-col gap-3 md:max-w-2xl" onSubmit={handleSubmit}>
              <label className="font-mono text-xs uppercase tracking-[0.24em] text-muted-foreground">
                dominio / seu_nome_aqui
              </label>
              <div className="grid gap-3 md:grid-cols-[minmax(0,1fr)_auto]">
                <div className="rounded-[1.4rem] border border-border bg-background/70 p-2">
                  <div className="flex items-center gap-3 rounded-[1rem] bg-background px-3 py-2">
                    <span className="font-mono text-sm text-muted-foreground">
                      /
                    </span>
                    <Input
                      value={draft}
                      onChange={(event) => setDraft(event.currentTarget.value)}
                      className="border-none bg-transparent px-0 text-base shadow-none focus-visible:ring-0"
                      placeholder="roteiro-live"
                    />
                  </div>
                </div>
                <Button className="h-auto rounded-[1.35rem] px-5 py-3" size="lg" type="submit">
                  <ArrowRight data-icon="inline-end" />
                  Abrir nota
                </Button>
              </div>
            </form>

            <div className="flex flex-wrap gap-3">
              {samplePads.map((slug) => (
                <Link
                  key={slug}
                  className={cn(
                    buttonVariants({ variant: "outline", size: "sm" }),
                    "rounded-full bg-white/65",
                  )}
                  href={`/${slug}`}
                >
                  <Link2 data-icon="inline-start" />
                  /{slug}
                </Link>
              ))}
            </div>
          </div>
        </div>

        <div className="grid gap-4">
          <Card className="rounded-[1.7rem] bg-[rgba(255,252,246,0.78)]">
            <CardHeader>
              <CardTitle>O que sincroniza</CardTitle>
              <CardDescription>
                Tudo o que importa para um bloco realmente compartilhado.
              </CardDescription>
            </CardHeader>
            <CardContent className="grid gap-3">
              <FeatureRow
                icon={Users2}
                title="Presenca viva"
                copy="Nome, cor, usuarios online e selecao atual."
              />
              <FeatureRow
                icon={MousePointer2}
                title="Ponteiro remoto"
                copy="Movimento no editor aparece para as outras pessoas."
              />
              <FeatureRow
                icon={Paperclip}
                title="Anexos"
                copy="Uploads entram na nota e surgem para todos em tempo real."
              />
            </CardContent>
          </Card>

          <Card className="rounded-[1.7rem] bg-primary text-primary-foreground">
            <CardHeader>
              <CardTitle>Backend esperado</CardTitle>
              <CardDescription className="text-primary-foreground/70">
                WebSocket do bridge em /ws e persistencia em Postgres.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3 font-mono text-xs uppercase tracking-[0.18em] text-primary-foreground/84">
              <p>frontend: http://127.0.0.1:3000</p>
              <p>bridge: ws://127.0.0.1:8080/ws</p>
              <p>slug vira ?doc=&lt;rota&gt;</p>
            </CardContent>
          </Card>
        </div>
      </section>
    </main>
  )
}

function FeatureRow({
  icon: Icon,
  title,
  copy,
}: {
  icon: typeof Users2
  title: string
  copy: string
}) {
  return (
    <div className="flex items-start gap-3 rounded-[1.2rem] border border-border/70 bg-background/70 p-3">
      <div className="rounded-full bg-secondary p-2 text-secondary-foreground">
        <Icon />
      </div>
      <div className="space-y-1">
        <p className="text-sm font-medium">{title}</p>
        <p className="text-sm leading-6 text-muted-foreground">{copy}</p>
      </div>
    </div>
  )
}
