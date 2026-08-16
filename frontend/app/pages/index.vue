<script setup lang="ts">
const searches = useSearchesStore()

const isDragging = ref(false)
const uploadError = ref('')
const fileInputRef = ref<HTMLInputElement | null>(null)
const sentinelRef = ref<HTMLElement | null>(null)

onMounted(() => {
  searches.loadInitial()

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
    await searches.create(resized)
  } catch {
    uploadError.value = 'Could not upload that image. Please try again.'
  }
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
  <div class="min-h-screen bg-neutral-950 text-neutral-100">
    <main class="mx-auto max-w-6xl px-4 py-8 sm:px-6">
      <h1 class="text-2xl font-semibold">
        Face Value
      </h1>
      <p class="mt-1 text-sm text-neutral-400">
        Upload a photo. See the average asking price across current listings.
      </p>

      <div
        class="mt-6 flex flex-col items-center justify-center gap-3 rounded-xl border-2 border-dashed px-6 py-12 text-center transition-colors"
        :class="isDragging ? 'border-emerald-400 bg-emerald-400/5' : 'border-neutral-700'"
        @dragover.prevent="isDragging = true"
        @dragleave.prevent="isDragging = false"
        @drop.prevent="onDrop"
      >
        <p class="text-sm text-neutral-300">
          Drag a photo here, or
        </p>
        <button
          type="button"
          class="rounded-md bg-emerald-500 px-4 py-2 text-sm font-medium text-neutral-950 transition-colors hover:bg-emerald-400"
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
        <p v-if="uploadError" class="text-sm text-red-400">
          {{ uploadError }}
        </p>
      </div>

      <div v-if="searches.loading && !searches.initialized" class="mt-10 text-center text-sm text-neutral-500">
        Loading…
      </div>

      <div
        v-else-if="searches.initialized && searches.items.length === 0"
        class="mt-10 rounded-xl border border-neutral-800 px-6 py-16 text-center"
      >
        <p class="text-lg font-medium text-neutral-200">
          Nothing appraised yet
        </p>
        <p class="mx-auto mt-2 max-w-md text-sm text-neutral-400">
          Upload a photo of something you own and a vision model will identify it,
          then look up current listings on eBay to estimate an average asking
          price. Not a sale, not an appraisal — just a starting point.
        </p>
      </div>

      <div
        v-else
        class="mt-10 grid grid-cols-2 gap-4 md:grid-cols-3 xl:grid-cols-4"
      >
        <SearchCard
          v-for="search in searches.items"
          :key="search.id"
          :search="search"
          @retry="searches.retry"
        />
      </div>

      <div ref="sentinelRef" class="h-1" />
      <div v-if="searches.loadingMore" class="py-6 text-center text-sm text-neutral-500">
        Loading more…
      </div>
    </main>
  </div>
</template>
