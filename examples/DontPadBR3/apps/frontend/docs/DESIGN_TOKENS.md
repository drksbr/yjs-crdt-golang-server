# Design Tokens (UI atual)

Este documento descreve os _design tokens_ implícitos na UI atual (DontPadBR2), com foco em guiar a implementação de uma versão nova mantendo consistência visual.

## Fonte da verdade

- Tailwind: configuração padrão (sem `theme.extend`). Ou seja: os tokens vêm principalmente do **uso de classes Tailwind** no app.
- CSS global: `app/globals.css` define alguns “component tokens” via `@layer components` (ex.: `.btn-primary`, `.glass`) e animações custom.
- Tema escuro: `darkMode: "media"` (via `prefers-color-scheme`), não por classe.

## Tokens de cor

### Neutros (base)

A UI é fortemente baseada em `slate-*` (com alguns `gray-*`). Abaixo, os mapeamentos semânticos mais recorrentes.

**Background (page)**

- Light: `bg-white` ou `bg-slate-50`
- Dark: `dark:bg-slate-900` ou `dark:bg-slate-950`

**Surface (cards/panels)**

- Light: `bg-white`
- Dark: `dark:bg-slate-800` (ou `dark:bg-slate-900` em superfícies mais profundas)

**Surface sutil (inputs, blocos de info)**

- Light: `bg-slate-50`, `bg-slate-100`
- Dark: `dark:bg-slate-800`, `dark:bg-slate-700/50`, `dark:bg-slate-900/50`

**Texto principal**

- Light: `text-slate-900` (em alguns pontos `text-gray-900`)
- Dark: `dark:text-slate-100` (em alguns pontos `dark:text-white`)

**Texto secundário / mutado**

- Light: `text-slate-600`, `text-slate-500`, `text-slate-400` (às vezes `text-gray-500`)
- Dark: `dark:text-slate-400`, `dark:text-slate-300`, `dark:text-slate-500`, `dark:text-slate-600`

**Bordas**

- Light: `border-slate-200`, `border-slate-300`, `border-slate-400`
- Dark: `dark:border-slate-700`, `dark:border-slate-800`, `dark:border-slate-600`, `dark:border-slate-500`

**Divisores**

- Light: `border-b border-slate-200`
- Dark: `dark:border-slate-800`

### Ação (primário vs. secundário)

**Botão primário (base)**

- Light: `bg-slate-900 text-white`
- Dark: `dark:bg-slate-100 dark:text-slate-900`
- Hover: `hover:bg-slate-800` / `dark:hover:bg-slate-200`
- Elevação: `shadow-sm` (base) + `hover:shadow`

**Botão secundário / “ghost” com borda**

- Light: `border border-slate-200 text-slate-700 hover:bg-slate-50`
- Dark: `dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800`

### Feedback/estado (status)

**Erro / perigo**

- Fundo: `bg-red-50` / `dark:bg-red-900/20`
- Borda: `border-red-200` / `dark:border-red-800/50` (ou `dark:border-red-800`)
- Texto: `text-red-700` / `dark:text-red-300` ou `text-red-600` / `dark:text-red-400`
- Ação destrutiva: `bg-red-600 hover:bg-red-700 text-white`

**Sucesso**

- Badge: `bg-green-100` / `dark:bg-green-900/30` com `text-green-700` / `dark:text-green-400`

**Conexão / atenção**

- Indicadores: `bg-amber-500` (ex.: conectando), `bg-emerald-500` (online)

### Acentos (colaboração e destaques)

Usos pontuais:

- `blue-500`: cursor/indicador de usuário.
- `emerald-500`/`emerald-400`: online/live, cursor/“ping”.
- `amber-500`: cursor/indicador e estados intermediários.

### Gradientes (PIN/segurança)

Na tela de PIN há ênfase em `amber`/`orange`:

- Ícone/spotlight: `bg-gradient-to-br from-amber-400 to-orange-500` + `shadow-orange-500/25`
- CTA: `bg-gradient-to-r from-amber-500 to-orange-500` (hover: `from-amber-600 to-orange-600`)
- Focus ring: `focus:ring-amber-400`

### Overlays e transparências

- Overlay modal: `bg-black/50` (muitas vezes com `backdrop-blur-sm`).
- Header “glass”: `bg-white/80` / `dark:bg-slate-900/80` + `backdrop-blur-lg`.

## Tokens globais (elementos base)

Definidos em `app/globals.css` fora de componentes.

### Links

