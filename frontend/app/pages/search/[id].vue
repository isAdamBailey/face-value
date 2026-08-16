<script setup lang="ts">
const route = useRoute()
const searches = useSearchesStore()
const { format: formatMoney } = useMoney()
const { format: formatRelativeTime } = useRelativeTime()

const id = route.params.id as string

const search = computed(() => searches.items.find(i => i.id === id))

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
  <div class="min-h-screen bg-neutral-950 text-neutral-100">
    <main class="mx-auto max-w-4xl px-4 py-8 sm:px-6">
      <NuxtLink to="/" class="text-sm text-neutral-400 hover:text-neutral-200">
        ← Back
      </NuxtLink>

      <div v-if="loadingDetail" class="mt-10 text-center text-sm text-neutral-500">
        Loading…
      </div>

      <div v-else-if="notFound" class="mt-10 rounded-xl border border-neutral-800 px-6 py-16 text-center">
        <p class="text-lg font-medium text-neutral-200">
          Search not found
        </p>
        <p class="mt-2 text-sm text-neutral-400">
          It may have been deleted, or it belongs to someone else.
        </p>
      </div>

      <div v-else-if="search" class="mt-6 space-y-8">
        <div class="grid gap-6 sm:grid-cols-2">
          <img
            :src="search.image_url"
            :alt="search.title || 'Uploaded item'"
            class="aspect-square w-full rounded-lg object-cover ring-1 ring-neutral-800"
          >

          <div class="space-y-3">
            <h1 class="text-xl font-semibold">
              {{ search.title || (isFailed ? 'Unidentified item' : 'Identifying…') }}
            </h1>

            <dl v-if="search.brand || search.model || search.category" class="space-y-1 text-sm">
              <div v-if="search.brand" class="flex gap-2">
                <dt class="text-neutral-500">
                  Brand
                </dt>
                <dd class="text-neutral-200">
                  {{ search.brand }}
                </dd>
              </div>
              <div v-if="search.model" class="flex gap-2">
                <dt class="text-neutral-500">
                  Model
                </dt>
                <dd class="text-neutral-200">
                  {{ search.model }}
                </dd>
              </div>
              <div v-if="search.category" class="flex gap-2">
                <dt class="text-neutral-500">
                  Category
                </dt>
                <dd class="text-neutral-200">
                  {{ search.category }}
                </dd>
              </div>
            </dl>

            <p v-if="search.condition_notes" class="text-sm text-neutral-400">
              {{ search.condition_notes }}
            </p>

            <p v-if="isPending" class="text-sm text-neutral-400">
              {{ search.still_working ? 'Still working — check back soon.' : 'Working…' }}
            </p>
            <p v-else-if="isFailed" class="text-sm text-red-400">
              {{ search.error_message || "Couldn't complete this search." }}
            </p>

            <p class="text-xs text-neutral-500">
              Uploaded {{ formatRelativeTime(search.created_at) }}
            </p>
          </div>
        </div>

        <div
          v-if="search.low_confidence"
          class="rounded-lg border border-amber-700/50 bg-amber-500/10 px-4 py-3 text-sm text-amber-300"
        >
          The vision model wasn't very confident about this identification. If the
          title, brand, or model above looks wrong, edit the search below and
          re-run pricing.
        </div>

        <template v-if="isComplete">
          <section>
            <p class="text-sm text-neutral-400">
              Average asking price, current listings
            </p>
            <p v-if="!hasNoComps" class="text-4xl font-semibold text-emerald-400">
              {{ formatMoney(search.price_trimmed_mean, search.currency) }}
            </p>

            <p v-if="isLowSample" class="mt-2 text-sm text-amber-400">
              Only {{ compCount }} comparable listing{{ compCount === 1 ? '' : 's' }} —
              treat this number as rough.
            </p>
            <p v-else-if="hasNoComps" class="mt-1 text-lg text-neutral-400">
              No comparable listings were found for this search query.
            </p>

            <dl v-if="!hasNoComps" class="mt-4 grid grid-cols-2 gap-3 text-sm sm:grid-cols-4">
              <div>
                <dt class="text-neutral-500">
                  Mean
                </dt>
                <dd class="text-neutral-200">
                  {{ formatMoney(search.price_mean, search.currency) }}
                </dd>
              </div>
              <div>
                <dt class="text-neutral-500">
                  Median
                </dt>
                <dd class="text-neutral-200">
                  {{ formatMoney(search.price_median, search.currency) }}
                </dd>
              </div>
              <div>
                <dt class="text-neutral-500">
                  Min
                </dt>
                <dd class="text-neutral-200">
                  {{ formatMoney(search.price_min, search.currency) }}
                </dd>
              </div>
              <div>
                <dt class="text-neutral-500">
                  Max
                </dt>
                <dd class="text-neutral-200">
                  {{ formatMoney(search.price_max, search.currency) }}
                </dd>
              </div>
            </dl>
          </section>
        </template>

        <section v-if="isComplete || isFailed" class="space-y-2">
          <label for="search_query" class="block text-sm font-medium text-neutral-300">
            Search query
          </label>
          <div class="flex gap-2">
            <input
              id="search_query"
              v-model="editedQuery"
              type="text"
              class="w-full rounded-md bg-neutral-800 px-3 py-2 text-sm text-neutral-100"
              @input="editedQueryTouched = true"
            >
            <button
              type="button"
              :disabled="rerunning || !editedQuery.trim()"
              class="shrink-0 rounded-md bg-emerald-500 px-4 py-2 text-sm font-medium text-neutral-950 transition-colors hover:bg-emerald-400 disabled:opacity-50"
              @click="onRerun"
            >
              {{ rerunning ? 'Re-running…' : 'Re-run pricing' }}
            </button>
          </div>
          <p v-if="rerunError" class="text-sm text-red-400">
            {{ rerunError }}
          </p>
          <p class="text-xs text-neutral-500">
            Vision sometimes gets the model number wrong — correct it here rather
            than re-uploading.
          </p>
        </section>

        <section v-if="(search.comps?.length ?? 0) > 0">
          <div class="flex items-center justify-between">
            <h2 class="text-sm font-medium text-neutral-300">
              Comparable listings
            </h2>
            <label v-if="excludedCount > 0" class="flex items-center gap-2 text-xs text-neutral-400">
              <input v-model="hideExcluded" type="checkbox" class="rounded">
              Hide outliers ({{ excludedCount }})
            </label>
          </div>

          <div class="mt-2 overflow-x-auto rounded-lg ring-1 ring-neutral-800">
            <table class="w-full text-left text-sm">
              <thead class="bg-neutral-900 text-neutral-400">
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
                  class="border-t border-neutral-800"
                  :class="{ 'opacity-50': comp.excluded }"
                >
                  <td class="px-3 py-2">
                    {{ comp.title }}
                    <span
                      v-if="comp.excluded"
                      class="ml-2 rounded bg-neutral-800 px-1.5 py-0.5 text-xs text-neutral-400"
                    >
                      outlier
                    </span>
                  </td>
                  <td class="px-3 py-2 whitespace-nowrap">
                    {{ formatMoney(comp.price, comp.currency) }}
                  </td>
                  <td class="px-3 py-2 text-neutral-400">
                    {{ comp.condition || '—' }}
                  </td>
                  <td class="px-3 py-2 text-neutral-400">
                    {{ comp.buying_option || '—' }}
                  </td>
                  <td class="px-3 py-2">
                    <a
                      v-if="comp.item_url"
                      :href="comp.item_url"
                      target="_blank"
                      rel="noopener noreferrer"
                      class="text-emerald-400 hover:text-emerald-300"
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
