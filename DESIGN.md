---
name: Face Value
description: Photo in, price out — a personal catalog of what your things are roughly worth.
colors:
  ground: "#eef0f2"
  ground-deep: "#e1e5e9"
  ink: "#1e2226"
  ink-soft: "#565f68"
  line: "#d3d8dd"
  line-strong: "#a7b0b8"
  terracotta-bg: "#f0d3c0"
  terracotta-ink: "#96432a"
  sage-bg: "#dbe6cd"
  sage-ink: "#45602f"
  ochre-bg: "#f2e0a8"
  ochre-ink: "#8c6512"
  slate-bg: "#d6e1e6"
  slate-ink: "#2e5c6e"
  plum-bg: "#e9d6e3"
  plum-ink: "#75335e"
  readout-bg: "#211b14"
  readout-glow: "#ffb454"
  readout-glow-dim: "#8a5f30"
  tag: "#c9a267"
  tag-ink: "#40331d"
  fail: "#963c2b"
  fail-bg: "#f0d9cf"
typography:
  display:
    fontFamily: "Baloo 2, ui-sans-serif, system-ui, sans-serif"
    fontWeight: 700
    letterSpacing: "normal"
  body:
    fontFamily: "ui-sans-serif, system-ui, sans-serif"
    fontWeight: 400
  readout:
    fontFamily: "JetBrains Mono, ui-monospace, SFMono-Regular, monospace"
    fontWeight: 700
rounded:
  card: "14px"
  card-lg: "18px"
  pill: "9999px"
  field: "6px"
spacing:
  card-padding: "12px"
  section-gap: "32px"
components:
  price-readout:
    backgroundColor: "{colors.readout-bg}"
    textColor: "{colors.readout-glow}"
    typography: "{typography.readout}"
    rounded: "{rounded.field}"
    padding: "2px 8px"
  button-primary:
    backgroundColor: "{colors.terracotta-ink}"
    textColor: "#ffffff"
    typography: "{typography.display}"
    rounded: "{rounded.pill}"
    padding: "8px 20px"
---

# Design System: Face Value

## Overview

**Creative North Star: "The Collector's Roster"**

Face Value refuses the marketplace-listing default. A search isn't a row in a
grid of things for sale — it's a specimen card in a personal collection, the
kind of catalog page a collector keeps for figurines, watches, or die-cast
cars: a name plate, a "found on" date, a one-line condition note, and — the
one cold, precise fact in the whole warm system — the price, rendered as a
lit instrument readout rather than plain type.

The system runs on a deliberate temperature contrast: a cool, quiet gray
ground and near-black ink carry the page, while each card claims its own
committed pastel field (never a neutral card on a neutral ground — color
commits at the region, not as an accent), and the price chip glows amber on
near-black, the one warm, mechanical note against all that cool gray and soft
pastel. Status is physical, not iconographic: a pending search carries a
small tag dangling off the card's corner, which visibly detaches (an
animated cinch-and-snap, not an instant disappearance) the moment the search
resolves; a failed search shows a torn paper corner instead of a red banner.

The system was corrected twice during build: an initial warm cream ground was
explicitly rejected by the user in favor of the current cool light gray, and
"roster"/"collection" as visible vocabulary was rejected in favor of the
product's own word, "searches" — the visual metaphor (a personal collector's
catalog) stays, but the UI never names itself that out loud.

**Key Characteristics:**
- Cool, neutral gray ground; warmth lives in the cards and the price glow, never the page.
- Five committed pastel card colors, deterministically assigned per item, never randomized per render.
- The price is always the coldest, most mechanical mark on the page — mono, tabular, glowing.
- Status is a physical device (dangling tag, torn corner), never a colored badge or spinner alone.
- All visible copy says "searches," never "roster" or "collection" — that vocabulary is build-internal only.

## Colors

Two families: a quiet, cool neutral scale that carries structure and reading,
and a five-color pastel "specimen" scale that never mixes with the neutral
scale — a card is always fully one pastel color, never neutral.

### Primary
- **Ember Terracotta** (`#96432a` ink / `#f0d3c0` bg): the system's one recurring accent — primary buttons, links, focus rings, drag-active states. Used as the "you can act here" signal across an otherwise quiet page.

