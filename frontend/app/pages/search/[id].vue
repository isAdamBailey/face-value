<script setup lang="ts">
const route = useRoute()
const searches = useSearchesStore()
const { format: formatMoney } = useMoney()
const { format: formatRelativeTime } = useRelativeTime()

const id = route.params.id as string

const search = computed(() => searches.items.find(i => i.id === id))
const palette = useSearchPalette(id)

const loadingDetail = ref(true)
const notFound = ref(false)

const editedQuery = ref('')
const editedQueryTouched = ref(false)
const rerunning = ref(false)
const rerunError = ref('')

// Outliers are shown by default — hiding the trimmed comps is what would
// make the headline number unauditable. This toggle only ever hides them,
// it never removes them from the underlying data.
const hideExcluded = ref(false)

watchEffect(() => {
  if (!editedQueryTouched.value && search.value?.search_query !== undefined) {
    editedQuery.value = search.value.search_query
  }
})

onMounted(async () => {
  try {
    const detail = await searches.fetchDetail(id)
    if (detail.status !== 'complete' && detail.status !== 'failed') {
      searches.poll(id)
    }
  } catch (err: unknown) {
    const statusCode = (err as { statusCode?: number; response?: { status?: number } })?.statusCode
      ?? (err as { response?: { status?: number } })?.response?.status
    if (statusCode === 404) {
      notFound.value = true
    } else {
      // Transient error (network blip, cold start) — fall back to bounded
      // polling rather than showing a dead end.
      searches.poll(id)
    }
  } finally {
    loadingDetail.value = false
  }
})

const isPending = computed(() => {
  const status = search.value?.status
  return status !== undefined && status !== 'complete' && status !== 'failed'
})
const isFailed = computed(() => search.value?.status === 'failed')
const isComplete = computed(() => search.value?.status === 'complete')

const compCount = computed(() => search.value?.comp_count ?? 0)
const isLowSample = computed(() => isComplete.value && compCount.value > 0 && compCount.value < 3)
const hasNoComps = computed(() => isComplete.value && compCount.value === 0)

const visibleComps = computed(() => {
  const comps = search.value?.comps ?? []
  return hideExcluded.value ? comps.filter(c => !c.excluded) : comps
})
const excludedCount = computed(() => (search.value?.comps ?? []).filter(c => c.excluded).length)

async function onRerun() {
  const current = search.value
  const query = editedQuery.value.trim()
  if (!current || !query) {
    return
  }

  rerunning.value = true
  rerunError.value = ''
  try {
    await searches.rerun(current.id, query)
  } catch {
    rerunError.value = 'Could not start the re-run. Please try again.'
  } finally {
    rerunning.value = false
  }
}
</script>

