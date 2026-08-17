# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

A small trusted group of allowlisted people (family/friends), each using the
app for their own stuff — not a single-user tool, not open/public. Access is
gated by passwordless magic-link login against a pre-set email allowlist.

## Product Purpose

Photo in, price out. A user uploads a photo of something they own; a vision
model identifies the item, and the app queries current eBay listings to
surface an average asking price. Success is a fast, low-friction "what's
this roughly worth" answer.

## Positioning

Not a sale, not an appraisal — a starting point. The app is explicit that it
reports the average *asking* price across current active eBay listings, not
a sold/realized value. This honest framing is deliberate (see Capabilities
and Constraints) and distinguishes it from tools that imply authoritative
valuation.

## Operating Context

- Casual, low-stakes use: a quick curiosity check ("what's this worth"),
  not a considered decision-support tool for selling, donating, or insuring.
  The experience should stay light and fast rather than earnest/authoritative.
- Mobile-friendly camera capture is a real usage path (`capture="environment"`
  on the file input) alongside drag-and-drop/file-picker on desktop.
- A search runs async: upload returns immediately, then the UI polls while
  vision → pricing → stats completes in the background.

## Capabilities and Constraints

- Vision identification via Hugging Face Inference Providers (VLM), pricing
  via eBay Browse API (active listings only — never "sold" data).
- UI copy must never claim "sold price," "market value," or "what it's worth"
  — only "average asking price across current listings" framing (tracked in
  GitHub issue #14).
- Low-confidence identifications and failed searches (e.g. pricing API
  errors) are real, expected states the UI must handle, not edge cases to
  hide.
- Access is allowlist-only; there is no public signup or open browsing.

## Brand Commitments

None yet. The current dark (`neutral-950`) + emerald-500 accent styling is a
functional placeholder from initial build-out, not a binding brand
commitment — open to being replaced by future design work.

## Evidence on Hand

No logos, brand assets, testimonials, or marketing copy exist. This is an
internal/personal tool with no public-facing marketing surface.

## Product Principles

1. Honesty over authority — the app is a starting point, never a definitive
   valuation; copy and design should avoid implying more certainty than the
   underlying data supports.
2. Fast and light — this is a casual curiosity tool, not a considered
   decision-support workflow; friction (forms, steps, waiting) should be
   minimized.
3. Small, trusted audience — no need to design for public trust-building,
   onboarding strangers, or scale; can lean into a more personal, direct
   voice than a consumer product would.
4. Real failure states are first-class — low confidence, no comps found, and
   API failures are expected outcomes the UI must represent clearly, not
   corner cases.
