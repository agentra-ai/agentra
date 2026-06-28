# Landing Page Code Review Fixes

**Goal:** Fix all 10 issues identified in the landing page code review — making the homepage production-ready with proper SSR/SEO, accessibility, type safety, and maintainability.

**Architecture:** The landing page (`app/(landing)/`) uses a feature module at `features/landing/` with its own i18n system. Fixes are organized into 4 phases by priority. Each phase is independently shippable.

**Tech Stack:** Next.js 16 (App Router), React 19, Three.js, next-intl, Tailwind CSS v4, shadcn/ui.

---

## Phase 1 — Critical: Stability, Accessibility, Dead Code

### 1.1 Delete duplicate `/homepage` route

The route `app/(landing)/homepage/page.tsx` renders the identical `<AgentraLanding />` as `/`, causing duplicate content and SEO canonical conflicts.

- [ ] Delete `app/(landing)/homepage/` directory entirely
- [ ] Remove any internal links pointing to `/homepage` (grep first)

### 1.2 Add ErrorBoundary + dynamic import for Three.js scene

`landing-proof-scene.tsx` (845 lines) loads Three.js unconditionally on the client. If it throws, the entire landing page white-screens. No error boundary exists anywhere in the app.

- [ ] Create `app/(landing)/error.tsx` — Next.js error boundary for the landing route group (renders a fallback dark section with the headline text, no 3D)
- [ ] Wrap `<LandingProofScene />` usage in `landing-theater.tsx` with a React error boundary component (create `features/landing/components/scene-error-boundary.tsx`)
- [ ] Change `LandingProofScene` import in `landing-theater.tsx` to `next/dynamic` with `{ ssr: false, loading: () => <FallbackGradient /> }` — this also removes Three.js from the server bundle
- [ ] Ensure the fallback gradient div (already in `landing-proof-scene.tsx` line 841) is extracted to a standalone export so both the dynamic loading state and the error boundary can reuse it

### 1.3 Add `prefers-reduced-motion` handling

The 3D scene runs a continuous `requestAnimationFrame` loop and the theater auto-advances steps every 4.8s. Users who request reduced motion get neither respected.

- [ ] Create `hooks/use-prefers-reduced-motion.ts` — returns `boolean` via `window.matchMedia("(prefers-reduced-motion: reduce)")`
- [ ] In `landing-theater.tsx`: when reduced motion is active, disable the `setInterval` auto-advance (user can still click step buttons manually)
- [ ] In `landing-proof-scene.tsx`: when reduced motion is active, render a static frame (one `renderer.render()` call, no `requestAnimationFrame` loop) — the robot stays at the current station without idle bobbing/eye pulse
- [ ] Add `aria-live="polite"` to the active step detail panel so screen readers announce step changes

### 1.4 Fix `any` types + dispose textures in 3D scene

The Three.js code uses `any` pervasively (suppressed with `eslint-disable`) and leaks textures on unmount.

- [ ] Define a `StationProps` interface in `landing-proof-scene.tsx`:
  ```ts
  interface StationProps {
    group: THREE.Group;
    mat0: THREE.Material;
    mat1?: THREE.Material;
    mat2?: THREE.Material;
  }
  ```
- [ ] Define a `RobotParts` interface for `buildRobot` return type:
  ```ts
  interface RobotParts {
    body: THREE.Mesh;
    head: THREE.Mesh;
    eyeL: THREE.Mesh;
    eyeR: THREE.Mesh;
    antenna: THREE.Mesh;
    armL: THREE.Mesh;
    armR: THREE.Mesh;
    legL: THREE.Mesh;
    legR: THREE.Mesh;
    scanBeam: THREE.Mesh;
  }
  ```
- [ ] Replace all `any` type annotations in `buildRobot`, `buildStationProps`, and the animation loop
- [ ] Remove both `eslint-disable-next-line @typescript-eslint/no-explicit-any` comments
- [ ] In the cleanup function: dispose textures alongside geometries and materials — traverse scene and dispose `texture` on materials that have `map`
- [ ] Replace `prop.group.children[index]` access pattern with direct references stored in `StationProps` (add `scanLine`, `crossH`, `crossV`, etc. fields as needed)

### 1.5 Remove dead code

- [ ] Delete `ImageIcon` from `shared.tsx` (never imported)
- [ ] Delete `heroButtonClassName` from `shared.tsx` (never imported)

---

## Phase 2 — High: SSR/SEO + Hydration

### 2.1 Split static content into server components

`AgentraLanding` is `"use client"`, meaning all headline/description/step text is absent from the initial HTML. Search engines and social crawlers see an empty page.

- [ ] Create `features/landing/components/landing-theater-content.tsx` — a **server component** that renders the left column (kicker, h1, description, CTAs, proof chips, works-with). It receives locale + user as props.
- [ ] Create `features/landing/components/landing-value-props.tsx` (already exists, but make it a server component — remove `"use client"`, pass `t` as prop instead of calling `useLocale()`)
- [ ] `landing-theater.tsx` becomes a thin client wrapper that renders:
  - `<LandingTheaterContent />` (server, via children or composition)
  - The interactive right panel (step switching, 3D scene) — this stays client
- [ ] `agentra-landing.tsx` — remove `"use client"` if possible, or keep it but ensure the server-rendered children produce HTML
- [ ] Verify: `curl https://web.agentra.orb.local/ | grep "Make coding agents"` — the headline text must appear in the raw HTML

### 2.2 Fix hydration risk in footer

`LandingFooter` calls `new Date().getFullYear()` inline in a client component. At the year boundary this can cause a server/client mismatch.

- [ ] Compute `currentYear` once at module level (outside component) in `landing-footer.tsx`
- [ ] Alternatively, pass it as a prop from the server layout

