const PALETTE = [
  { bg: 'bg-terracotta-bg', text: 'text-terracotta-ink', imgBg: 'bg-terracotta-ink/15', skeleton: 'bg-terracotta-ink/25' },
  { bg: 'bg-sage-bg', text: 'text-sage-ink', imgBg: 'bg-sage-ink/15', skeleton: 'bg-sage-ink/25' },
  { bg: 'bg-ochre-bg', text: 'text-ochre-ink', imgBg: 'bg-ochre-ink/15', skeleton: 'bg-ochre-ink/25' },
  { bg: 'bg-slate-bg', text: 'text-slate-ink', imgBg: 'bg-slate-ink/15', skeleton: 'bg-slate-ink/25' },
  { bg: 'bg-plum-bg', text: 'text-plum-ink', imgBg: 'bg-plum-ink/15', skeleton: 'bg-plum-ink/25' }
] as const

/** useSearchPalette deterministically assigns one of the five committed
 * card colors from a search id, so a card's color stays stable across
 * reloads instead of reshuffling every render. Returns Tailwind utility
 * class names (registered as @theme tokens in main.css), never raw
 * values, so templates never need an inline style for card color. */
export function useSearchPalette(id: string) {
  let hash = 0
  for (let i = 0; i < id.length; i++) {
    hash = (hash * 31 + id.charCodeAt(i)) >>> 0
  }
  return PALETTE[hash % PALETTE.length]!
}