<template>
  <div class="min-h-screen bg-ground text-ink">
    <main class="mx-auto max-w-4xl px-4 py-8 sm:px-6">
      <NuxtLink to="/" class="text-sm text-ink-soft hover:underline">
        ← Back to searches
      </NuxtLink>

      <div v-if="loadingDetail" class="mt-10 text-center text-sm text-ink-soft">
        Loading…
      </div>

      <div
        v-else-if="notFound"
        class="mt-10 rounded-2xl border border-line bg-ground-deep px-6 py-16 text-center"
      >
        <p class="font-display text-lg font-semibold">
          Search not found
        </p>
        <p class="mt-2 text-sm text-ink-soft">
          It may have been deleted, or it belongs to someone else.
        </p>
      </div>

      <div v-else-if="search" class="mt-6 space-y-8">
        <div
          class="detail-card relative overflow-hidden rounded-[18px] ring-1 ring-white/70"
          :class="palette.bg"
        >
          <Transition name="tag-detach">
            <span
              v-if="isPending"
              class="tag"
              aria-hidden="true"
            >
              <svg viewBox="0 0 10 10" class="tag__hole"><circle cx="5" cy="5" r="3" /></svg>
              {{ search.still_working ? 'still working' : 'pending' }}
            </span>
          </Transition>
          <div v-if="isFailed" class="torn-corner" aria-hidden="true" />

          <div class="grid gap-0 sm:grid-cols-2">
            <img
              :src="search.image_url"
              :alt="search.title || 'Uploaded item'"
              class="aspect-square w-full object-cover"
            >

            <div class="space-y-3 p-6">
              <h1 class="font-display text-2xl font-bold text-ink">
                {{ search.title || (isFailed ? 'Unidentified item' : 'Identifying…') }}
              </h1>

              <dl v-if="search.brand || search.model || search.category" class="space-y-1 text-sm">
                <div v-if="search.brand" class="flex gap-2">
                  <dt class="text-ink-soft">
                    Brand
                  </dt>
                  <dd class="text-ink">
                    {{ search.brand }}
                  </dd>
                </div>
                <div v-if="search.model" class="flex gap-2">
                  <dt class="text-ink-soft">
                    Model
                  </dt>
                  <dd class="text-ink">
                    {{ search.model }}
                  </dd>
                </div>
                <div v-if="search.category" class="flex gap-2">
                  <dt class="text-ink-soft">
                    Category
                  </dt>
                  <dd class="text-ink">
                    {{ search.category }}
                  </dd>
                </div>
              </dl>

              <p v-if="search.condition_notes" class="text-sm text-ink-soft italic">
                “{{ search.condition_notes }}”
              </p>

              <p v-if="isPending" class="text-sm text-ink-soft">
                {{ search.still_working ? 'Still working — check back soon.' : 'Working…' }}
              </p>
              <p v-else-if="isFailed" class="text-sm font-medium text-fail">
                {{ search.error_message || "Couldn't complete this search." }}
              </p>

              <p class="text-xs tracking-wide text-ink-soft uppercase">
                found {{ formatRelativeTime(search.created_at) }}
              </p>
            </div>
          </div>
        </div>

        <div
          v-if="search.low_confidence"
          class="rounded-lg border border-ochre-ink bg-ochre-bg px-4 py-3 text-sm text-ochre-ink"
        >
          The vision model wasn't very confident about this identification. If the
          title, brand, or model above looks wrong, edit the search below and
          re-run pricing.
        </div>

        <template v-if="isComplete">
          <section>
            <p class="text-sm text-ink-soft">
              Average asking price, current listings
            </p>
            <p v-if="!hasNoComps" class="readout readout--lg mt-1">
              {{ formatMoney(search.price_trimmed_mean, search.currency) }}
            </p>

            <p v-if="isLowSample" class="mt-2 text-sm text-ochre-ink">
              Only {{ compCount }} comparable listing{{ compCount === 1 ? '' : 's' }} —
              treat this number as rough.
            </p>
            <p v-else-if="hasNoComps" class="mt-1 text-lg text-ink-soft">
              No comparable listings were found for this search query.
            </p>

            <dl v-if="!hasNoComps" class="mt-4 grid grid-cols-2 gap-3 text-sm sm:grid-cols-4">
              <div>
                <dt class="text-ink-soft">
                  Mean
                </dt>
                <dd class="text-ink">
                  {{ formatMoney(search.price_mean, search.currency) }}
                </dd>
              </div>
              <div>
                <dt class="text-ink-soft">
                  Median
                </dt>
                <dd class="text-ink">
                  {{ formatMoney(search.price_median, search.currency) }}
                </dd>
              </div>
              <div>
                <dt class="text-ink-soft">
                  Min
                </dt>
                <dd class="text-ink">
                  {{ formatMoney(search.price_min, search.currency) }}
                </dd>
              </div>
              <div>
                <dt class="text-ink-soft">
                  Max
                </dt>
                <dd class="text-ink">
                  {{ formatMoney(search.price_max, search.currency) }}
                </dd>
              </div>
            </dl>
          </section>
        </template>

        <section v-if="isComplete || isFailed" class="space-y-2">
          <label for="search_query" class="block text-sm font-medium text-ink">
            Search query
          </label>
          <div class="flex gap-2">
            <input
              id="search_query"
              v-model="editedQuery"
              type="text"
              class="w-full rounded-md bg-ground-deep px-3 py-2 text-sm text-ink ring-1 ring-line-strong"
              @input="editedQueryTouched = true"
            >
            <button
              type="button"
              :disabled="rerunning || !editedQuery.trim()"
              class="shrink-0 rounded-full bg-terracotta-ink px-4 py-2 font-display text-sm font-semibold text-white transition-transform hover:-translate-y-0.5 disabled:opacity-50 disabled:hover:translate-y-0"
              @click="onRerun"
            >
              {{ rerunning ? 'Re-running…' : 'Re-run pricing' }}
            </button>
          </div>
          <p v-if="rerunError" class="text-sm text-fail">
            {{ rerunError }}
          </p>
          <p class="text-xs text-ink-soft">
            Vision sometimes gets the model number wrong — correct it here rather
            than re-uploading.
          </p>
        </section>

        <section v-if="(search.comps?.length ?? 0) > 0">
          <div class="flex items-center justify-between">
            <h2 class="text-sm font-semibold tracking-wide text-ink-soft uppercase">
              Comparable listings
            </h2>
            <label v-if="excludedCount > 0" class="flex items-center gap-2 text-xs text-ink-soft">
              <input v-model="hideExcluded" type="checkbox" class="rounded">
              Hide outliers ({{ excludedCount }})
            </label>
          </div>

          <div class="mt-2 overflow-x-auto rounded-lg ring-1 ring-line">
            <table class="w-full text-left text-sm">
              <thead class="bg-ground-deep text-ink-soft">
                <tr>
                  <th class="px-3 py-2 font-medium">
                    Title
                  </th>
                  <th class="px-3 py-2 font-medium">
                    Price
                  </th>
                  <th class="px-3 py-2 font-medium">
                    Condition
                  </th>
                  <th class="px-3 py-2 font-medium">
                    Buying option
                  </th>
                  <th class="px-3 py-2 font-medium" />
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="comp in visibleComps"
                  :key="comp.external_id"
                  class="border-t border-line"
                  :class="{ 'opacity-50': comp.excluded }"
                >
                  <td class="px-3 py-2">
                    {{ comp.title }}
                    <span
                      v-if="comp.excluded"
                      class="ml-2 rounded bg-ground-deep px-1.5 py-0.5 text-xs text-ink-soft"
                    >
                      outlier
                    </span>
                  </td>
                  <td class="px-3 py-2 font-mono whitespace-nowrap">
                    {{ formatMoney(comp.price, comp.currency) }}
                  </td>
                  <td class="px-3 py-2 text-ink-soft">
                    {{ comp.condition || '—' }}
                  </td>
                  <td class="px-3 py-2 text-ink-soft">
                    {{ comp.buying_option || '—' }}
                  </td>
                  <td class="px-3 py-2">
                    <a
                      v-if="comp.item_url"
                      :href="comp.item_url"
                      target="_blank"
                      rel="noopener noreferrer"
                      class="text-terracotta-ink hover:underline"
                    >
                      View →
                    </a>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>
      </div>
    </main>
  </div>