- Base: `text-slate-900 dark:text-slate-100`
- Hover: `hover:text-slate-700 dark:hover:text-slate-300`
- Decoração: `decoration-slate-300 dark:decoration-slate-600 underline-offset-4` + hover ajusta para `hover:decoration-slate-900 dark:hover:decoration-slate-100`
- Externo: `a[target="_blank"]::after` adiciona `↗` e usa `text-slate-400 dark:text-slate-500`

### Inline code

- Light: `bg-slate-100 border-slate-200 text-slate-800`
- Dark: `dark:bg-slate-800 dark:border-slate-700 dark:text-slate-200`
- Forma: `rounded px-2 py-1 font-mono text-sm`

### Checkbox (input padrão)

- Tamanho: `w-5 h-5` + `appearance-none`
- Borda: `border-slate-300 dark:border-slate-600` (hover: `hover:border-slate-400` / `dark:hover:border-slate-500`)
- Raio: `rounded-md`
- Checked:
  - Fundo/borda: `checked:bg-slate-900 checked:border-slate-900` (dark: `dark:checked:bg-slate-100 dark:checked:border-slate-100`)
  - Ícone: SVG via `background-image` (com variação em `prefers-color-scheme: dark`)

## Tokens de tipografia

**Famílias**

- Default: `font-sans`
- Monoespaçada: `font-mono` (ex.: linhas de demo, timers, código)

**Tamanhos recorrentes**

- `text-xs`, `text-sm`, `text-lg`, `text-xl`, `text-2xl`, `text-4xl` (home/hero)
- Responsivo: `md:text-5xl`, `lg:text-6xl`

**Pesos**

- `font-medium`, `font-semibold`, `font-bold`

**Tracking / caixa**

- `tracking-tight` (títulos)
- `uppercase tracking-wide` / `uppercase tracking-wider` (labels)
- Tracking custom de PIN: `tracking-[0.5em]` e `tracking-[0.3em]`

**Altura de linha**

- `leading-relaxed`
- `leading-tight`

**Números**

- `tabular-nums` / `font-variant-numeric: tabular-nums;` (contadores)

## Tokens de layout e espaçamento

Como o Tailwind está “stock”, os tokens de spacing são da escala padrão. Alguns padrões recorrentes:

**Larguras máximas**

- `max-w-sm`, `max-w-md`, `max-w-lg`, `max-w-xl`, `max-w-2xl`
- Layout: `max-w-6xl`, `md:max-w-7xl`

**Alturas**

- Header: `h-16`

**Padding/margin comuns**

- `px-6`, `p-6`, `p-8`, `py-12`, `py-16`, `py-20`, `py-24`
- Gaps: `gap-2`, `gap-3`, `gap-4`, `gap-6`

## Tokens de raio (border-radius)

- `rounded` (base)
- `rounded-md` (inputs/botões)
- `rounded-lg` (cards/modais)
- `rounded-xl` (cards “hero”, containers)
- `rounded-2xl` (PIN container)
- `rounded-full` (badges, dots, avatares, botões circulares)

## Tokens de sombras

- `shadow-sm` (cards leves)
- `shadow-lg` (cards/CTAs)
- `shadow-xl` (modais)
- `shadow-2xl` (PIN container)
- Sombras coloridas (PIN): `shadow-orange-500/25` e variações no hover.
- Em dark: aparece `dark:shadow-slate-900/50` em alguns cards.

## Tokens de borda e foco

**Bordas**

- Predominante: `border` com `border-slate-200` (light) / `dark:border-slate-700|800` (dark)
- Variações: `border-2` em alguns botões circulares; `border-dashed` em presença.

**Rings (focus)**

- Neutro forte: `focus:ring-slate-900` / `dark:focus:ring-slate-100`
- Acento (PIN): `focus:ring-amber-400`
- Em inputs: `focus:border-transparent` aparece junto do ring.

## Tokens de movimento (motion)

### Durações/easing (Tailwind)

- `duration-150`, `duration-200`, `duration-300`, `duration-600`
- `ease-out`, `ease-in-out`

### Animações custom (globals.css)

Classes utilitárias:

- `animate-fade-in-up`: `0.6s ease-out forwards`
- `animate-fade-in`: `0.6s ease-out forwards`
- `animate-slide-in-left` / `animate-slide-in-right`: `0.6s ease-out forwards`
- `animate-typing-cursor`: `1s infinite`
- `animate-float`: `3s ease-in-out infinite`

Delays:

- `animation-delay-100|200|300|400|500|600`

Keyframes definidos:

- `fade-in-up`, `fade-in`, `slide-in-left`, `slide-in-right`, `typing-cursor`, `counter-pulse`, `float`, `gradient-shift`

