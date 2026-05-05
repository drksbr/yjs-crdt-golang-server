"use client";

import { useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import { sanitizeDocumentId } from "@/lib/colors";

const navItems = [
  { href: "#abrir-nota", label: "Abrir nota" },
  { href: "#recursos", label: "Recursos" },
  { href: "#seguranca", label: "Segurança" },
  { href: "#faq", label: "FAQ" },
];

const quickExamples = [
  "plantao-noturno",
  "reuniao-produto",
  "checklist-cirurgia",
];

const heroStats = [
  { value: "0", label: "cadastros obrigatórios" },
  { value: "1 link", label: "para toda a equipe" },
  { value: "tempo real", label: "em desktop e mobile" },
];

const featureCards = [
  {
    eyebrow: "Editor vivo",
    title: "Texto, checklist, kanban e desenho no mesmo documento",
    description:
      "Comece simples e transforme a nota em uma estrutura de trabalho quando precisar.",
  },
  {
    eyebrow: "Organização",
    title: "Subnotas para separar contexto sem perder o fio principal",
    description:
      "Cada assunto ganha espaço próprio, mas continua conectado ao documento raiz.",
  },
  {
    eyebrow: "Captura",
    title: "Arquivos e áudio junto da conversa operacional",
    description:
      "Registre evidências, anexos e notas de voz sem sair da página da nota.",
  },
];

const capabilityItems = [
  "Edição colaborativa com presença",
  "Checklist hierárquica",
  "Kanban com cor e links",
  "Desenho livre",
  "Subdocumentos pesquisáveis",
  "PIN e modo somente leitura",
];

const comparisonRows = [
  {
    label: "Entrar em uma nota",
    common: "Criar conta, workspace e pasta",
    dontpad: "Digitar um nome e abrir",
  },
  {
    label: "Organizar contexto",
    common: "Vários apps e links soltos",
    dontpad: "Nota, subnotas e anexos juntos",
  },
  {
    label: "Compartilhar com alguém",
    common: "Convites e permissões longas",
    dontpad: "URL curta com controle por PIN",
  },
  {
    label: "Trabalhar no celular",
    common: "Layout apertado e painéis presos",
    dontpad: "Área útil priorizada para escrever",
  },
];

const securityItems = [
  {
    title: "PIN quando precisa controlar acesso",
    description:
      "Notas privadas ou somente leitura mantêm a colaboração simples sem abrir mão de controle.",
  },
  {
    title: "Rollback por versões",
    description:
      "Snapshots manuais e pontos automáticos raros ajudam a voltar no tempo sem sobrecarregar o servidor.",
  },
  {
    title: "Sincronização com estado visível",
    description:
      "O sistema mostra o que importa sem transformar cada tecla em ruído de status.",
  },
];

const faqItems = [
  {
    question: "Preciso criar conta para usar?",
    answer:
      "Não. A proposta é abrir uma nota por nome, compartilhar o link e trabalhar imediatamente.",
  },
  {
    question: "Consigo usar no celular?",
    answer:
      "Sim. A home e os editores foram pensados para ocupar melhor a largura disponível em telas pequenas.",
  },
  {
    question: "Dá para proteger uma nota?",
    answer:
      "Sim. Você pode usar PIN, modo privado ou leitura pública com edição controlada.",
  },
];

function LogoMark() {
  return (
    <div className="flex h-11 w-11 items-center justify-center rounded-2xl bg-slate-950 text-sm font-black tracking-tight text-white shadow-[0_14px_30px_-18px_rgba(15,23,42,0.9)] dark:bg-white dark:text-slate-950">
      DP
    </div>
  );
}

function ArrowIcon() {
  return (
    <svg
      aria-hidden="true"
      className="h-4 w-4"
      viewBox="0 0 20 20"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
    >
      <path
        d="M4.25 10H15.2M10.95 5.75 15.2 10l-4.25 4.25"
        stroke="currentColor"
        strokeWidth="1.8"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

function ProductPreview() {
  return (
    <div className="relative mx-auto mt-12 max-w-5xl">
      <div className="absolute -inset-8 -z-10 rounded-[2.5rem] bg-[radial-gradient(circle_at_50%_0%,rgba(20,184,166,0.18),transparent_55%),radial-gradient(circle_at_80%_30%,rgba(245,158,11,0.16),transparent_45%)] blur-2xl dark:bg-[radial-gradient(circle_at_50%_0%,rgba(45,212,191,0.14),transparent_55%),radial-gradient(circle_at_80%_30%,rgba(251,191,36,0.12),transparent_45%)]" />

      <div className="overflow-hidden rounded-[2rem] border border-slate-200/80 bg-white shadow-[0_30px_90px_-55px_rgba(15,23,42,0.55)] dark:border-white/10 dark:bg-slate-950">
        <div className="flex items-center justify-between border-b border-slate-200 bg-slate-50 px-4 py-3 dark:border-white/10 dark:bg-slate-900">
          <div className="flex items-center gap-2">
            <span className="h-3 w-3 rounded-full bg-red-300" />
            <span className="h-3 w-3 rounded-full bg-amber-300" />
            <span className="h-3 w-3 rounded-full bg-emerald-300" />
          </div>
          <div className="hidden rounded-full border border-slate-200 bg-white px-4 py-1.5 font-mono text-xs text-slate-500 dark:border-white/10 dark:bg-slate-950 dark:text-slate-400 sm:block">
            dontpad.com.br/plantao-noturno
          </div>
          <span className="rounded-full bg-emerald-50 px-3 py-1 text-xs font-semibold text-emerald-700 dark:bg-emerald-400/10 dark:text-emerald-300">
            sincronizado
          </span>
        </div>

        <div className="grid gap-0 lg:grid-cols-[minmax(0,1fr)_20rem]">
          <div className="p-5 sm:p-8">
            <div className="flex flex-wrap items-center gap-2">
              {["principal", "checklist", "áudio", "arquivos"].map((item) => (
                <span
                  key={item}
                  className="rounded-full border border-slate-200 bg-slate-50 px-3 py-1 text-xs font-semibold text-slate-600 dark:border-white/10 dark:bg-slate-900 dark:text-slate-300"
                >
                  {item}
                </span>
              ))}
            </div>

            <div className="mt-8 max-w-2xl">
              <p className="text-xs font-bold uppercase tracking-[0.18em] text-teal-700 dark:text-teal-300">
                nota ativa
              </p>
              <h3 className="mt-3 text-2xl font-semibold tracking-tight text-slate-950 dark:text-white sm:text-3xl">
                Pendências do plantão e próximas decisões
              </h3>
              <div className="mt-6 space-y-3">
                {[
                  "Ajustar prioridades da equipe antes da troca de turno.",
                  "Validar exames pendentes e anexar os arquivos relevantes.",
                  "Registrar áudio curto com contexto para quem assumir depois.",
                ].map((line, index) => (
                  <div
                    key={line}
                    className="flex items-start gap-3 rounded-2xl border border-slate-200 bg-slate-50 px-4 py-3 text-sm leading-6 text-slate-700 dark:border-white/10 dark:bg-slate-900 dark:text-slate-300"
                  >
                    <span className="mt-1 flex h-5 w-5 shrink-0 items-center justify-center rounded-md bg-slate-950 text-[10px] font-bold text-white dark:bg-white dark:text-slate-950">
                      {index + 1}
                    </span>
                    {line}
                  </div>
                ))}
              </div>
            </div>
          </div>

          <div className="border-t border-slate-200 bg-slate-50 p-5 dark:border-white/10 dark:bg-slate-900 lg:border-l lg:border-t-0">
            <div className="rounded-3xl border border-slate-200 bg-white p-4 dark:border-white/10 dark:bg-slate-950">
              <div className="flex items-center justify-between">
                <p className="text-xs font-bold uppercase tracking-[0.16em] text-slate-500 dark:text-slate-400">
                  subnotas
                </p>
                <span className="rounded-full bg-teal-50 px-2.5 py-1 text-xs font-semibold text-teal-700 dark:bg-teal-400/10 dark:text-teal-300">
                  6 itens
                </span>
              </div>
              <div className="mt-4 space-y-2">
                {["Triagem", "Pendências", "Arquivos", "Alta"].map((item) => (
                  <div
                    key={item}
                    className="rounded-2xl border border-slate-200 bg-slate-50 px-3 py-2.5 text-sm font-medium text-slate-700 dark:border-white/10 dark:bg-slate-900 dark:text-slate-300"
                  >
                    {item}
                  </div>
                ))}
              </div>
            </div>

            <div className="mt-4 rounded-3xl border border-slate-200 bg-white p-4 dark:border-white/10 dark:bg-slate-950">
              <p className="text-xs font-bold uppercase tracking-[0.16em] text-slate-500 dark:text-slate-400">
                colaboradores
              </p>
              <div className="mt-4 flex -space-x-2">
                {["IR", "MD", "AC", "LT"].map((person, index) => (
                  <span
                    key={person}
                    className={`flex h-9 w-9 items-center justify-center rounded-full border-2 border-white text-[11px] font-bold text-white dark:border-slate-950 ${
                      index === 0
                        ? "bg-slate-950"
                        : index === 1
                          ? "bg-teal-600"
                          : index === 2
                            ? "bg-amber-500"
                            : "bg-sky-600"
                    }`}
                  >
                    {person}
                  </span>
                ))}
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

export default function HomePage() {
  const [documentName, setDocumentName] = useState("");
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);
  const router = useRouter();

  const handleCreateDocument = (event: FormEvent) => {
    event.preventDefault();
    const sanitized = sanitizeDocumentId(documentName);
    if (!sanitized) return;
    router.push(`/${encodeURIComponent(sanitized)}`);
  };

  const closeMenu = () => setIsMobileMenuOpen(false);

  return (
    <div className="min-h-screen bg-[#f4f1ea] text-slate-950 transition-colors dark:bg-[#070a0f] dark:text-slate-100">
      <div className="pointer-events-none fixed inset-0 -z-10 bg-[radial-gradient(circle_at_20%_0%,rgba(20,184,166,0.16),transparent_28rem),radial-gradient(circle_at_80%_10%,rgba(245,158,11,0.14),transparent_24rem)] dark:bg-[radial-gradient(circle_at_20%_0%,rgba(45,212,191,0.12),transparent_28rem),radial-gradient(circle_at_80%_10%,rgba(251,191,36,0.1),transparent_24rem)]" />

      <header className="sticky top-0 z-30 px-3 py-3">
        <div className="mx-auto flex h-16 w-full max-w-[1180px] items-center justify-between rounded-3xl border border-white/70 bg-white/80 px-4 shadow-[0_18px_50px_-38px_rgba(15,23,42,0.55)] backdrop-blur-xl dark:border-white/10 dark:bg-slate-950/90">
          <a href="#" className="flex min-w-0 items-center gap-3 no-underline" aria-label="DontPad BR">
            <LogoMark />
            <div className="min-w-0">
              <div className="truncate text-base font-black tracking-tight text-slate-950 dark:text-white">
                DontPad BR
              </div>
              <div className="hidden text-xs font-medium text-slate-500 dark:text-slate-400 sm:block">
                notas colaborativas diretas
              </div>
            </div>
          </a>

          <nav className="hidden items-center gap-7 md:flex">
            {navItems.map((item) => (
              <a
                key={item.href}
                href={item.href}
                className="text-sm font-semibold text-slate-600 no-underline transition hover:text-slate-950 dark:text-slate-400 dark:hover:text-white"
              >
                {item.label}
              </a>
            ))}
          </nav>

          <div className="hidden items-center gap-3 md:flex">
            <a
              href="#abrir-nota"
              className="inline-flex items-center gap-2 rounded-2xl bg-slate-950 px-4 py-2.5 text-sm font-bold text-white no-underline transition hover:-translate-y-0.5 hover:bg-slate-800 dark:bg-white dark:text-slate-950 dark:hover:bg-slate-200"
            >
              Começar
              <ArrowIcon />
            </a>
          </div>

          <button
            type="button"
            onClick={() => setIsMobileMenuOpen((current) => !current)}
            className="inline-flex h-11 w-11 items-center justify-center rounded-2xl border border-slate-200 bg-white text-slate-700 transition hover:bg-slate-50 md:hidden dark:border-white/10 dark:bg-slate-900 dark:text-slate-200"
            aria-label={isMobileMenuOpen ? "Fechar menu" : "Abrir menu"}
            aria-expanded={isMobileMenuOpen}
          >
            <span className="sr-only">{isMobileMenuOpen ? "Fechar menu" : "Abrir menu"}</span>
            <svg aria-hidden="true" className="h-5 w-5" viewBox="0 0 20 20" fill="none">
              {isMobileMenuOpen ? (
                <path d="m5 5 10 10M15 5 5 15" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
              ) : (
                <path d="M4 6h12M4 10h12M4 14h12" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
              )}
            </svg>
          </button>
        </div>

        {isMobileMenuOpen && (
          <div className="mx-auto mt-2 w-full max-w-[1180px] rounded-3xl border border-white/70 bg-white/95 p-3 shadow-xl backdrop-blur-xl md:hidden dark:border-white/10 dark:bg-slate-950/95">
            <div className="grid gap-1">
              {navItems.map((item) => (
                <a
                  key={item.href}
                  href={item.href}
                  onClick={closeMenu}
                  className="rounded-2xl px-4 py-3 text-sm font-semibold text-slate-700 no-underline hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-900"
                >
                  {item.label}
                </a>
              ))}
            </div>
          </div>
        )}
      </header>

      <main className="px-3 pb-4">
        <section className="mx-auto max-w-[1750px] overflow-hidden rounded-[2rem] border border-white/70 bg-white px-4 pb-12 pt-12 shadow-[0_24px_80px_-64px_rgba(15,23,42,0.8)] dark:border-white/10 dark:bg-slate-950 sm:px-6 sm:pt-16 lg:rounded-[2.5rem] lg:px-10 lg:pb-16 lg:pt-20">
          <div className="mx-auto max-w-[1180px]">
            <div className="mx-auto max-w-4xl text-center">
              <span className="inline-flex items-center gap-2 rounded-full border border-teal-200 bg-teal-50 px-4 py-2 text-xs font-black uppercase tracking-[0.18em] text-teal-800 dark:border-teal-400/20 dark:bg-teal-400/10 dark:text-teal-200">
                colaboração instantânea para notas reais
              </span>

              <h1 className="mt-7 text-balance text-5xl font-black text-slate-950 sm:text-6xl lg:text-7xl dark:text-white">
                Abra uma nota pelo nome. Organize tudo no mesmo lugar.
              </h1>

              <p className="mx-auto mt-6 max-w-2xl text-lg leading-8 text-slate-600 dark:text-slate-400">
                Um espaço direto para escrever, dividir em subnotas, gravar áudio, anexar arquivos e colaborar em tempo real sem transformar uma nota simples em um projeto inteiro.
              </p>
            </div>

            <form
              id="abrir-nota"
              onSubmit={handleCreateDocument}
              className="mx-auto mt-9 grid max-w-3xl scroll-mt-28 gap-3 rounded-[1.75rem] border border-slate-200 bg-slate-50 p-2 shadow-inner dark:border-white/10 dark:bg-slate-900 sm:grid-cols-[1fr_auto]"
            >
              <label className="flex min-w-0 items-center gap-3 rounded-3xl bg-white px-5 py-4 dark:bg-slate-950">
                <span className="font-mono text-lg font-semibold text-slate-400 dark:text-slate-500">/</span>
                <input
                  type="text"
                  placeholder="nome-da-sua-nota"
                  value={documentName}
                  onChange={(event) => setDocumentName(event.target.value)}
                  className="min-w-0 flex-1 bg-transparent text-base font-semibold text-slate-950 placeholder:text-slate-400 focus:outline-none dark:text-white dark:placeholder:text-slate-500"
                  autoComplete="off"
                />
              </label>
              <button
                type="submit"
                disabled={!documentName.trim()}
                className="inline-flex items-center justify-center gap-2 rounded-3xl bg-slate-950 px-6 py-4 text-sm font-black text-white transition hover:-translate-y-0.5 hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-white dark:text-slate-950 dark:hover:bg-slate-200"
              >
                Abrir nota
                <ArrowIcon />
              </button>
            </form>

            <div className="mx-auto mt-4 flex max-w-3xl flex-col items-center gap-3 rounded-[1.5rem] border border-teal-200/70 bg-teal-50/70 px-4 py-4 text-center dark:border-teal-400/20 dark:bg-teal-400/10">
              <p className="max-w-2xl text-sm font-medium text-slate-700 dark:text-slate-200">
                Crie uma nota por nome, compartilhe a URL e continue o trabalho no mesmo espaço com texto, subnotas, arquivos e áudio.
              </p>
            </div>

            <div className="mx-auto mt-4 flex max-w-3xl flex-wrap items-center justify-center gap-2">
              {quickExamples.map((example) => (
                <button
                  key={example}
                  type="button"
                  onClick={() => setDocumentName(example)}
                  className="rounded-full border border-slate-200 bg-white px-3 py-1.5 text-sm font-semibold text-slate-600 transition hover:border-slate-300 hover:text-slate-950 dark:border-white/10 dark:bg-slate-900 dark:text-slate-300 dark:hover:text-white"
                >
                  /{example}
                </button>
              ))}
            </div>

            <div className="mx-auto mt-9 grid max-w-4xl gap-3 sm:grid-cols-3">
              {heroStats.map((item) => (
                <div
                  key={item.label}
                  className="rounded-3xl border border-slate-200 bg-slate-50 p-4 text-center dark:border-white/10 dark:bg-slate-900"
                >
                  <div className="text-lg font-black tracking-tight text-slate-950 dark:text-white">
                    {item.value}
                  </div>
                  <p className="mt-1 text-sm font-medium text-slate-500 dark:text-slate-400">
                    {item.label}
                  </p>
                </div>
              ))}
            </div>

            <ProductPreview />
          </div>
        </section>

        <section id="recursos" className="mx-auto mt-4 max-w-[1750px] scroll-mt-28 rounded-[2rem] border border-white/70 bg-[#fbfaf6] px-4 py-14 dark:border-white/10 dark:bg-slate-900 sm:px-6 lg:rounded-[2.5rem] lg:px-10 lg:py-20">
          <div className="mx-auto max-w-[1180px]">
            <div className="grid gap-10 lg:grid-cols-[0.82fr_1.18fr] lg:items-start">
              <div>
                <span className="inline-flex rounded-full border border-slate-200 bg-white px-4 py-2 text-xs font-black uppercase tracking-[0.18em] text-slate-500 dark:border-white/10 dark:bg-slate-950 dark:text-slate-400">
                  recursos principais
                </span>
                <h2 className="mt-5 text-balance text-4xl font-black text-slate-950 sm:text-5xl dark:text-white">
                  Menos interface decorativa. Mais superfície útil para trabalhar.
                </h2>
                <p className="mt-5 max-w-xl text-lg leading-8 text-slate-600 dark:text-slate-400">
                  A home precisa vender a ideia rapidamente, mas o produto precisa continuar prático: escrever, organizar, buscar e compartilhar.
                </p>
              </div>

              <div className="grid gap-4 md:grid-cols-3 lg:grid-cols-1">
                {featureCards.map((feature) => (
                  <article
                    key={feature.title}
                    className="rounded-[1.75rem] border border-slate-200 bg-white p-6 shadow-[0_18px_50px_-42px_rgba(15,23,42,0.5)] dark:border-white/10 dark:bg-slate-950"
                  >
                    <p className="text-xs font-black uppercase tracking-[0.18em] text-teal-700 dark:text-teal-300">
                      {feature.eyebrow}
                    </p>
                    <h3 className="mt-3 text-xl font-black tracking-tight text-slate-950 dark:text-white">
                      {feature.title}
                    </h3>
                    <p className="mt-3 text-sm leading-7 text-slate-600 dark:text-slate-400">
                      {feature.description}
                    </p>
                  </article>
                ))}
              </div>
            </div>

            <div className="mt-12 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
              {capabilityItems.map((item) => (
                <div
                  key={item}
                  className="flex items-center gap-3 rounded-3xl border border-slate-200 bg-white px-4 py-4 text-sm font-bold text-slate-700 dark:border-white/10 dark:bg-slate-950 dark:text-slate-300"
                >
                  <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-2xl bg-teal-50 text-teal-700 dark:bg-teal-400/10 dark:text-teal-300">
                    <svg aria-hidden="true" className="h-4 w-4" viewBox="0 0 20 20" fill="none">
                      <path d="m4 10.5 3.2 3.2L16 5" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
                    </svg>
                  </span>
                  {item}
                </div>
              ))}
            </div>
          </div>
        </section>

        <section className="mx-auto mt-4 max-w-[1750px] rounded-[2rem] border border-white/70 bg-white px-4 py-14 dark:border-white/10 dark:bg-slate-950 sm:px-6 lg:rounded-[2.5rem] lg:px-10 lg:py-20">
          <div className="mx-auto max-w-[1180px]">
            <div className="mx-auto max-w-3xl text-center">
              <span className="inline-flex rounded-full border border-amber-200 bg-amber-50 px-4 py-2 text-xs font-black uppercase tracking-[0.18em] text-amber-800 dark:border-amber-400/20 dark:bg-amber-400/10 dark:text-amber-200">
                comparação prática
              </span>
              <h2 className="mt-5 text-balance text-4xl font-black text-slate-950 sm:text-5xl dark:text-white">
                O produto fica bonito quando a fricção desaparece.
              </h2>
            </div>

            <div className="mt-12 overflow-hidden rounded-[1.75rem] border border-slate-200 dark:border-white/10">
              <div className="grid grid-cols-[1.1fr_1fr_1fr] bg-slate-950 text-sm font-black text-white dark:bg-white dark:text-slate-950">
                <div className="p-4">Situação</div>
                <div className="border-l border-white/10 p-4 dark:border-slate-200">Fluxo comum</div>
                <div className="border-l border-white/10 bg-teal-600 p-4 dark:border-slate-200 dark:bg-teal-500">DontPad BR</div>
              </div>
              {comparisonRows.map((row) => (
                <div
                  key={row.label}
                  className="grid grid-cols-1 border-t border-slate-200 bg-white text-sm dark:border-white/10 dark:bg-slate-950 md:grid-cols-[1.1fr_1fr_1fr]"
                >
                  <div className="p-4 font-black text-slate-950 dark:text-white">{row.label}</div>
                  <div className="border-t border-slate-200 p-4 text-slate-500 dark:border-white/10 dark:text-slate-400 md:border-l md:border-t-0">
                    {row.common}
                  </div>
                  <div className="border-t border-slate-200 bg-teal-50 p-4 font-bold text-teal-900 dark:border-white/10 dark:bg-teal-400/10 dark:text-teal-200 md:border-l md:border-t-0">
                    {row.dontpad}
                  </div>
                </div>
              ))}
            </div>
          </div>
        </section>

        <section id="seguranca" className="mx-auto mt-4 max-w-[1750px] scroll-mt-28 rounded-[2rem] border border-slate-900 bg-slate-950 px-4 py-14 text-white dark:border-white/10 sm:px-6 lg:rounded-[2.5rem] lg:px-10 lg:py-20">
          <div className="mx-auto grid max-w-[1180px] gap-10 lg:grid-cols-[0.9fr_1.1fr] lg:items-center">
            <div>
              <span className="inline-flex rounded-full border border-white/10 bg-white/5 px-4 py-2 text-xs font-black uppercase tracking-[0.18em] text-slate-300">
                segurança e confiabilidade
              </span>
              <h2 className="mt-5 text-balance text-4xl font-black sm:text-5xl">
                Bonita por fora, responsável por dentro.
              </h2>
              <p className="mt-5 max-w-xl text-lg leading-8 text-slate-300">
                A landing mostra o produto com clareza, mas as decisões recentes também reduzem carga desnecessária e mantêm controle de acesso.
              </p>
            </div>

            <div className="grid gap-4">
              {securityItems.map((item, index) => (
                <article
                  key={item.title}
                  className="rounded-[1.5rem] border border-white/10 bg-white/[0.06] p-5"
                >
                  <div className="flex items-start gap-4">
                    <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-2xl bg-white text-sm font-black text-slate-950">
                      0{index + 1}
                    </span>
                    <div>
                      <h3 className="text-lg font-black tracking-tight">{item.title}</h3>
                      <p className="mt-2 text-sm leading-7 text-slate-300">{item.description}</p>
                    </div>
                  </div>
                </article>
              ))}
            </div>
          </div>
        </section>

        <section id="faq" className="mx-auto mt-4 max-w-[1750px] scroll-mt-28 rounded-[2rem] border border-white/70 bg-[#fbfaf6] px-4 py-14 dark:border-white/10 dark:bg-slate-900 sm:px-6 lg:rounded-[2.5rem] lg:px-10 lg:py-20">
          <div className="mx-auto max-w-[1180px]">
            <div className="grid gap-10 lg:grid-cols-[0.78fr_1.22fr]">
              <div>
                <span className="inline-flex rounded-full border border-slate-200 bg-white px-4 py-2 text-xs font-black uppercase tracking-[0.18em] text-slate-500 dark:border-white/10 dark:bg-slate-950 dark:text-slate-400">
                  perguntas frequentes
                </span>
                <h2 className="mt-5 text-balance text-4xl font-black text-slate-950 dark:text-white">
                  Direto o suficiente para começar agora.
                </h2>
              </div>

              <div className="grid gap-3">
                {faqItems.map((item) => (
                  <article
                    key={item.question}
                    className="rounded-[1.5rem] border border-slate-200 bg-white p-5 dark:border-white/10 dark:bg-slate-950"
                  >
                    <h3 className="text-base font-black text-slate-950 dark:text-white">{item.question}</h3>
                    <p className="mt-2 text-sm leading-7 text-slate-600 dark:text-slate-400">{item.answer}</p>
                  </article>
                ))}
              </div>
            </div>

            <div className="mt-12 rounded-[2rem] border border-slate-200 bg-white p-6 dark:border-white/10 dark:bg-slate-950 sm:p-8">
              <div className="grid gap-6 lg:grid-cols-[1fr_auto] lg:items-center">
                <div>
                  <h2 className="text-3xl font-black text-slate-950 dark:text-white">
                    Escolha um nome e entre na nota.
                  </h2>
                  <p className="mt-3 text-base leading-7 text-slate-600 dark:text-slate-400">
                    O melhor teste do DontPad BR é criar uma nota real e usar por alguns minutos com alguém.
                  </p>
                </div>
                <a
                  href="#abrir-nota"
                  className="inline-flex items-center justify-center gap-2 rounded-3xl bg-slate-950 px-6 py-4 text-sm font-black text-white no-underline transition hover:-translate-y-0.5 hover:bg-slate-800 dark:bg-white dark:text-slate-950 dark:hover:bg-slate-200"
                >
                  Criar agora
                  <ArrowIcon />
                </a>
              </div>
            </div>
          </div>
        </section>
      </main>

      <footer className="px-3 pb-6 pt-2">
        <div className="mx-auto flex max-w-[1180px] flex-col gap-4 rounded-3xl border border-white/70 bg-white/70 px-5 py-5 text-sm text-slate-500 dark:border-white/10 dark:bg-slate-950/70 dark:text-slate-400 md:flex-row md:items-center md:justify-between">
          <div className="flex items-center gap-3">
            <LogoMark />
            <div>
              <div className="font-black text-slate-950 dark:text-white">DontPad BR</div>
              <div className="text-xs">documentos colaborativos com entrada direta</div>
            </div>
          </div>
          <div className="flex flex-wrap gap-4">
            {navItems.map((item) => (
              <a
                key={item.href}
                href={item.href}
                className="font-semibold text-slate-500 no-underline transition hover:text-slate-950 dark:text-slate-400 dark:hover:text-white"
              >
                {item.label}
              </a>
            ))}
          </div>
          <p>&copy; {new Date().getFullYear()} DontPad BR.</p>
        </div>
      </footer>
    </div>
  );
}