</template>

<style scoped>
.detail-card {
  box-shadow: 0 1px 0 rgba(36, 31, 26, 0.06), 0 10px 28px -16px rgba(36, 31, 26, 0.4);
}

.readout {
  display: inline-block;
  font-family: var(--font-mono);
  font-variant-numeric: tabular-nums;
  font-weight: 700;
  color: var(--color-readout-glow);
  background: var(--color-readout-bg);
  border-radius: 8px;
  text-shadow: 0 0 10px var(--color-readout-glow-dim);
}

.readout--lg {
  font-size: 2.5rem;
  padding: 0.25rem 0.9rem;
}

.tag {
  position: absolute;
  top: 1rem;
  right: -0.35rem;
  z-index: 10;
  display: flex;
  align-items: center;
  gap: 0.3rem;
  padding: 0.25rem 0.6rem 0.25rem 0.4rem;
  background: var(--color-tag);
  color: var(--color-tag-ink);
  font-size: 0.7rem;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  border-radius: 3px 0 0 3px;
  transform: rotate(3deg);
  box-shadow: 0 2px 4px rgba(36, 31, 26, 0.25);
}

.tag__hole {
  width: 8px;
  height: 8px;
  fill: none;
  stroke: var(--color-tag-ink);
  stroke-width: 1.2;
}

.torn-corner {
  position: absolute;
  top: 0;
  right: 0;
  width: 3.5rem;
  height: 3.5rem;
  background: var(--color-fail-bg);
  clip-path: polygon(100% 0, 100% 100%, 55% 45%, 78% 18%, 40% 30%, 0 0);
  z-index: 10;
}

/* The tag's exit is the resolution moment: a last cinch tight then it
   snaps off to the side, rather than the pending state simply vanishing. */
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

@media (prefers-reduced-motion: reduce) {
  .tag-detach-leave-active,
  .tag-detach-enter-active {
    transition: none;
  }
}
</style>