Além disso, aparecem utilitários do Tailwind como `animate-pulse`, `animate-ping`, `animate-spin`.

### Motion: CSS exato (copiar/colar)

Fonte: `app/globals.css`.

```css
/* Modal animations */
@keyframes modal-backdrop-in {
  from {
    opacity: 0;
  }
  to {
    opacity: 1;
  }
}

@keyframes modal-content-in {
  from {
    opacity: 0;
    transform: scale(0.95) translateY(10px);
  }
  to {
    opacity: 1;
    transform: scale(1) translateY(0);
  }
}

@keyframes modal-shake {
  0%,
  100% {
    transform: translateX(0);
  }
  25% {
    transform: translateX(-4px);
  }
  75% {
    transform: translateX(4px);
  }
}

.animate-modal-shake {
  animation: modal-shake 0.4s ease-in-out;
}

/* Homepage Animations */
@keyframes fade-in-up {
  from {
    opacity: 0;
    transform: translateY(20px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@keyframes fade-in {
  from {
    opacity: 0;
  }
  to {
    opacity: 1;
  }
}

@keyframes slide-in-left {
  from {
    opacity: 0;
    transform: translateX(-20px);
  }
  to {
    opacity: 1;
    transform: translateX(0);
  }
}

@keyframes slide-in-right {
  from {
    opacity: 0;
    transform: translateX(20px);
  }
  to {
    opacity: 1;
    transform: translateX(0);
  }
}

@keyframes typing-cursor {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0;
  }
}

@keyframes counter-pulse {
  0%,
  100% {
    transform: scale(1);
  }
  50% {
    transform: scale(1.02);
  }
}

@keyframes float {
  0%,
  100% {
    transform: translateY(0px);
  }
  50% {
    transform: translateY(-6px);
  }
}

@keyframes gradient-shift {
  0% {
    background-position: 0% 50%;
  }
  50% {
    background-position: 100% 50%;
  }
  100% {
    background-position: 0% 50%;
  }
}

.animate-fade-in-up {
  animation: fade-in-up 0.6s ease-out forwards;
}

.animate-fade-in {
  animation: fade-in 0.6s ease-out forwards;
}

.animate-slide-in-left {
  animation: slide-in-left 0.6s ease-out forwards;
}

.animate-slide-in-right {
  animation: slide-in-right 0.6s ease-out forwards;
}

.animate-typing-cursor {
  animation: typing-cursor 1s infinite;
}

.animate-float {
  animation: float 3s ease-in-out infinite;
}

.animation-delay-100 {
  animation-delay: 100ms;
}

.animation-delay-200 {
  animation-delay: 200ms;
}

.animation-delay-300 {
  animation-delay: 300ms;
}

.animation-delay-400 {
  animation-delay: 400ms;
}

.animation-delay-500 {
  animation-delay: 500ms;
}

.animation-delay-600 {
  animation-delay: 600ms;
}
```

## “Component tokens” (classes utilitárias globais)

Definidas em `app/globals.css` via `@layer components`.

- `.btn-primary`
  - `px-4 py-2 bg-slate-900 dark:bg-slate-100 text-white dark:text-slate-900 rounded-lg hover:bg-slate-800 dark:hover:bg-slate-200 transition font-medium shadow-sm hover:shadow`

- `.btn-secondary`
  - `px-4 py-2 border border-slate-200 dark:border-slate-700 text-slate-700 dark:text-slate-300 rounded-lg hover:bg-slate-50 dark:hover:bg-slate-800 transition font-medium`

- `.card`
  - `rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 shadow-sm hover:shadow-md transition`

- `.glass`
  - `bg-white/80 dark:bg-slate-900/80 backdrop-blur-xl border border-white/20 dark:border-slate-700/50`

- `.glass-dark`
  - `bg-slate-900/80 backdrop-blur-xl border border-slate-700/50`

- `.collab-demo`
  - `relative overflow-hidden rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800`

- `.collab-line`
  - `font-mono text-sm leading-relaxed`

- `.cursor-user-1|2|3`
  - Barras finas (`w-0.5 h-5`) com `rounded-full` em `blue-500`, `emerald-500`, `amber-500`.

## Tokens específicos do editor

Há overrides do BlockNote/checkbox com cores “hardcoded” em `app/globals.css` (não-derivadas de Tailwind):

- `#d1d5db`, `#4b5563`, `#1f2937`, `#e2e8f0`

Recomendação para a versão nova: transformar isso em tokens semânticos (ex.: `editor.border`, `editor.textMuted`, etc.) para evitar hex solto.

## Scrollbar