---

## Phase 3 — Medium: Design Tokens, Component Split, Dead i18n

### 3.1 Replace hardcoded colors with CSS design tokens

The landing page uses ~30 unique hardcoded hex values (`#04070d`, `#05070b`, `#0a0d12`, etc.) and opacity utilities (`text-white/72`, `bg-white/[0.04]`). AGENTS.md requires design tokens.

The landing page intentionally uses a darker palette than shadcn's `.dark` theme, so we add landing-specific CSS variables.

- [ ] Add to `app/globals.css` `:root` block:
  ```css
  /* Landing page — custom dark theme */
  --landing-bg: oklch(0.07 0.005 250);
  --landing-bg-deep: oklch(0.05 0.005 250);
  --landing-surface: oklch(0.09 0.005 250);
  --landing-fg: oklch(0.98 0 0);
  --landing-fg-muted: oklch(0.62 0.005 250);
  --landing-fg-subtle: oklch(0.42 0.005 250);
  --landing-border: oklch(0.98 0 0 / 8%);
  --landing-accent: oklch(0.72 0.12 200);
  ```
- [ ] Map these as Tailwind theme tokens in the `@theme inline` block:
  ```css
  --color-landing-bg: var(--landing-bg);
  --color-landing-bg-deep: var(--landing-bg-deep);
  --color-landing-surface: var(--landing-surface);
  --color-landing-fg: var(--landing-fg);
  --color-landing-fg-muted: var(--landing-fg-muted);
  --color-landing-fg-subtle: var(--landing-fg-subtle);
  --color-landing-border: var(--landing-border);
  --color-landing-accent: var(--landing-accent);
  ```
- [ ] Replace hardcoded values across landing components:
  - `bg-[#04070d]` → `bg-landing-bg`
  - `bg-[#05070b]` → `bg-landing-bg-deep`
  - `text-white` → `text-landing-fg`
  - `text-white/72` → `text-landing-fg-muted`
  - `text-white/42` → `text-landing-fg-subtle`
  - `border-white/8` → `border-landing-border`
  - etc.
- [ ] Keep `white/XX` opacity utilities only where they're used as overlays on the 3D canvas (inside the scene container) — these are genuinely dynamic and don't need tokens

### 3.2 Split `landing-theater.tsx` (474 lines) into smaller components

- [ ] Extract `landing-theater-left-column.tsx` — kicker, headline, description, CTAs, proof chips, works-with section (this is the server-renderable part from 2.1)
- [ ] Extract `landing-theater-panel.tsx` — the right-side "product proof" panel (header bar, task info, progress, scene container, detail card)
- [ ] Extract `landing-theater-detail-card.tsx` — the bottom detail card (active focus, result, system facts, next action)
- [ ] Extract `landing-theater-step-buttons.tsx` — the clickable step buttons overlaid on the scene
- [ ] `landing-theater.tsx` becomes the orchestrator: manages `activeIndex` state, auto-advance timer, reduced-motion check; renders the sub-components

### 3.3 Remove dead i18n content

`en.ts`/`zh.ts` define `hero`, `features`, `howItWorks`, `openSource`, and `faq` sections that are never rendered. These will be removed (not rendered) — the landing page's current 3-section structure (theater + value props + footer) is intentional.

- [ ] Remove `hero`, `features`, `howItWorks`, `openSource`, `faq` from `LandingDict` type in `types.ts`
- [ ] Remove the corresponding sections from `en.ts` and `zh.ts`
- [ ] Remove `cycleLabel`, `cycleHint`, `panelFeedLabel` from `theater` if also unused (grep to confirm)

---

## Phase 4 — Medium: i18n Unification

### 4.1 Migrate landing i18n to `next-intl`

The landing page has its own React Context i18n (`features/landing/i18n/`) while the rest of the app uses `next-intl`. This violates AGENTS.md ("Prefer existing patterns over parallel abstractions").

- [ ] Move all landing content from `features/landing/i18n/en.ts` into `messages/en.json` under a `"landing"` top-level key
- [ ] Move all landing content from `features/landing/i18n/zh.ts` into `messages/zh-CN.json` under a `"landing"` top-level key
- [ ] In landing components: replace `useLocale()` → `useTranslations("landing")` (next-intl hook)
- [ ] For arrays (e.g. `theater.steps`), use next-intl's array message format or keep as JSON in the messages file
- [ ] Remove `features/landing/i18n/` directory entirely (context.tsx, en.ts, zh.ts, types.ts, index.ts)
- [ ] Remove `LocaleProvider` from `app/(landing)/layout.tsx` — next-intl is already initialized in `app/layout.tsx`
- [ ] Move locale detection logic (Accept-Language + cookie) into `i18n/request.ts` (already partially handles cookie)
- [ ] Update `landing-footer.tsx` locale switcher to use `useLocale()` from `next-intl` + `useRouter()` to switch locale
- [ ] The locale switcher cookie name stays `agentra-locale` — just ensure `i18n/request.ts` reads it consistently with the values `"en"` and `"zh-CN"`

---

## Verification

After each phase:
```bash
pnpm typecheck          # TypeScript errors
pnpm lint               # ESLint
pnpm test               # Vitest unit tests
```

After all phases:
```bash
make check              # Full pipeline (typecheck + tests + Go tests + E2E)
```

Manual checks:
- `curl https://web.agentra.orb.local/` — verify headline text appears in raw HTML
- Browser DevTools → Rendering → emulate `prefers-reduced-motion: reduce` — verify no auto-advance, static 3D
- Browser DevTools → disable WebGL — verify fallback renders without errors
- Toggle language EN/中文 — verify all text switches