### Neutral
- **Cool Slate Ground** (`#eef0f2`): the page background. Deliberately cool and quiet — not paper, not kraft.
- **Ground Deep** (`#e1e5e9`): recessed surfaces — empty states, input fields, table headers.
- **Near-Black Ink** (`#1e2226`): primary text.
- **Soft Ink** (`#565f68`): secondary text, labels, metadata — tinted from the ink hue, never true gray.
- **Line** (`#d3d8dd`) / **Line Strong** (`#a7b0b8`): hairlines, borders, dashed upload zone.

### Specimen palette (cards)
Five pastel fields, assigned deterministically per search id (a hash of the id, never random) so a card's color is stable across reloads. Each pairs a light background with a darker ink of the same hue for its metadata text:
- **Terracotta** (`#f0d3c0` / `#96432a`)
- **Sage** (`#dbe6cd` / `#45602f`)
- **Ochre** (`#f2e0a8` / `#8c6512`)
- **Slate** (`#d6e1e6` / `#2e5c6e`)
- **Plum** (`#e9d6e3` / `#75335e`)

### Named Rules
**The One Ground Rule.** The page ground is always the cool neutral; a card is always a full pastel field. The two families never blend — no tinted neutral, no desaturated pastel standing in for the page background. The one bounded exception is the specimen wash (see Components) behind the letterhead — everywhere the search grid itself sits, ground stays flat neutral.

**The Cold Price Rule.** The price readout is always near-black background with amber glow (`#211b14` / `#ffb454`), regardless of the card's pastel color. It's the one element in the system that never adopts the local palette — its job is to read as a different kind of fact (measured, not decorated).

## Typography

**Display Font:** Baloo 2 (self-hosted variable font, weights 500–800)
**Body Font:** system sans stack (`ui-sans-serif, system-ui, sans-serif`)
**Readout Font:** JetBrains Mono (self-hosted variable font) — reserved for the price figure and raw comp prices only

**Character:** Baloo 2's rounded, friendly bubble-terminal letterforms carry every name plate and heading — the collector's-catalog warmth. Body copy stays in a plain system sans so metadata and paragraphs never compete with the display voice. JetBrains Mono's tabular figures give the price its instrument-panel precision.

### Hierarchy
- **Display** (Baloo 2, 700, `text-2xl`–`text-3xl`): page title, card nameplates, dialog headings.
- **Body** (system sans, 400, `text-sm`): descriptions, labels, table content.
- **Label** (system sans, 500, `text-xs`, uppercase, tracked): metadata like "found 3 hours ago," section headers.
- **Readout** (JetBrains Mono, 700, tabular-nums): the price figure only — never used for any other numeral in the UI (dates, counts stay in the body font).

### Named Rules
**The One Mono Rule.** JetBrains Mono appears in exactly two places: the glowing price chip and the comp table's price column. It is never used as a "technical" costume elsewhere.

## Layout

Cards sit in a responsive CSS grid: 2 columns on mobile, 3 at `sm`, 4 at `xl`
(`grid-cols-2 sm:grid-cols-3 xl:grid-cols-4`), 16px gap. The detail page is a
single card's own layout scaled up — image and metadata side by side at
`sm` and above, stacked below — under a `max-w-4xl` reading column. Section
rhythm follows more space above a heading than below it; the standard gap
between major sections is 32px (`space-y-8`).

## Elevation & Depth

Flat by default — cards carry a soft, real-offset shadow (`0 1px 0
rgba(36,31,26,.06), 0 6px 16px -10px rgba(36,31,26,.35)`), never a hard
zero-blur block shadow. The price readout's "glow" is a `text-shadow`, not a
box shadow — it reads as emitted light from the glyphs themselves, matching
its instrument-panel character.

### Named Rules
**The Real Shadow Rule.** Every shadow carries both an offset and a blur. A flat colored halo is never used as a substitute for depth.

## Shapes

Cards use a 14px radius (18px on the larger detail-page card); buttons and
the price/query input are full pill radius. The price readout and tag use a
tighter 6px/3px radius respectively — smaller elements read as more
precisely cut. The pending tag and failed-search torn corner are the two
places geometry departs from pure rounded rectangles: the tag is a small
rotated rectangle with a punched hole (drawn as SVG, not an icon font), and
the torn corner is a `clip-path` polygon cut into the card's top-right.

## Components