Há customização de scrollbar via seletores `::-webkit-scrollbar*` em `app/globals.css` (WebKit). Se a versão nova precisar suportar Firefox de forma equivalente, isso provavelmente vira uma decisão de token/tema (ex.: espessura e cores de thumb/track).

## Paleta de colaboradores

Em `lib/colors.ts` existe uma paleta determinística de cores por colaborador (pares light/dark em hex), usada para presença/cursor.

Formato:

- `COLLABORATOR_COLORS[i] = { light: "#...", dark: "#..." }`

Cores (na ordem atual do arquivo):

1. `#ef4444` / `#dc2626`
2. `#f97316` / `#ea580c`
3. `#f59e0b` / `#d97706`
4. `#eab308` / `#ca8a04`
5. `#84cc16` / `#65a30d`
6. `#22c55e` / `#16a34a`
7. `#10b981` / `#059669`
8. `#06b6d4` / `#0891b2`
9. `#3b82f6` / `#2563eb`
10. `#8b5cf6` / `#7c3aed`

Helpers:

- `getCollaboratorColor(index, isDark)`
- `getCollaboratorColorStyle(index, isDark)` → `{ backgroundColor }`
- `getCollaboratorColorWithAlpha(index, isDark, alpha)` → `rgba(...)`

## Replicação 1:1 (classes reais)

Se o objetivo é reproduzir a UI atual **1:1**, os tokens acima ajudam a entender intenção/semântica — mas a fonte de verdade é o **uso real** de classes Tailwind em cada tela/componente.

- Mapa completo (gerado): `docs/CLASSNAME_MAP.md`
- Como regenerar o mapa:
  - `node scripts/extract-classnames.mjs > docs/CLASSNAME_MAP.md`

Notas importantes:

- Dark mode: `tailwind.config.js` usa `darkMode: "media"` (via `prefers-color-scheme`).
- Estilos globais e utilitários custom: `app/globals.css` (links, `code`, checkbox, scrollbar, `@layer components`, keyframes/animações).

### Receitas 1:1 (telas e componentes principais)

As receitas abaixo são um “atalho” para replicação. Para cobertura total (inclusive estados e trechos dinâmicos), use o mapa: `docs/CLASSNAME_MAP.md`.

#### Home (landing)

Fonte: `app/(home)/page.tsx`

- Container raiz:

```tsx
className =
  "min-h-screen bg-slate-50 dark:bg-slate-950 flex flex-col font-sans text-slate-900 dark:text-slate-100 transition-colors";
```

- Header sticky “glass” (variante da home):

```tsx
className =
  "border-b border-slate-200 dark:border-slate-800 bg-white/80 dark:bg-slate-900/80 backdrop-blur-lg sticky top-0 z-10";
```

- Hero section gradient:

```tsx
className =
  "flex flex-col items-center justify-center px-6 py-28 bg-gradient-to-b from-slate-50 dark:from-slate-950 to-white dark:to-slate-900";
```

- Form (cartão com borda + sombra):

```tsx
className =
  "flex gap-2 shadow-lg dark:shadow-slate-900/50 p-1.5 bg-white dark:bg-slate-800 rounded-xl border border-slate-200 dark:border-slate-700 hover:shadow-xl transition-shadow duration-300";
```

- Botão “Ir →” do form:

```tsx
className =
  "px-8 py-3 bg-slate-900 dark:bg-slate-100 text-white dark:text-slate-900 rounded-lg font-medium hover:bg-slate-800 dark:hover:bg-slate-200 transition-all duration-200 disabled:opacity-50 disabled:cursor-not-allowed hover:shadow-md";
```

- Card de exemplo (ExampleCard):

```tsx
className =
  "p-6 rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 hover:border-slate-300 dark:hover:border-slate-600 hover:shadow-lg dark:hover:shadow-slate-900/50 transition-all duration-300 h-full transform hover:-translate-y-1";
```

- Item de feature (ícone “chip” com hover invertendo foreground/background):

```tsx
className =
  "flex-shrink-0 w-12 h-12 rounded-xl bg-slate-100 dark:bg-slate-800 flex items-center justify-center text-slate-600 dark:text-slate-400 group-hover:bg-slate-900 dark:group-hover:bg-slate-100 group-hover:text-white dark:group-hover:text-slate-900 transition-all duration-300";
```

#### Documento (editor)

Fonte: `components/DocumentView.tsx` (renderizado por `app/[documentId]/page.tsx` e `app/[documentId]/[subdocumentName]/page.tsx`)

- Container raiz:

