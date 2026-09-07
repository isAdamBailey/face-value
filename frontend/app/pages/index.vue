<script setup lang="ts">
const searches = useSearchesStore()
const route = useRoute()
const router = useRouter()
const { label: searcherLabel } = useSearcher()

const isDragging = ref(false)
const uploadError = ref('')
const fileInputRef = ref<HTMLInputElement | null>(null)
const sentinelRef = ref<HTMLElement | null>(null)

// "Everyone" plus one option per searcher; the active one is whatever
// ?email= says, so a filtered view is a link someone can share.
const filterOptions = computed(() => [
  { email: null, label: 'Everyone' },
  ...searches.searchers.map(email => ({ email, label: searcherLabel(email) }))
])

onMounted(() => {
  const email = route.query.email
  searches.loadInitial(typeof email === 'string' ? email : null)

  const observer = new IntersectionObserver((entries) => {
    if (entries[0]?.isIntersecting) {
      searches.loadMore()
    }
  })

  if (sentinelRef.value) {
    observer.observe(sentinelRef.value)
  }

  onUnmounted(() => observer.disconnect())
})

async function handleFiles(files: FileList | null) {
  const file = files?.[0]
  if (!file) {
    return
  }

  uploadError.value = ''
  try {
    const resized = await resizeImageFile(file)
    // create() returns to the shared view so the new card is visible.
    await searches.create(resized)
    syncFilterToUrl()
  } catch {
    uploadError.value = 'Could not upload that image. Please try again.'
  }
}

async function selectFilter(email: string | null) {
  await searches.setEmailFilter(email)
  syncFilterToUrl()
}

/** syncFilterToUrl reflects the store's filter into the address bar so the
 * view can be linked and survives a reload. */
function syncFilterToUrl() {
  const email = searches.emailFilter
  router.replace({ query: email ? { email } : {} })
}

function onDrop(event: DragEvent) {
  isDragging.value = false
  handleFiles(event.dataTransfer?.files ?? null)
}

function onFileInputChange(event: Event) {
  const target = event.target as HTMLInputElement
  handleFiles(target.files)
  target.value = ''
}

function openFilePicker() {
  fileInputRef.value?.click()
}

</script>

<template>
  <div class="min-h-screen bg-ground text-ink">
    <div class="specimen-wash">
      <main class="mx-auto max-w-6xl px-4 pt-8 pb-6 sm:px-6">
        <header class="flex items-center justify-between gap-3">
          <div class="flex items-center gap-3">
            <AppLogo :size="36" />
            <div>
              <h1 class="font-display text-3xl font-bold tracking-tight text-ink sm:text-4xl">
                Face Value
              </h1>
              <p class="mt-1 text-sm text-ink-soft">
                Everyone's searches — average asking price across current
                listings, never a sale.
              </p>
            </div>
          </div>
          <p class="hidden shrink-0 text-right text-xs tracking-widest text-ink-soft uppercase sm:block">
            Est. searches<br>No. {{ String(searches.items.length).padStart(3, '0') }}
          </p>
        </header>

        <div
          class="mt-6 flex flex-col items-center justify-center gap-3 rounded-2xl border-2 border-dashed px-6 py-12 text-center transition-colors sm:py-14"
          :class="isDragging ? 'border-terracotta-ink bg-terracotta-bg' : 'border-line-strong bg-ground/70'"
          @dragover.prevent="isDragging = true"
          @dragleave.prevent="isDragging = false"
          @drop.prevent="onDrop"
        >
          <span class="flex h-12 w-12 items-center justify-center rounded-full bg-terracotta-bg text-terracotta-ink">
            <svg viewBox="0 0 24 24" width="22" height="22" fill="none" aria-hidden="true">
              <path d="M4 16.5V18a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-1.5M12 15V4m0 0-4 4m4-4 4 4" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" />
            </svg>
          </span>
          <p class="text-sm text-ink-soft">
            Drop a photo to add a find, or
          </p>
          <button
            type="button"
            class="min-h-12 w-full rounded-full bg-terracotta-ink px-7 py-3.5 font-display text-base font-semibold text-white transition-transform hover:-translate-y-0.5 active:translate-y-0 sm:w-auto"
            @click="openFilePicker"
          >
            Choose a photo
          </button>
          <input
            ref="fileInputRef"
            type="file"
            accept="image/jpeg,image/png,image/webp"
            capture="environment"
            class="sr-only"
            @change="onFileInputChange"
          >
          <p v-if="uploadError" class="text-sm text-fail">
            {{ uploadError }}
          </p>
        </div>
      </main>
    </div>

    <main class="mx-auto max-w-6xl px-4 pb-8 sm:px-6">
      <div
        v-if="searches.searchers.length > 1"
        class="mt-8 flex flex-wrap items-center gap-2"
      >
        <span class="text-xs tracking-widest text-ink-soft uppercase">
          Searched by
        </span>
        <button
          v-for="option in filterOptions"
          :key="option.email ?? 'everyone'"
          type="button"
          class="inline-flex min-h-8 items-center rounded-full px-3 py-1 text-xs transition-colors"
          :class="searches.emailFilter === option.email
            ? 'bg-terracotta-ink text-white'
            : 'bg-ground-deep text-ink-soft ring-1 ring-line hover:text-ink'"
          :title="option.email ?? undefined"
          :aria-pressed="searches.emailFilter === option.email"
          @click="selectFilter(option.email)"
        >
          {{ option.label }}
        </button>
      </div>

      <div v-if="searches.loading && !searches.initialized" class="mt-10 text-center text-sm text-ink-soft">
        Loading searches…
      </div>

      <div
        v-else-if="searches.initialized && searches.items.length === 0"
        class="mt-10 rounded-2xl border border-line bg-ground-deep px-6 py-16 text-center"
      >
        <p class="font-display text-lg font-semibold text-ink">
          {{ searches.emailFilter ? 'No searches from that address yet' : 'No searches yet' }}
        </p>
        <p class="mx-auto mt-2 max-w-md text-sm text-ink-soft">
          Upload a photo of something you own and a vision model will identify it,
          then look up current listings on eBay to estimate an average asking
          price. Not a sale, not an appraisal — just a starting point.
        </p>
        <button
          v-if="searches.emailFilter"
          type="button"
          class="mt-4 rounded-full px-3 py-1.5 text-sm font-medium text-terracotta-ink underline decoration-dotted underline-offset-2"
          @click="selectFilter(null)"
        >
          Show everyone's searches
        </button>
      </div>

      <div
        v-else
        class="mt-8 grid grid-cols-2 gap-4 sm:grid-cols-3 xl:grid-cols-4"
      >
        <SearchCard
          v-for="search in searches.items"
          :key="search.id"
          :search="search"
          @retry="searches.retry"
        />
      </div>

      <div ref="sentinelRef" class="h-1" />
      <div v-if="searches.loadingMore" class="py-6 text-center text-sm text-ink-soft">
        Loading more…
      </div>
    </main>
  </div>
</template>

