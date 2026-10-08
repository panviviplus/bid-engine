# Design — 标擎 BidEngine

Locked design system. Future Hallmark work reads this file first and defers to
it. Amend intentionally; approved page designs do not drift without explicit
user approval.

## System

- Genre · modern-minimal application with an atmospheric authentication variant
- Macrostructure · Split Studio for SignIn · Workbench for authenticated pages
- Theme · custom, preserving BidEngine deep-sea navy and signal gold
- Axes · dark paper / geometric sans / warm gold accent
- Typography · Inter + PingFang SC, upright headings only

## SignIn V10 — frozen

- Three screens: positioning and authentication · capability loop · flexible
  entry
- Logo · `/bid-engine-logo.ico`, 46 px; adjacent “标擎” wordmark, 18 px
- Hero · “让每一次投标，都有章可循”
- Stage progress · one continuous, linear 20-second lifecycle; nodes activate
  when the runner reaches their measured positions, and stage bubbles linger 8
  seconds before fading
- Capability and entry showcases · 6-second rotation, manual controls, pause on
  hover/focus/hidden page
- Mobile · single column; authentication before showcases; automatic motion
  disabled
- Authentication · SMS and password remain available; APP QR entry is a
  non-interactive future notice
- Authentication card · desktop top edge aligns with the brand logo and sits 8
  px inside the former right boundary
- Card header · always “登录标擎”; the adjacent “智能投标工作台” capsule uses a
  contained signal-gold + paper dual-track sweep, with a static gold treatment
  under reduced motion
- Registration · opens from password login in a responsive application modal;
  desktop uses a centred, internally scrolling dialog and mobile uses the full
  dynamic viewport. Nickname and company remain optional; mobile, SMS code, and
  password remain required. The standalone `/signup` route remains unchanged
- User-facing copy must not contain internal design jargon or fabricated metrics

## Tokens

`tokens.css` is the portable source; Chakra mappings live in `theme/default.js`.

```css
:root {
  --color-paper: oklch(98.4% 0.004 247);
  --color-ink: oklch(17% 0.03 252);
  --color-muted: oklch(81% 0.025 250);
  --color-accent: oklch(79% 0.13 82);
  --color-focus: oklch(79% 0.13 82);
  --color-auth-canvas: oklch(16% 0.028 252);
  --color-auth-surface: oklch(27% 0.04 250);
  --font-display: "Inter", "PingFang SC", ui-sans-serif, system-ui, sans-serif;
  --font-body: "Inter", "PingFang SC", ui-sans-serif, system-ui, sans-serif;
  --ease-out: cubic-bezier(0.16, 1, 0.3, 1);
  --ease-in-out: cubic-bezier(0.65, 0, 0.35, 1);
  --dur-signin-pill-cycle: 4s;
  --radius-card: 24px;
  --radius-pill: 999px;
  --radius-input: 8px;
}
```

## CTA voice

- Primary · deep navy fill · 8 px radius · direct verb-first labels
- Secondary · restrained outline or text action · same focus treatment
- Focus · immediate signal-gold ring with at least 3:1 contrast

## Motion stance

- Information-first stage emphasis, product cross-fades, and silent
  micro-feedback
- The core-positioning capsule may use one low-frequency dual-track sweep with a
  long dwell; it remains static when reduced motion is requested
- Animate opacity and transform only; no bounce or decorative perpetual motion
- Reduced-motion fallback · static/manual state with opacity transitions no
  longer than 150 ms

## Responsive layout

- Application canvas · authenticated pages consume the entire width left by
  the side rail; page-level `max-width` caps are forbidden.
- Page gutters · 12 px at the smallest viewport, then 16 / 24 / 32 / 40 px as
  the canvas grows. Authentication surfaces use the roomier 16 / 32 / 40 /
  48 px scale.
- Wide screens · at 2560×1440, cards and workspace panels add fluid columns or
  expand their functional surface instead of recentering inside a 1440–1720 px
  wrapper.
- Compact laptops · on common 14-inch viewports, toolbars may wrap before
  shrinking controls; detail sidebars collapse before the primary content is
  compressed.
- Small screens · page tracks collapse to one column, controls take the
  available width, touch targets remain at least 44 px high, and only complex
  editors may scroll inside their own bounded workspace.
- Reading measure · line-length limits belong to paragraphs, forms, and
  document content only; they must never constrain the full page canvas.
- Viewport safety · use `100dvh` or the parent shell height, `width: 100%`,
  `min-width: 0`, and root `overflow-x: clip`; never use `100vw` for page roots.

## Exports

- CSS custom properties · `tokens.css`
- Chakra UI · `theme/default.js` under `colors.signin`
- Tailwind, DTCG, and shadcn exports are intentionally omitted because this
  project uses Chakra UI