```tsx
className =
  "min-h-screen flex flex-col bg-white dark:bg-slate-900 font-sans text-slate-900 dark:text-slate-100 transition-colors";
```

- Header sticky (variante do editor):

```tsx
className =
  "border-b border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 sticky top-0 z-20";
```

- Botões de painel (Arquivos/Áudio/Subdocs): classe é dinâmica; os dois estados (ativos vs inativos) são:

Ativo:

```tsx
"border-slate-400 dark:border-slate-500 bg-slate-100 dark:bg-slate-800 text-slate-900 dark:text-slate-100";
```

Inativo (default):

```tsx
"border-slate-200 dark:border-slate-700 text-slate-700 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-800 hover:border-slate-300 dark:hover:border-slate-600";
```

- Overlay/backdrop do painel (mobile):

```tsx
className = "fixed inset-0 bg-black/40 z-30 md:hidden";
```

- Painel lateral (Arquivos/Áudio/Subdocs) — base (o trecho final alterna `translate-x-*`):

```tsx
className={`fixed top-16 right-0 bottom-0 md:w-96 w-full border-l border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 overflow-auto shadow-lg z-40 transition-transform duration-200 ease-out ${/* ... */}`}
```

#### PIN (SecureDocumentProvider)

Fonte: `components/SecureDocumentProvider.tsx`

- Container raiz (tela de PIN):

```tsx
className =
  "min-h-screen flex flex-col bg-slate-50 dark:bg-slate-950 font-sans text-slate-900 dark:text-slate-100 transition-colors";
```

- Card:

```tsx
className =
  "bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl shadow-lg dark:shadow-slate-900/50 p-8";
```

- Input PIN:

```tsx
className =
  "w-full px-4 py-3 text-center text-xl font-medium tracking-[0.3em] rounded-lg border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-800 focus:outline-none focus:ring-2 focus:ring-slate-900 dark:focus:ring-slate-100 focus:border-transparent text-slate-900 dark:text-white placeholder-slate-300 dark:placeholder-slate-600 transition disabled:opacity-50 disabled:cursor-not-allowed";
```

#### Modal de PIN (PasswordProtection)

Fonte: `components/PasswordProtection.tsx`

- Container modal:

```tsx
className = "fixed inset-0 z-50 flex items-center justify-center p-4";
```

- Card “glass” com raio 2xl:

```tsx
className =
  "backdrop-blur-xl bg-white/90 dark:bg-slate-800/90 border border-slate-200/50 dark:border-slate-700/50 rounded-2xl shadow-2xl overflow-hidden";
```

- CTA gradiente amber/orange:

```tsx
className =
  "w-full py-3.5 px-4 text-sm font-semibold rounded-xl transition disabled:opacity-50 disabled:cursor-not-allowed bg-gradient-to-r from-amber-500 to-orange-500 hover:from-amber-600 hover:to-orange-600 text-white shadow-lg shadow-orange-500/25 hover:shadow-xl hover:shadow-orange-500/30 active:scale-[0.98]";
```

#### Toast

Fonte: `components/Toast.tsx`

```tsx
className =
  "fixed bottom-6 right-6 bg-slate-900 dark:bg-slate-100 text-white dark:text-slate-900 px-4 py-3 rounded-lg shadow-lg flex items-center gap-2 animate-in fade-in slide-in-from-bottom-4 duration-300 z-50";
```

#### ConfirmDeleteModal

Fonte: `components/ConfirmDeleteModal.tsx`

- Backdrop:

```tsx
className = "fixed inset-0 z-40 bg-black/50 transition-opacity";
```

- Card:

```tsx
className =
  "w-full max-w-sm bg-white dark:bg-slate-800 rounded-lg shadow-lg border border-slate-200 dark:border-slate-700 animate-in fade-in zoom-in-95 duration-200";
```

#### AudioNoteRecorder

Fonte: `components/AudioNoteRecorder.tsx`

- Overlay + blur:

```tsx
className =
  "fixed inset-0 bg-black/50 flex items-center justify-center z-50 backdrop-blur-sm px-4";
```

- Card:

```tsx
className =
  "bg-white dark:bg-slate-900 rounded-lg shadow-xl w-full max-w-md border border-slate-200 dark:border-slate-700";
```

#### DocumentSettings

Fonte: `components/DocumentSettings.tsx`

- Backdrop:

```tsx
className = "absolute inset-0 bg-black/30 backdrop-blur-sm";
```

- Modal:

```tsx
className =
  "relative bg-white dark:bg-slate-800 rounded-xl shadow-xl max-w-lg w-full border border-slate-200/50 dark:border-slate-700/50 overflow-hidden";
```