### Buttons
- **Shape:** full pill (`rounded-full`).
- **Primary:** Ember Terracotta background, white text, Baloo 2 display font, `min-h-12` with `px-5 py-3` to `px-7 py-3.5` — full-width on mobile, auto-width from `sm` up. Lifts 2px on hover (`hover:-translate-y-0.5`), settles back on `active`.
- **Ghost/Link:** terracotta-ink text with a dotted underline (the "Retry" action on a failed card) or plain underline-on-hover (comp "View" links). Given a small padded hit area (`px-1 py-1.5` or `-mx-2 -my-1 px-2 py-1`) so the tap target clears 44px even though the visible underline stays link-sized.

### Logo Mark
A single authored shape: a rotated rounded-square tag with a punched circular hole near its top-left corner, filled Ember Terracotta, the hole cut in the surrounding ground color. It's the same "physical tag" device the pending-status chip already uses, reused at brand scale rather than inventing a second icon language — `AppLogo.vue`, sized 30–44px depending on context (header vs. login).

### Specimen Wash
A bounded atmospheric background (`.specimen-wash` in `main.css`) used behind the letterhead on the home page and centered on the login page: five blurred radial-gradient blooms, one per specimen hue (terracotta/sage/ochre/slate/plum), layered over `ground-deep` via a blurred `::before`. It answers "the ground is always neutral" by keeping the wash to these two bounded regions — the search grid itself still sits on flat `ground`. Read it as the specimen palette introducing itself before it gets cut into individual cards below.

### Search Card (signature component)
The system's defining element. A pastel-field card (one of the five specimen colors, chosen deterministically) with: a square photo, a Baloo 2 nameplate, a small uppercase "found `<time ago>`" label in the card's own ink tone, an italic one-line note (condition or search query), and the price readout bottom-left with comp count bottom-right. Pending searches carry the dangling tag (top-right, rotated, animates in/out via a cinch-and-snap transition rather than appearing/vanishing instantly); failed searches carry the torn-corner clip-path instead and drop the photo to `saturate-50 opacity-70`.

### Price Readout
- **Style:** near-black chip (`#211b14`), amber glow text (`#ffb454`) with a soft `text-shadow`, JetBrains Mono, tabular figures, bold.
- **Sizes:** compact (`1.15rem`) inline on cards; large (`2.5rem`) as the detail page's headline figure.
- **Rule:** never recolors to match the surrounding card — always the same dark-chip-amber-glow treatment regardless of context.

### Fields
- **Style:** `bg-ground-deep`, `ring-1 ring-line-strong`, `rounded-md`, no visible border beyond the ring.
- **Focus:** browser-default focus-visible outline is themed to Ember Terracotta (`outline: 2px solid var(--color-terracotta-ink)`), not left as browser blue.

### Tables (comparable listings)
- **Header:** `bg-ground-deep`, soft-ink label text, uppercase-free (sentence case).
- **Rows:** hairline top border (`border-line`); excluded/outlier rows drop to 50% opacity and carry a small "outlier" pill rather than being hidden by default.

## Do's and Don'ts

### Do:
- **Do** keep the page ground cool and neutral (`#eef0f2`) — warmth is a property of the cards and the price glow, never the page itself.
- **Do** assign card color deterministically from the search id, so the same search always renders the same pastel field.
- **Do** render status (pending/still-working/failed) as a physical device (tag, torn corner) with an actual enter/exit transition, never as a static badge that just appears and disappears.
- **Do** reserve JetBrains Mono strictly for price figures.
- **Do** theme browser-native surfaces (selection color, focus ring, scrollbar) from the palette rather than leaving OS defaults.
- **Do** default touch-only controls to visible state (`@media (hover: none)`) rather than gating them behind `:hover`/`group-hover`, which touch input can never trigger.

### Don't:
- **Don't** use "roster" or "collection" in any user-visible copy — the product's own word is "searches."
- **Don't** write inline `style="..."` attributes in templates; every color/font value is a Tailwind utility class backed by the `@theme` tokens above. Scoped `<style>` blocks remain only for effects Tailwind can't express (clip-path shapes, text-shadow glow, keyframed transitions).
- **Don't** let a card's pastel background bleed into the price readout — the readout is always the dark-chip treatment regardless of card color.
- **Don't** use a warm/cream ground — this was tried and explicitly rejected by the user.
- **Don't** use bounce/elastic easing on any transition (flagged by the project's design detector); motion decelerates smoothly, exponential ease-out.
