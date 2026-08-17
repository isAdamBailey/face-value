<script setup lang="ts">
import type { Search } from '~/stores/searches'

const props = defineProps<{ search: Search }>()
const emit = defineEmits<{ retry: [id: string] }>()

const { format: formatMoney } = useMoney()
const { format: formatRelativeTime } = useRelativeTime()
const palette = useSearchPalette(props.search.id)

const isPending = computed(() => !isTerminal.value)
const isTerminal = computed(() => props.search.status === 'complete' || props.search.status === 'failed')
const isFailed = computed(() => props.search.status === 'failed')
const isComplete = computed(() => props.search.status === 'complete')

const nameplate = computed(() => {
  if (props.search.title) {
    return props.search.title
  }
  return isFailed.value ? 'Unidentified item' : 'Identifying…'
})

const noteLine = computed(() => {
  if (isFailed.value) {
    return props.search.error_message || "Couldn't complete this search"
  }
  return props.search.condition_notes || props.search.search_query || ''
})
</script>

<template>
  <NuxtLink
    :to="isTerminal ? `/search/${search.id}` : undefined"
    class="search-card group relative flex flex-col overflow-hidden rounded-[14px] ring-1 ring-white/70 transition-transform"
    :class="[palette.bg, isTerminal ? 'hover:-translate-y-0.5' : 'cursor-default']"
  >
    <Transition name="tag-detach">
      <span
        v-if="isPending"
        class="tag"
        :class="{ 'tag--knotted': search.still_working }"
        aria-hidden="true"
      >
        <svg viewBox="0 0 10 10" class="tag__hole"><circle cx="5" cy="5" r="3" /></svg>
        {{ search.still_working ? 'still working' : 'pending' }}
      </span>
    </Transition>

    <div
      v-if="isFailed"
      class="torn-corner"
      aria-hidden="true"
    />

    <div class="aspect-square w-full overflow-hidden" :class="palette.imgBg">
      <div
        v-if="!search.image_url"
        class="h-full w-full animate-pulse"
        :class="palette.skeleton"
      />
      <img
        v-else
        :src="search.image_url"
        :alt="search.title || 'Uploaded item'"
        class="h-full w-full object-cover"
        :class="{ 'saturate-50 opacity-70': isFailed }"
      >
    </div>

    <div class="flex flex-1 flex-col gap-1.5 p-3">
      <p class="truncate font-display text-[1.05rem] leading-tight font-semibold text-ink">
        {{ nameplate }}
      </p>

      <p class="text-[0.7rem] tracking-wide uppercase" :class="palette.text">
        found {{ formatRelativeTime(search.created_at) }}
      </p>

      <p
        v-if="noteLine"
        class="line-clamp-2 text-xs text-ink-soft italic"
      >
        “{{ noteLine }}”
      </p>

      <div class="mt-auto flex items-end justify-between gap-2 pt-2">
        <span
          v-if="isComplete"
          class="readout"
        >
          {{ formatMoney(search.price_trimmed_mean, search.currency) }}
        </span>
        <span v-else class="text-xs text-ink-soft">
          {{ isFailed ? '' : 'valuing…' }}
        </span>

        <span
          v-if="isComplete && search.comp_count !== undefined"
          class="shrink-0 text-[0.65rem] text-ink-soft"
        >
          {{ search.comp_count }} listing{{ search.comp_count === 1 ? '' : 's' }}
        </span>
      </div>

      <button
        v-if="isFailed || search.still_working"
        type="button"
        class="self-start text-xs font-medium text-terracotta-ink underline decoration-dotted underline-offset-2"
        @click.prevent="emit('retry', search.id)"
      >
        Retry
      </button>
    </div>
  </NuxtLink>
</template>

<style scoped>
.search-card {
  box-shadow: 0 1px 0 rgba(36, 31, 26, 0.06), 0 6px 16px -10px rgba(36, 31, 26, 0.35);
}

.readout {
  font-family: var(--font-mono);
  font-variant-numeric: tabular-nums;
  font-weight: 700;
  font-size: 1.15rem;
  color: var(--color-readout-glow);
  background: var(--color-readout-bg);
  padding: 0.15rem 0.5rem;
  border-radius: 6px;
  text-shadow: 0 0 8px var(--color-readout-glow-dim);
}

.tag {
  position: absolute;
  top: 0.6rem;
  right: -0.35rem;
  z-index: 10;
  display: flex;
  align-items: center;
  gap: 0.25rem;
  padding: 0.15rem 0.5rem 0.15rem 0.35rem;
  background: var(--color-tag);
  color: var(--color-tag-ink);
  font-size: 0.6rem;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  border-radius: 3px 0 0 3px;
  transform: rotate(3deg);
  box-shadow: 0 2px 4px rgba(36, 31, 26, 0.25);
  transition: transform 0.5s cubic-bezier(0.16, 1, 0.3, 1), opacity 0.5s ease;
}

.tag__hole {
  width: 7px;
  height: 7px;
  fill: none;
  stroke: var(--color-tag-ink);
  stroke-width: 1.2;
}

.tag--knotted {
  transform: rotate(-2deg);
}

/* The tag's exit is the resolution moment: a last cinch tight (scale up,
   straighten) then it snaps off to the side, rather than the pending
   card state simply vanishing. */
.tag-detach-leave-active {
  transition: transform 0.4s cubic-bezier(0.16, 1, 0.3, 1), opacity 0.4s ease-in 0.15s;
}

.tag-detach-leave-to {
  transform: translateX(0.75rem) rotate(18deg) scale(0.85);
  opacity: 0;
}

.tag-detach-enter-active {
  transition: transform 0.3s cubic-bezier(0.16, 1, 0.3, 1), opacity 0.3s ease;
}

.tag-detach-enter-from {
  transform: translateX(-0.5rem) rotate(-12deg) scale(0.9);
  opacity: 0;
}

.torn-corner {
  position: absolute;
  top: 0;
  right: 0;
  width: 2.25rem;
  height: 2.25rem;
  background: var(--color-fail-bg);
  clip-path: polygon(100% 0, 100% 100%, 55% 45%, 78% 18%, 40% 30%, 0 0);
  z-index: 10;
}

@media (prefers-reduced-motion: reduce) {
  .tag,
  .tag-detach-leave-active,
  .tag-detach-enter-active {
    transition: none;
  }
}
</style>
